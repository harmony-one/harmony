package explorer

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"

	core2 "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

/*
 * All the code here is work of progress for the sprint.
 */

// Tx types ...
const (
	Received = "RECEIVED"
	Sent     = "SENT"
)

// Data ...
type Data struct {
	Addresses []string `json:"Addresses"`
}

// Address ...
type Address struct {
	ID         string                `json:"id"`
	Balance    *big.Int              `json:"balance"`
	TXs        []*Transaction        `json:"txs"`
	StakingTXs []*StakingTransaction `json:"staking_txs"`
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
func GetTransaction(tx *types.Transaction, addressBlock *types.Block) *Transaction {
	msg, err := tx.AsMessage(types.NewEIP155Signer(tx.ChainID()))
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
	}
	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.Gas()))
	to := ""
	if msg.To() != nil {
		if to, err = common2.AddressToBech32(*msg.To()); err != nil {
			return nil
		}
	}
	from := ""
	if from, err = common2.AddressToBech32(msg.From()); err != nil {
		return nil
	}
	return &Transaction{
		ID:        tx.Hash().Hex(),
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
	}
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
	}

	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.Gas()))

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
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
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
