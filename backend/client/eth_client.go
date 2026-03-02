package client

import (
	"backend/utils"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EthClient struct {
	context    *context.Context
	etchClient *ethclient.Client
}

func NewEthClient(c *context.Context, d *ethclient.Client) *EthClient {
	return &EthClient{context: c, etchClient: d}
}

func (p *EthClient) getBlockByTag() (*types.Header, common.Hash, *utils.AppError) {

	var raw json.RawMessage

	err := p.etchClient.Client().CallContext(*p.context, &raw, "eth_getBlockByTag", "latest", false)
	if err != nil {
		return nil, common.Hash{}, utils.NewAppError(500, "EthClient.getBlockByTag Failed to call eth_getBlockByTag"+err.Error())
	}

	if len(raw) == 0 || string(raw) == "null" {
		return nil, common.Hash{}, nil
	}

	// 解析完整的区块头字段
	var blockData struct {
		Number      string         `json:"number"`
		Hash        common.Hash    `json:"hash"`
		ParentHash  common.Hash    `json:"parentHash"`
		UncleHash   common.Hash    `json:"sha3Uncles"`
		Coinbase    common.Address `json:"miner"`
		Root        common.Hash    `json:"stateRoot"`
		TxHash      common.Hash    `json:"transactionsRoot"`
		ReceiptHash common.Hash    `json:"receiptsRoot"`
		Bloom       hexutil.Bytes  `json:"logsBloom"`
		Difficulty  *hexutil.Big   `json:"difficulty"`
		GasLimit    hexutil.Uint64 `json:"gasLimit"`
		GasUsed     hexutil.Uint64 `json:"gasUsed"`
		Time        hexutil.Uint64 `json:"timestamp"`
		Extra       hexutil.Bytes  `json:"extraData"`
		MixDigest   common.Hash    `json:"mixHash"`
		Nonce       hexutil.Bytes  `json:"nonce"`
		BaseFee     *hexutil.Big   `json:"baseFeePerGas"`
	}

	if err := json.Unmarshal(raw, &blockData); err != nil {
		return nil, common.Hash{}, utils.NewAppError(500, "EthClient.getBlockByTag Failed to unmarshal block data"+err.Error())
	}

	//解析区块号
	num, ok := new(big.Int).SetString(blockData.Number[2:], 16)
	if !ok {
		return nil, common.Hash{}, utils.NewAppError(500, "EthClient.getBlockByTag Failed to parse block number invalid block number format")
	}

	// 构造完整的 Header
	header := &types.Header{
		ParentHash:  blockData.ParentHash,
		UncleHash:   blockData.UncleHash,
		Coinbase:    blockData.Coinbase,
		Root:        blockData.Root,
		TxHash:      blockData.TxHash,
		ReceiptHash: blockData.ReceiptHash,
		Bloom:       types.BytesToBloom(blockData.Bloom),
		Difficulty:  big.NewInt(0),
		Number:      num,
		GasLimit:    uint64(blockData.GasLimit),
		GasUsed:     uint64(blockData.GasUsed),
		Time:        uint64(blockData.Time),
		Extra:       blockData.Extra,
		MixDigest:   blockData.MixDigest,
		BaseFee:     nil,
	}

	if blockData.Difficulty != nil {
		header.Difficulty = blockData.Difficulty.ToInt()
	}

	if blockData.BaseFee != nil {
		header.BaseFee = blockData.BaseFee.ToInt()
	}

	if len(blockData.Nonce) >= 8 {
		var nonceBytes [8]byte
		copy(nonceBytes[:], blockData.Nonce[:8])
		header.Nonce = types.BlockNonce(nonceBytes)
	}

	// 返回 Header 和 RPC 提供的 hash
	// 注意：手动构造的 Header 计算出的 hash 可能不准确，因为：
	// 1. RPC 返回的某些字段可能格式不完全匹配 go-ethereum 的内部格式
	// 2. Header 的内部缓存字段可能未正确初始化
	// 因此，我们应该直接使用 RPC 返回的 hash，它与浏览器显示的 hash 一致
	return header, blockData.Hash, nil
}

func (p *EthClient) fetchBlockWithRetry(blockNumber *big.Int, maxRetries int) (*types.Block, *utils.AppError) {
	var lastErr *utils.AppError
	for attempt := 0; attempt < maxRetries; attempt++ {
		reqCtx, cancel := context.WithTimeout(*p.context, 10*time.Second)
		block, err := p.etchClient.BlockByNumber(reqCtx, blockNumber)
		cancel()

		if err == nil {
			return block, nil
		}
		lastErr = lastErr
		if attempt < maxRetries-1 {
			backoff := time.Duration((attempt+1)*500) * time.Millisecond
			log.Printf("[WARN] failed to fetch block %s, retry %d/%d after %v: %v",
				blockNumber.String(), attempt+1, maxRetries, backoff, err)
			time.Sleep(backoff) // 等待一段时间后重试
		}
		lastErr = utils.NewAppError(500, "EthClient.fetchBlockWithRetry Failed to fetch block"+err.Error())
		time.Sleep(2 * time.Second) // 等待一段时间后重试
	}
	return nil, lastErr
}

func (p *EthClient) fetchBlockRange(start, end uint64, rateLimit time.Duration) ([]types.Block, *utils.AppError) {
	successCount := 0
	skipCount := 0
	ticker := time.NewTicker(rateLimit)
	defer ticker.Stop()
	result := []types.Block{}
	for num := start; num <= end; num++ {
		<-ticker.C
		blockNumber := big.NewInt(0).SetUint64(num)
		block, err := p.fetchBlockWithRetry(blockNumber, 2)
		if err != nil {
			skipCount++
			log.Printf("[ERROR] failed to fetch block %d: %v", num, err)
			continue
		}
		result = append(result, *block)
		successCount++
		p.printBlockInfo(fmt.Sprintf("Block %d", num), block)

		// 检查上下文是否已取消
		select {
		case <-(*p.context).Done():
			log.Printf("[INFO] Context cancelled, stopping at block %d", num)
			return nil, utils.NewAppError(499, "EthClient.fetchBlockRange Context cancelled operation aborted")
		default:
		}
	}
	log.Printf("[INFO] Finished fetching blocks from %d to %d: %d succeeded, %d skipped", start, end, successCount, skipCount)
	return result, nil
}

// printBlockInfo 打印详细的区块信息
func (p *EthClient) printBlockInfo(title string, block *types.Block) {
}

func (p *EthClient) queryTransactionByHash(txHashHex string) (*types.Transaction, bool, *types.Receipt, *utils.AppError) {
	txHash := common.HexToHash(txHashHex)

	tx, isPending, err := p.etchClient.TransactionByHash(*p.context, txHash)
	if err != nil {
		return nil, false, nil, utils.NewAppError(500, "EthClient.queryTransactionByHashFailed to query transaction by hash"+err.Error())
	}

	receipt, err := p.etchClient.TransactionReceipt(*p.context, txHash)
	if err != nil {
		return nil, false, nil, utils.NewAppError(500, "EthClient.queryTransactionByHash Failed to get transaction receipt"+err.Error())
	}
	return tx, isPending, receipt, nil

}

func (p *EthClient) sendTransaction(toAddrHex string, amountEth float64) *utils.AppError {
	//TODO:后续从配置文件里获取
	privKeyHex := os.Getenv("SENDER_PRIVATE_KEY")
	if privKeyHex == "" {
		log.Fatal("SENDER_PRIVATE_KEY is not set (required for send mode)")
	}

	privKey, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		return utils.NewAppError(500, "EthClient.sendTransaction Invalid private key"+err.Error())
	}

	publicKey := privKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return utils.NewAppError(500, "EthClient.sendTransaction Cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := p.etchClient.PendingNonceAt(*p.context, fromAddress)
	if err != nil {
		return utils.NewAppError(500, "EthClient.sendTransaction Failed to get nonce"+err.Error())
	}

	gasLimit := uint64(21000) // 标准交易的 gas limit

	toAddr := common.HexToAddress(toAddrHex)

	chainID, err := p.etchClient.NetworkID(*p.context)
	if err != nil {
		return utils.NewAppError(500, "EthClient.sendTransaction Failed to get network ID"+err.Error())
	}

	gasTipCap, err := p.etchClient.SuggestGasTipCap(*p.context)
	if err != nil {
		return utils.NewAppError(500, "EthClient.sendTransaction Failed to suggest gas tip cap"+err.Error())
	}

	// 获取 base fee，计算 fee cap
	header, err := p.etchClient.HeaderByNumber(*p.context, nil)
	if err != nil {
		log.Fatalf("failed to get header: %v", err)
	}

	baseFee := header.BaseFee
	if baseFee == nil {
		gasPrice, err := p.etchClient.SuggestGasPrice(*p.context)
		if err != nil {
			return utils.NewAppError(500, "EthClient.sendTransaction Failed to suggest gas price"+err.Error())
		}
		baseFee = gasPrice
	}

	// fee cap = base fee * 2 + tip cap（简单策略）
	gasFeeCap := new(big.Int).Add(
		new(big.Int).Mul(baseFee, big.NewInt(2)),
		gasTipCap,
	)

	// 转换 ETH 金额为 Wei
	// amountEth * 1e18
	amountWei := new(big.Float).Mul(
		big.NewFloat(amountEth),
		big.NewFloat(1e18),
	)
	valueWei, _ := amountWei.Int(nil)

	balance, err := p.etchClient.BalanceAt(*p.context, fromAddress, nil)
	if err != nil {
		return utils.NewAppError(500, "EthClient.sendTransaction Failed to get account balance"+err.Error())
	}

	// 计算总费用：value + gasFeeCap * gasLimit
	totalCost := new(big.Int).Add(
		valueWei,
		new(big.Int).Mul(gasFeeCap, big.NewInt(int64(gasLimit))),
	)

	if balance.Cmp(totalCost) < 0 {
		log.Fatalf("insufficient balance: have %s wei, need %s wei", balance.String(), totalCost.String())
	}

	// 构造交易（EIP-1559 动态费用交易）
	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &toAddr,
		Value:     valueWei,
		Data:      nil,
	}
	tx := types.NewTx(txData)

	// 签名交易
	signer := types.NewLondonSigner(chainID)
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		log.Fatalf("failed to sign transaction: %v", err)
	}

	// 发送交易
	if err := p.etchClient.SendTransaction(*p.context, signedTx); err != nil {
		log.Fatalf("failed to send transaction: %v", err)
	}
	return nil
}

// trim0x 移除十六进制字符串前缀 "0x"
func (p *EthClient) trim0x(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}

func (p *EthClient) weiToEth(wei *big.Int) *big.Float {
	weiFloat := new(big.Float).SetInt(wei)
	ethValue := new(big.Float).Quo(weiFloat, big.NewFloat(1e18))
	return ethValue
}
func (p *EthClient) ethToWei(eth float64) *big.Int {
	ethFloat := big.NewFloat(eth)
	weiFloat := new(big.Float).Mul(ethFloat, big.NewFloat(1e18))
	weiInt, _ := weiFloat.Int(nil)
	return weiInt
}
func (p *EthClient) getCurrentBlockNumber() (uint64, *utils.AppError) {
	header, err := p.etchClient.HeaderByNumber(*p.context, nil)
	if err != nil {
		return 0, utils.NewAppError(500, "EthClient.getCurrentBlockNumber Failed to get current block number"+err.Error())
	}
	return header.Number.Uint64(), nil
}
func (p *EthClient) getBalanceAt(addressHex string) (*big.Int, *utils.AppError) {
	address := common.HexToAddress(addressHex)
	balance, err := p.etchClient.BalanceAt(*p.context, address, nil)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getBalanceAt Failed to get balance at address"+err.Error())
	}
	return balance, nil
}
func (p *EthClient) getTransactionCount(addressHex string) (uint64, *utils.AppError) {
	address := common.HexToAddress(addressHex)
	nonce, err := p.etchClient.NonceAt(*p.context, address, nil)
	if err != nil {
		return 0, utils.NewAppError(500, "EthClient.getTransactionCount Failed to get transaction count"+err.Error())
	}
	return nonce, nil
}
func (p *EthClient) getGasPrice() (*big.Int, *utils.AppError) {
	gasPrice, err := p.etchClient.SuggestGasPrice(*p.context)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getGasPrice Failed to get gas price"+err.Error())
	}
	return gasPrice, nil
}
func (p *EthClient) getNetworkID() (*big.Int, *utils.AppError) {
	networkID, err := p.etchClient.NetworkID(*p.context)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getNetworkID Failed to get network ID"+err.Error())
	}
	return networkID, nil
}
func (p *EthClient) getPendingNonceAt(addressHex string) (uint64, *utils.AppError) {
	address := common.HexToAddress(addressHex)
	nonce, err := p.etchClient.PendingNonceAt(*p.context, address)
	if err != nil {
		return 0, utils.NewAppError(500, "EthClient.getPendingNonceAt Failed to get pending nonce"+err.Error())
	}
	return nonce, nil
}
func (p *EthClient) getSuggestedGasTipCap() (*big.Int, *utils.AppError) {
	gasTipCap, err := p.etchClient.SuggestGasTipCap(*p.context)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getSuggestedGasTipCap Failed to suggest gas tip cap"+err.Error())
	}
	return gasTipCap, nil
}
func (p *EthClient) getHeaderByNumber(blockNumber *big.Int) (*types.Header, *utils.AppError) {
	header, err := p.etchClient.HeaderByNumber(*p.context, blockNumber)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getHeaderByNumber Failed to get header by number"+err.Error())
	}
	return header, nil
}
func (p *EthClient) getBlockByNumber(blockNumber *big.Int) (*types.Block, *utils.AppError) {
	block, err := p.etchClient.BlockByNumber(*p.context, blockNumber)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getBlockByNumber Failed to get block by number"+err.Error())
	}
	return block, nil
}
func (p *EthClient) getTransactionReceipt(txHashHex string) (*types.Receipt, *utils.AppError) {
	txHash := common.HexToHash(txHashHex)
	receipt, err := p.etchClient.TransactionReceipt(*p.context, txHash)
	if err != nil {
		return nil, utils.NewAppError(500, "EthClient.getTransactionReceipt Failed to get transaction receipt"+err.Error())
	}
	return receipt, nil
}
func (p *EthClient) getTransactionByHash(txHashHex string) (*types.Transaction, bool, *utils.AppError) {
	txHash := common.HexToHash(txHashHex)
	tx, isPending, err := p.etchClient.TransactionByHash(*p.context, txHash)
	if err != nil {
		return nil, false, utils.NewAppError(500, "EthClient.getTransactionByHash Failed to get transaction by hash"+err.Error())
	}
	return tx, isPending, nil
}
func (p *EthClient) sendRawTransaction(signedTxData []byte) (common.Hash, *utils.AppError) {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(signedTxData); err != nil {
		return common.Hash{}, utils.NewAppError(500, "EthClient.sendRawTransaction Failed to unmarshal signed transaction data"+err.Error())
	}

	err := p.etchClient.SendTransaction(*p.context, tx)
	if err != nil {
		return common.Hash{}, utils.NewAppError(500, "EthClient.sendRawTransaction Failed to send raw transaction"+err.Error())
	}
	return tx.Hash(), nil
}
