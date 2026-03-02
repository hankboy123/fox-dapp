package client

import (
	"backend/utils"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ERC-20 标准 ABI（包含 Transfer 事件定义）

type EthListener struct {
	rpcURL    string
	contract  *common.Address
	context   *context.Context
	rpcClient *ethclient.Client
}

func NewEthListener(r string, a *common.Address, c *context.Context, d *ethclient.Client) *EthListener {
	return &EthListener{rpcURL: r, contract: a, context: c, rpcClient: d}
}

func (e *EthListener) Listen() *utils.AppError {

	// 解析 ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		log.Fatalf("failed to parse ABI: %v", err)
	}

	headerCh := make(chan *types.Header)
	subNewHeader, err := e.rpcClient.SubscribeNewHead(*e.context, headerCh)
	if err != nil {
		println("Failed to subscribe to new headers:", err.Error())
		return utils.NewAppError(500, "Failed to subscribe to new headers"+err.Error())
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	query := ethereum.FilterQuery{
		Addresses: []common.Address{*e.contract},
	}

	logsCh := make(chan types.Log)
	subLogs, err := e.rpcClient.SubscribeFilterLogs(*e.context, query, logsCh)
	if err != nil {
		println("Failed to subscribe to logs:", err.Error())
		return utils.NewAppError(500, "Failed to subscribe to  logs"+err.Error())
	}

	for {
		select {
		case vLog := <-logsCh:
			e.parseLogEvent(&vLog, parsedABI)
		case h := <-headerCh:
			if h == nil {
				continue
			}
			fmt.Printf("[%s] New Block - Number: %d, Hash: %s\n",
				time.Now().Format(time.RFC3339),
				h.Number.Uint64(),
				h.Hash().Hex(),
			)
		case errLog := <-subLogs.Err():
			println("Log subscription error:", errLog.Error())
			return utils.NewAppError(500, "Log subscription error"+errLog.Error())
		case errNewHeader := <-subNewHeader.Err():
			println("Subscription error:", errNewHeader.Error())
			return utils.NewAppError(500, "Subscription error"+errNewHeader.Error())
		case sig := <-sigCh:
			// Handle graceful shutdown
			fmt.Printf("received signal %s, shutting down...\n", sig.String())
			return nil
		case <-(*e.context).Done():
			fmt.Println("context cancelled, shutting down...")
			return nil

		}
	}
}

func (e *EthListener) parseLogEvent(vLog *types.Log, parseABI abi.ABI) (map[string]interface{}, *utils.AppError) {
	event := make(map[string]interface{})

	eventTopic := vLog.Topics[0]
	var eventName string
	var eventSig abi.Event

	for name, event := range parseABI.Events {
		if event.ID == eventTopic {
			eventName = name
			eventSig = event
			break
		}
	}
	if eventName == "" {
		// 如果无法识别事件类型，打印原始信息
		fmt.Printf("[%s] Unknown Event - Block: %d, Tx: %s, Topic[0]: %s\n",
			time.Now().Format(time.RFC3339),
			vLog.BlockNumber,
			vLog.TxHash.Hex(),
			eventTopic.Hex(),
		)
		return nil, utils.NewAppError(500, "Unknown Event")
	}

	event["event_name"] = eventName
	event["block_number"] = vLog.BlockNumber
	event["tx_hash"] = vLog.TxHash.Hex()
	event["contract_address"] = vLog.Address.Hex()
	event["event_signature"] = eventSig.String()
	event["indexed_count"] = len(eventSig.Inputs)
	event["data_count"] = len(eventSig.Inputs.NonIndexed())
	event["total_arg_count"] = len(eventSig.Inputs)
	if len(vLog.Topics)-1 != len(eventSig.Inputs) {
		return nil, utils.NewAppError(500, "Mismatched indexed argument count")
	}
	if len(vLog.Data) == 0 && len(eventSig.Inputs.NonIndexed()) > 0 {
		return nil, utils.NewAppError(500, "No data for non-indexed arguments")
	}

	// 解析 indexed 参数
	indexedParamIndex := 0
	for i, input := range eventSig.Inputs {
		if !input.Indexed {
			continue
		}

		topicIndex := indexedParamIndex + 1
		indexedParamIndex++

		if topicIndex >= len(vLog.Topics) {
			continue
		}

		topic := vLog.Topics[topicIndex]
		fmt.Printf("    [%d] %s (%s): ", i+1, input.Name, input.Type)

		switch input.Type.T {
		case abi.AddressTy:
			address := common.BytesToAddress(topic.Bytes())
			fmt.Printf("%s\n", address.Hex())
			event[input.Name] = address.Hex()
		case abi.UintTy, abi.IntTy:
			bigIntValue := new(big.Int).SetBytes(topic.Bytes())
			fmt.Printf("%s\n", bigIntValue.String())
			event[input.Name] = bigIntValue.String()
		case abi.BoolTy:
			boolValue := topic.Bytes()[len(topic.Bytes())-1] == 1
			fmt.Printf("%t\n", boolValue)
			event[input.Name] = boolValue
		case abi.BytesTy, abi.FixedBytesTy:
			fmt.Printf("0x%s\n", hex.EncodeToString(topic.Bytes()))
			event[input.Name] = "0x" + hex.EncodeToString(topic.Bytes())
		default:
			fmt.Printf("Unsupported indexed type: %s\n", input.Type.String())
			return nil, utils.NewAppError(500, "Unsupported indexed type")
		}
	}

	if len(vLog.Data) > 0 {
		fmt.Printf("    Data: 0x%s\n", hex.EncodeToString(vLog.Data))
		nonIndexedInputs := make([]abi.Argument, 0)
		for _, input := range eventSig.Inputs {
			if !input.Indexed {
				nonIndexedInputs = append(nonIndexedInputs, input)
			}
		}
		fmt.Printf("    Non-Indexed Inputs: %+v\n", nonIndexedInputs)
		if len(nonIndexedInputs) > 0 {
			fmt.Printf("    Data Length: %d bytes\n", len(vLog.Data))
			values, err := parseABI.Unpack(eventName, vLog.Data)
			if err != nil {
				fmt.Printf("    Failed to unpack non-indexed data: %v\n", err)
			} else {
				nonIndexedIdx := 0
				for i, input := range eventSig.Inputs {
					value := values[nonIndexedIdx]
					fmt.Printf("    [%d] %s (%s): ", i+1, input.Name, input.Type)

					switch input.Type.T {
					case abi.AddressTy:
						address := value.(common.Address)
						fmt.Printf("%s\n", address.Hex())
						event[input.Name] = address.Hex()
					case abi.UintTy, abi.IntTy:
						bigIntValue := value.(*big.Int)
						fmt.Printf("%s\n", bigIntValue.String())
						event[input.Name] = bigIntValue.String()
					case abi.BoolTy:
						boolValue := value.(bool)
						fmt.Printf("%t\n", boolValue)
						event[input.Name] = boolValue
					case abi.BytesTy, abi.FixedBytesTy:
						byteSlice := value.([]byte)
						fmt.Printf("0x%s\n", hex.EncodeToString(byteSlice))
						event[input.Name] = "0x" + hex.EncodeToString(byteSlice)
					default:
						fmt.Printf("Unsupported non-indexed type: %s\n", input.Type.String())
					}
					nonIndexedIdx++

				}
			}
		} else {
			fmt.Printf("    No Non-Indexed Inputs\n")
		}
	}

	return event, nil
}

func (e *EthListener) ListenWithReconnect() {
	var attempt int

	for {
		select {
		case <-(*e.context).Done():
			fmt.Println("Context cancelled, stopping reconnection loop")
			return
		default:
		}
		attempt++
		log.Printf("connect attempt #%d to %s", attempt, e.rpcURL)

		client, err := ethclient.DialContext(*e.context, e.rpcURL)
		if err != nil {
			log.Printf("failed to connect to Ethereum node: %v", err)
			e.sleepWithBackoff(attempt)
			continue
		}
		e.rpcClient = client
		log.Println("connected to Ethereum node")

		errForListen := e.Listen()
		if errForListen != nil {
			e.rpcClient.Close()
			e.sleepWithBackoff(attempt)
			goto RECONNECT
		}
		return

	RECONNECT:
	}

}

func (e *EthListener) sleepWithBackoff(attempt int) {
	sec := int(math.Min(60, math.Pow(2, float64(attempt))))
	d := time.Duration(sec) * time.Second
	fmt.Printf("Sleeping for %s before retrying...\n", d)
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return
	case <-(*e.context).Done():
		fmt.Println("context cancelled during backoff sleep")
		return
	}
}
