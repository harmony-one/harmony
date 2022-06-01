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

	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	jsoniter "github.com/json-iterator/go"
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

// ToMessage converts CallArgs to the Message type used by the core evm
// Adapted from go-ethereum/internal/ethapi/api.go
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

func (d Delegation) IntoStructuredResponse() StructuredResponse {
	return StructuredResponse{
		"validator_address": d.ValidatorAddress,
		"delegator_address": d.DelegatorAddress,
		"amount":            d.Amount,
		"reward":            d.Reward,
		"Undelegations":     d.Undelegations,
	}
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
	var jsonIter = jsoniter.ConfigCompatibleWithStandardLibrary
	dat, err := jsonIter.Marshal(input)
	if err != nil {
		return nil, err
	}
	d := jsonIter.NewDecoder(bytes.NewReader(dat))
	d.UseNumber()
	err = d.Decode(&objMap)
	if err != nil {
		return nil, err
	}
	return objMap, nil
}

// BlockNumber ..
type BlockNumber rpc.BlockNumber

const (
	// LatestBlockNumber is the alias to rpc latest block number
	LatestBlockNumber = BlockNumber(rpc.LatestBlockNumber)

	// PendingBlockNumber is the alias to rpc pending block number
	PendingBlockNumber = BlockNumber(rpc.PendingBlockNumber)
)

// UnmarshalJSON converts a hex string or integer to a block number
func (bn *BlockNumber) UnmarshalJSON(data []byte) error {
	baseBn := rpc.BlockNumber(0)
	baseErr := baseBn.UnmarshalJSON(data)
	if baseErr != nil {
		input := strings.TrimSpace(string(data))
		if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
			input = input[1 : len(input)-1]
		}
		input = strings.TrimPrefix(strings.ToLower(input), "0x")
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
	VRF              string            `json:"vrf"`
	VRFProof         string            `json:"vrfProof"`
	CrossLinks       *types.CrossLinks `json:"crossLinks,omitempty"`
}

// NewHeaderInformation returns the header information that will serialize to the RPC representation.
func NewHeaderInformation(header *block.Header, leader string) *HeaderInformation {
	if header == nil {
		return nil
	}

	vrfAndProof := header.Vrf()
	vrf := common.Hash{}
	vrfProof := []byte{}
	if len(vrfAndProof) == 32+96 {
		copy(vrf[:], vrfAndProof[:32])
		vrfProof = vrfAndProof[32:]
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
		VRF:              hex.EncodeToString(vrf[:]),
		VRFProof:         hex.EncodeToString(vrfProof),
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

// AddressOrList represents an address or a list of addresses
type AddressOrList struct {
	Address     *common.Address
	AddressList []common.Address
}

// UnmarshalJSON defines the input parsing of AddressOrList
func (aol *AddressOrList) UnmarshalJSON(data []byte) (err error) {
	var itf interface{}
	if err := json.Unmarshal(data, &itf); err != nil {
		return err
	}
	switch d := itf.(type) {
	case string: // Single address
		addr, err := internal_common.ParseAddr(d)
		if err != nil {
			return err
		}
		aol.Address = &addr
		return nil

	case []interface{}: // Address array
		var addrs []common.Address
		for _, addrItf := range d {
			addrStr, ok := addrItf.(string)
			if !ok {
				return errors.New("not invalid address array")
			}
			addr, err := internal_common.ParseAddr(addrStr)
			if err != nil {
				return fmt.Errorf("invalid address: %v", addrStr)
			}
			addrs = append(addrs, addr)
		}
		aol.AddressList = addrs
		return nil

	default:
		return errors.New("must provide one address or address list")
	}
}
