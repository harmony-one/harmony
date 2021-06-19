package explorer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	core2 "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

type oneAddress string

type (
	// TxRecord is the data structure stored in explorer db for the a Transaction record
	TxRecord struct {
		Hash      common.Hash
		Timestamp time.Time
	}

	// TxType is the transaction type. Currently on txSent and txReceived.
	// TODO: add staking to this type logic handle.
	TxType byte
)

const (
	txUnknown TxType = iota
	txSent
	txReceived

	txSentStr     = "SENT"
	txReceivedStr = "RECEIVED"
)

func (t TxType) String() string {
	switch t {
	case txSent:
		return txSentStr
	case txReceived:
		return txReceivedStr
	}
	return "UNKNOWN"
}

func legTxTypeToTxType(legType string) (TxType, error) {
	switch legType {
	case LegReceived:
		return txReceived, nil
	case LegSent:
		return txSent, nil
	}
	return txUnknown, fmt.Errorf("unknown transaction type: %v", legType)
}

func legTxRecordToTxRecord(leg *LegTxRecord) (*TxRecord, TxType, error) {
	txHash := common.HexToHash(leg.Hash)
	t, err := legTxTypeToTxType(leg.Type)
	if err != nil {
		return nil, 0, err
	}
	i, err := strconv.ParseInt(leg.Timestamp, 10, 64)
	if err != nil {
		return nil, 0, err
	}
	tm := time.Unix(i, 0)
	return &TxRecord{
		Hash:      txHash,
		Timestamp: tm,
	}, t, nil
}

type storedTxRecord struct {
	Hash common.Hash
	Type byte
	Time uint64
}

func (tx *TxRecord) EncodeRLP(w io.Writer) error {
	storedTx := storedTxRecord{
		Hash: tx.Hash,
		Time: uint64(tx.Timestamp.Unix()),
	}
	return rlp.Encode(w, storedTx)
}

func (tx *TxRecord) DecodeRLP(st *rlp.Stream) error {
	var storedTx storedTxRecord
	if err := st.Decode(&storedTx); err != nil {
		return err
	}
	tx.Hash = storedTx.Hash
	tx.Timestamp = time.Unix(int64(storedTx.Time), 0)
	return nil
}

// Tx types ...
const (
	LegReceived = "RECEIVED"
	LegSent     = "SENT"
)

// LegTxRecord ...
type LegTxRecord struct {
	Hash      string
	Type      string
	Timestamp string
}

// LegTxRecords ...
type LegTxRecords []*LegTxRecord

// Data ...
type Data struct {
	Addresses []string `json:"Addresses"`
}

// Address ...
type Address struct {
	ID         string       `json:"id"`
	Balance    *big.Int     `json:"balance"` // Deprecated
	TXs        LegTxRecords `json:"txs"`
	StakingTXs LegTxRecords `json:"staking_txs"`
}

// Transaction ...
type Transaction struct {
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Value     *big.Int `json:"value"`
	Bytes     string   `json:"bytes"`
	Data      string   `json:"data"`
	GasFee    *big.Int `json:"gasFee"`
	FromShard uint32   `json:"fromShard"`
	ToShard   uint32   `json:"toShard"`
	Type      string   `json:"type"`
}

// GetTransaction ...
func GetTransaction(tx *types.Transaction, addressBlock *types.Block) (*Transaction, error) {
	msg, err := tx.AsMessage(types.NewEIP155Signer(tx.ChainID()))
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
	}
	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.GasLimit()))
	to := ""
	if msg.To() != nil {
		if to, err = common2.AddressToBech32(*msg.To()); err != nil {
			return nil, err
		}
	}
	from := ""
	if from, err = common2.AddressToBech32(msg.From()); err != nil {
		return nil, err
	}

	return &Transaction{
		ID:        tx.HashByType().Hex(),
		Timestamp: strconv.Itoa(int(addressBlock.Time().Int64() * 1000)),
		From:      from,
		To:        to,
		Value:     msg.Value(),
		Bytes:     strconv.Itoa(int(tx.Size())),
		Data:      hex.EncodeToString(tx.Data()),
		GasFee:    gasFee,
		FromShard: tx.ShardID(),
		ToShard:   tx.ToShardID(),
		Type:      "",
	}, nil
}

// StakingTransaction ...
type StakingTransaction struct {
	Transaction
}

// GetStakingTransaction ...
func GetStakingTransaction(tx *staking.StakingTransaction, addressBlock *types.Block) (*StakingTransaction, error) {
	msg, err := core2.StakingToMessage(tx, addressBlock.Header().Number())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
		return nil, err
	}

	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.GasLimit()))

	var toAddress *common.Address
	// Populate to address of delegate and undelegate staking txns
	// This is needed for supporting received txns support correctly for staking txns history api
	// For other staking txns, there is no to address.
	switch tx.StakingType() {
	case staking.DirectiveDelegate:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
		if err != nil {
			return nil, err
		}
		if _, ok := stkMsg.(*staking.Delegate); !ok {
			return nil, core2.ErrInvalidMsgForStakingDirective
		}
		delegateMsg := stkMsg.(*staking.Delegate)
		if !bytes.Equal(msg.From().Bytes()[:], delegateMsg.DelegatorAddress.Bytes()[:]) {
			return nil, core2.ErrInvalidSender
		}

		toAddress = &delegateMsg.ValidatorAddress
	case staking.DirectiveUndelegate:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveUndelegate)
		if err != nil {
			return nil, err
		}
		if _, ok := stkMsg.(*staking.Undelegate); !ok {
			return nil, core2.ErrInvalidMsgForStakingDirective
		}

		undelegateMsg := stkMsg.(*staking.Undelegate)
		if !bytes.Equal(msg.From().Bytes()[:], undelegateMsg.DelegatorAddress.Bytes()[:]) {
			return nil, core2.ErrInvalidSender
		}

		toAddress = &undelegateMsg.ValidatorAddress
	default:
		break
	}

	to := ""
	if toAddress != nil {
		if to, err = common2.AddressToBech32(*toAddress); err != nil {
			return nil, err
		}
	}

	from := ""
	if from, err = common2.AddressToBech32(msg.From()); err != nil {
		return nil, err
	}

	txn := Transaction{
		ID:        tx.Hash().Hex(),
		Timestamp: strconv.Itoa(int(addressBlock.Time().Int64() * 1000)),
		From:      from,
		To:        to,
		Value:     msg.Value(),
		Bytes:     strconv.Itoa(int(tx.Size())),
		Data:      hex.EncodeToString(msg.Data()),
		GasFee:    gasFee,
		FromShard: tx.ShardID(),
		ToShard:   0,
		Type:      string(msg.Type()),
	}

	return &StakingTransaction{
		Transaction: txn,
	}, nil
}
