package client

import (
	"backend/utils"
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const erc20ABIJSON = `[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "from", "type": "address"},
      {"indexed": true, "name": "to", "type": "address"},
      {"indexed": false, "name": "value", "type": "uint256"}
    ],
    "name": "Transfer",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "owner", "type": "address"},
      {"indexed": true, "name": "spender", "type": "address"},
      {"indexed": false, "name": "value", "type": "uint256"}
    ],
    "name": "Approval",
    "type": "event"
  }
]`

type ErcClient struct {
	context    *context.Context
	etchClient *ethclient.Client
	parsedABI  *abi.ABI
}

func NewErcClient(p *abi.ABI, c *context.Context, d *ethclient.Client) *ErcClient {
	return &ErcClient{context: c, etchClient: d, parsedABI: p}
}

func (e *ErcClient) handleBalanceOf(contractHex, addrHex string) (*big.Int, *utils.AppError) {
	if contractHex == "" || addrHex == "" {
		return nil, utils.NewAppError(400, "contract or address is empty")
	}
	contractAddress := common.HexToAddress(contractHex)
	accountAddress := common.HexToAddress(addrHex)

	// 准备调用数据
	data, err := e.parsedABI.Pack("balanceOf", accountAddress)
	if err != nil {
		return nil, utils.NewAppError(500, "failed to pack balanceOf data: "+err.Error())
	}

	// 创建调用消息
	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	// 执行调用
	result, err := e.etchClient.CallContract(*e.context, msg, nil)
	if err != nil {
		return nil, utils.NewAppError(500, "failed to call contract: "+err.Error())
	}

	// 解析返回值
	balance := new(big.Int)
	if err := e.parsedABI.UnpackIntoInterface(balance, "balanceOf", result); err != nil {
		return nil, utils.NewAppError(500, "failed to unpack balanceOf result: "+err.Error())
	}

	return balance, nil
}

// handleTransfer 发送 ERC-20 transfer 交易
func (e *ErcClient) handleTransfer(contractHex, toHex, amountStr string) *utils.AppError {
	if contractHex == "" || toHex == "" || amountStr == "" {
		log.Fatal("missing --contract, --to, or --amount flag for transfer mode")
		return utils.NewAppError(400, "missing contract, to, or amount for transfer mode")
	}

	// 检查私钥环境变量(后续从配置文件里获取)
	privKeyHex := os.Getenv("SENDER_PRIVATE_KEY")
	if privKeyHex == "" {

		log.Fatal("SENDER_PRIVATE_KEY is not set (required for transfer mode)")
		return utils.NewAppError(400, "SENDER_PRIVATE_KEY is not set (required for transfer mode)")
	}

	privKey, err := crypto.HexToECDSA(e.trim0x(privKeyHex))
	if err != nil {
		return utils.NewAppError(500, "invalid private key: "+err.Error())
	}

	publicKey := privKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return utils.NewAppError(500, "error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	contractAddress := common.HexToAddress(contractHex)
	toAddress := common.HexToAddress(toHex)

	// 准备交易数据
	decimals, appErr := e.getTokenDecimals(contractAddress)
	if appErr != nil {
		return appErr
	}

	amount, err1 := e.parseTokenAmount(amountStr, decimals)
	if err1 != nil {
		return err1
	}

	chainID, err2 := e.etchClient.ChainID(*e.context)
	if err2 != nil {
		return utils.NewAppError(500, err2.Error())
	}

	// 获取 nonce
	nonce, err3 := e.etchClient.PendingNonceAt(*e.context, fromAddress)
	if err3 != nil {
		return utils.NewAppError(500, err3.Error())
	}

	callData, err := e.parsedABI.Pack("transfer", toAddress, amount)
	if err != nil {
		return utils.NewAppError(500, err.Error())
	}

	gasLimit, err := e.etchClient.EstimateGas(*e.context, ethereum.CallMsg{
		From: fromAddress,
		To:   &contractAddress,
		Data: callData,
	})
	if err != nil {
		return utils.NewAppError(500, err.Error())
	}

	gasLimit = gasLimit * 120 / 100

	gasTipCap, err := e.etchClient.SuggestGasTipCap(*e.context)
	if err != nil {
		return utils.NewAppError(500, err.Error())
	}

	// 获取 base fee，计算 fee cap
	header, err := e.etchClient.HeaderByNumber(*e.context, nil)
	if err != nil {
		return utils.NewAppError(500, err.Error())
	}

	baseFee := header.BaseFee
	if baseFee == nil {
		// 如果不支持 EIP-1559，使用传统 gas price
		gasPrice, err := e.etchClient.SuggestGasPrice(*e.context)
		if err != nil {
			return utils.NewAppError(500, err.Error())
		}
		baseFee = gasPrice
	}

	// fee cap = base fee * 2 + tip cap（简单策略）
	gasFeeCap := new(big.Int).Add(
		new(big.Int).Mul(baseFee, big.NewInt(2)),
		gasTipCap,
	)

	// 检查 ETH 余额是否足够支付 Gas 费用
	balance, err := e.etchClient.BalanceAt(*e.context, fromAddress, nil)
	if err != nil {
		return utils.NewAppError(500, err.Error())
	}

	// 计算总费用：gasFeeCap * gasLimit（ERC-20 转账不需要发送 ETH，只需要支付 Gas）
	totalGasCost := new(big.Int).Mul(gasFeeCap, big.NewInt(int64(gasLimit)))

	if balance.Cmp(totalGasCost) < 0 {
		return utils.NewAppError(500, err.Error())
	}

	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &contractAddress,
		Value:     big.NewInt(0),
		Data:      callData,
	}
	tx := types.NewTx(txData)

	signer := types.NewLondonSigner(chainID)

	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return utils.NewAppError(500, err.Error())
	}
	if err := e.etchClient.SendTransaction(*e.context, signedTx); err != nil {
		return utils.NewAppError(500, err.Error())
	}
	tokenAmount := e.formatTokenAmount(amount, decimals)
	fmt.Printf("Amount        : %s tokens (%s raw units)\n", tokenAmount, amount.String())
	e.waitForTransaction(signedTx.Hash())
	return nil
}

func (e *ErcClient) waitForTransaction(txHash common.Hash) {
	waitCtx, cancel := context.WithTimeout(*e.context, 2*time.Minute)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-waitCtx.Done():
			return
		case <-ticker.C:
			receipt, err := e.etchClient.TransactionReceipt(waitCtx, txHash)
			if err != nil {
				continue
			}

			if receipt.Status == 0 {

			} else {
				if len(receipt.Logs) > 0 {

				}
			}
			return
		}
	}

}

func (e *ErcClient) getTokenDecimals(contractAddress common.Address) (uint8, *utils.AppError) {
	// 准备调用数据
	data, err := e.parsedABI.Pack("decimals")
	if err != nil {
		return 0, utils.NewAppError(500, "failed to pack decimals data: "+err.Error())
	}

	// 创建调用消息
	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	// 执行调用
	result, err := e.etchClient.CallContract(*e.context, msg, nil)
	if err != nil {
		return 0, utils.NewAppError(500, "failed to call contract: "+err.Error())
	}

	// 解析返回值
	var decimals uint8
	if err := e.parsedABI.UnpackIntoInterface(&decimals, "decimals", result); err != nil {
		return 0, utils.NewAppError(500, "failed to unpack decimals result: "+err.Error())
	}

	return decimals, nil
}
func (e *ErcClient) parseTokenAmount(amountStr string, decimals uint8) (*big.Int, *utils.AppError) {
	if strings.Contains(amountStr, ".") {
		amountFloat, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return nil, utils.NewAppError(500, "invalid amount format: "+err.Error())
		}

		bigFloat := big.NewFloat(amountFloat)

		multiplier := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
		bigFloat.Mul(bigFloat, multiplier)

		amount, _ := bigFloat.Int(nil)
		return amount, nil
	} else {
		amountBigInt, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			return nil, utils.NewAppError(500, "invalid amount: "+amountStr)
		}
		return amountBigInt, nil
	}
}

// trim0x 移除十六进制字符串前缀 "0x"
func (e *ErcClient) trim0x(s string) string {
	if len(s) >= 2 && s[0:2] == "0x" {
		return s[2:]
	}
	return s
}

func (e *ErcClient) formatTokenAmount(amount *big.Int, decimals uint8) string {
	amountFloat := new(big.Float).SetInt(amount)
	divisor := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
	amountFloat.Quo(amountFloat, divisor)
	return amountFloat.Text('f', int(decimals))
}

func (e *ErcClient) handleParseEvent(txHashHex string) {
	if txHashHex == "" {
		log.Fatal("missing --txhash flag for parse-event mode")
	}

	txHash := common.HexToHash(txHashHex)

	receipt, err := e.etchClient.TransactionReceipt(*e.context, txHash)
	if err != nil {
		log.Fatalf("failed to get transaction receipt: %v", err)
	}

	transferEvent := e.parsedABI.Events["Transfer"]
	transferEventSigHash := crypto.Keccak256Hash([]byte(transferEvent.Sig))
	foundTransfer := false

	for _, vLog := range receipt.Logs {

		if len(vLog.Topics) == 0 || vLog.Topics[0] != transferEventSigHash {
			continue
		}
		foundTransfer = true

		if len(vLog.Topics) >= 2 {
			fromAddr := common.BytesToAddress(vLog.Topics[1].Bytes())
			fmt.Printf("  Parsed Address: %s\n", fromAddr.Hex())
			fmt.Printf("\n")
		}

		if len(vLog.Topics) >= 3 {
			toAddr := common.BytesToAddress(vLog.Topics[2].Bytes())
			fmt.Printf("  Parsed Address: %s\n", toAddr.Hex())
			fmt.Printf("\n")
		}

		if len(vLog.Data) > 0 {
			values, err := e.parsedABI.Unpack("Transfer", vLog.Data)
			if err != nil {
				log.Fatalf("failed to unpack Transfer event data: %v", err)
			}

			if len(values) > 0 {
				value, ok := values[0].(*big.Int)
				if ok {
					fmt.Printf("  Parsed Value: %s\n", value.String())
					fmt.Printf("\n")
				}

			}
		}

		if len(vLog.Topics) >= 3 {
			fromAddr := common.BytesToAddress(vLog.Topics[1].Bytes())
			toAddr := common.BytesToAddress(vLog.Topics[2].Bytes())

			var value *big.Int
			if len(vLog.Data) > 0 {
				values, err := e.parsedABI.Unpack("Transfer", vLog.Data)
				if err == nil && len(values) > 0 {
					if v, ok := values[0].(*big.Int); ok {
						value = v
					}
				}
			}

			if value != nil {
				fmt.Printf("  from  : %s (from Topics[1])\n", fromAddr.Hex())
				fmt.Printf("  to    : %s (from Topics[2])\n", toAddr.Hex())
				fmt.Printf("  value : %s (from Data)\n", value.String())
			}
		}

	}
	if !foundTransfer {
		fmt.Printf("No Transfer event found in this transaction.\n")
		fmt.Printf("Total logs: %d\n", len(receipt.Logs))
	}
}
