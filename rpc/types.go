package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
}

// Adapted from go-ethereum/internal/ethapi/api.go
// ToMessage converts CallArgs to the Message type used by the core evm
func (args *CallArgs) ToMessage(globalGasCap *big.Int) types.Message {
	// Set sender address or use zero address if none specified.
	var addr common.Address
	if args.From != nil {
		addr = *args.From
	}

	// Set default gas & gas price if none were set
	gas := uint64(math.MaxUint64 / 2)
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != nil && globalGasCap.Uint64() < gas {
		utils.Logger().Warn().
			Uint64("requested", gas).
			Uint64("cap", globalGasCap.Uint64()).
			Msg("Caller gas above allowance, capping")
		gas = globalGasCap.Uint64()
	}
	gasPrice := new(big.Int)
	if args.GasPrice != nil {
		gasPrice = args.GasPrice.ToInt()
	}

	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}

	var data []byte
	if args.Data != nil {
		data = []byte(*args.Data)
	}

	msg := types.NewMessage(addr, args.To, 0, value, gas, gasPrice, data, false)
	return msg
}

// StakingNetworkInfo returns global staking info.
type StakingNetworkInfo struct {
	TotalSupply       numeric.Dec `json:"total-supply"`
	CirculatingSupply numeric.Dec `json:"circulating-supply"`
	EpochLastBlock    uint64      `json:"epoch-last-block"`
	TotalStaking      *big.Int    `json:"total-staking"`
	MedianRawStake    numeric.Dec `json:"median-raw-stake"`
}

// Delegation represents a particular delegation to a validator
type Delegation struct {
	ValidatorAddress string         `json:"validator_address"`
	DelegatorAddress string         `json:"delegator_address"`
	Amount           *big.Int       `json:"amount"`
	Reward           *big.Int       `json:"reward"`
	Undelegations    []Undelegation `json:"Undelegations"`
}

// Undelegation represents one undelegation entry
type Undelegation struct {
	Amount *big.Int
	Epoch  *big.Int
}

// StructuredResponse type of RPCs
type StructuredResponse = map[string]interface{}

// NewStructuredResponse creates a structured response from the given input
func NewStructuredResponse(input interface{}) (StructuredResponse, error) {
	var objMap StructuredResponse
	dat, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(dat))
	d.UseNumber()
	err = d.Decode(&objMap)
	if err != nil {
		return nil, err
	}
	return objMap, nil
}

// BlockNumber ..
type BlockNumber rpc.BlockNumber

// UnmarshalJSON converts a hex string or integer to a block number
func (bn *BlockNumber) UnmarshalJSON(data []byte) error {
	baseBn := rpc.BlockNumber(0)
	baseErr := baseBn.UnmarshalJSON(data)
	if baseErr != nil {
		input := strings.TrimSpace(string(data))
		if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
			input = input[1 : len(input)-1]
		}
		input = strings.TrimPrefix(input, "0x")
		num, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return err
		}
		*bn = BlockNumber(num)
		return nil
	}
	*bn = BlockNumber(baseBn)
	return nil
}

// Int64 ..
func (bn BlockNumber) Int64() int64 {
	return (int64)(bn)
}

// EthBlockNumber ..
func (bn BlockNumber) EthBlockNumber() rpc.BlockNumber {
	return (rpc.BlockNumber)(bn)
}

// TransactionIndex ..
type TransactionIndex uint64

// UnmarshalJSON converts a hex string or integer to a Transaction index
func (i *TransactionIndex) UnmarshalJSON(data []byte) (err error) {
	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}

	var num int64
	if strings.HasPrefix(input, "0x") {
		num, err = strconv.ParseInt(strings.TrimPrefix(input, "0x"), 16, 64)
	} else {
		num, err = strconv.ParseInt(input, 10, 64)
	}
	if err != nil {
		return err
	}

	*i = TransactionIndex(num)
	return nil
}

// TxHistoryArgs is struct to include optional transaction formatting params.
type TxHistoryArgs struct {
	Address   string `json:"address"`
	PageIndex uint32 `json:"pageIndex"`
	PageSize  uint32 `json:"pageSize"`
	FullTx    bool   `json:"fullTx"`
	TxType    string `json:"txType"`
	Order     string `json:"order"`
}

// UnmarshalFromInterface ..
func (ta *TxHistoryArgs) UnmarshalFromInterface(blockArgs interface{}) error {
	var args TxHistoryArgs
	dat, err := json.Marshal(blockArgs)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &args); err != nil {
		return err
	}
	*ta = args
	return nil
}

// HeaderInformation represents the latest consensus information
type HeaderInformation struct {
	BlockHash        common.Hash       `json:"blockHash"`
	BlockNumber      uint64            `json:"blockNumber"`
	ShardID          uint32            `json:"shardID"`
	Leader           string            `json:"leader"`
	ViewID           uint64            `json:"viewID"`
	Epoch            uint64            `json:"epoch"`
	Timestamp        string            `json:"timestamp"`
	UnixTime         uint64            `json:"unixtime"`
	LastCommitSig    string            `json:"lastCommitSig"`
	LastCommitBitmap string            `json:"lastCommitBitmap"`
	CrossLinks       *types.CrossLinks `json:"crossLinks,omitempty"`
}

// NewHeaderInformation returns the header information that will serialize to the RPC representation.
func NewHeaderInformation(header *block.Header, leader string) *HeaderInformation {
	if header == nil {
		return nil
	}

	result := &HeaderInformation{
		BlockHash:        header.Hash(),
		BlockNumber:      header.Number().Uint64(),
		ShardID:          header.ShardID(),
		Leader:           leader,
		ViewID:           header.ViewID().Uint64(),
		Epoch:            header.Epoch().Uint64(),
		UnixTime:         header.Time().Uint64(),
		Timestamp:        time.Unix(header.Time().Int64(), 0).UTC().String(),
		LastCommitBitmap: hex.EncodeToString(header.LastCommitBitmap()),
	}

	sig := header.LastCommitSignature()
	result.LastCommitSig = hex.EncodeToString(sig[:])

	if header.ShardID() == shard.BeaconChainShardID {
		decodedCrossLinks := &types.CrossLinks{}
		err := rlp.DecodeBytes(header.CrossLinks(), decodedCrossLinks)
		if err != nil {
			result.CrossLinks = &types.CrossLinks{}
		} else {
			result.CrossLinks = decodedCrossLinks
		}
	}

	return result
}

// Taken from go-ethereum/internal/ethapi/api.go
// ExecutionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as transaction
// execution status, the amount of gas used and the return value
type ExecutionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}

// StructLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   error              `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `jsogun:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

// FormatLogs formats EVM returned structured logs for json output
func FormatLogs(logs []vm.StructLog) []StructLogRes {
	formatted := make([]StructLogRes, len(logs))
	for index, trace := range logs {
		formatted[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.Err,
		}
		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = fmt.Sprintf("%x", math.PaddedBigBytes(stackValue, 32))
			}
			formatted[index].Stack = &stack
		}
		if trace.Memory != nil {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
			}
			formatted[index].Memory = &memory
		}
		if trace.Storage != nil {
			storage := make(map[string]string)
			for i, storageValue := range trace.Storage {
				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
			}
			formatted[index].Storage = &storage
		}
	}
	return formatted
}