package explorer

import (
	"github.com/ethereum/go-ethereum/common"

	"bytes"
	"encoding/hex"
	"math/big"
	"strconv"

	core2 "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

/*
 * All the code here is work of progress for the sprint.
 */

// Tx types ...
const (
	Received = "RECEIVED"
	Sent     = "SENT"
)

// Errors ...
var (
	// ErrInvalidMsgForStakingDirective is returned if a staking message does not
	// match the related directive
	ErrInvalidMsgForStakingDirective = errors.New("staking message does not match directive message")
	// ErrInvalidSender is returned if a txn or staking txn message has incorrect from address
	ErrInvalidSender = errors.New("staking message does not match directive message")
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

	// CreateValidator

	ValidatorAddress   common.Address          `json:"validator-address"`
	Description        staking.Description     `json:"description"`
	CommissionRates    staking.CommissionRates `json:"commission"`
	MinSelfDelegation  *big.Int                `json:"min-self-delegation"`
	MaxTotalDelegation *big.Int                `json:"max-total-delegation"`
	SlotPubKeys        []shard.BLSPublicKey    `json:"slot-pub-keys"`
	SlotKeySigs        []shard.BLSSignature    `json:"slot-key-sigs"`

	// EditValidator

	// ValidatorAddress common.Address        `json:"validator-address"`
	// Description      staking.Description   `json:"description"`
	Amount         *big.Int     `json:"amount"`
	CommissionRate *numeric.Dec `json:"commission-rate" rlp:"nil"`
	// MinSelfDelegation  *big.Int              `json:"min-self-delegation" rlp:"nil"`
	// MaxTotalDelegation *big.Int              `json:"max-total-delegation" rlp:"nil"`
	SlotKeyToRemove *shard.BLSPublicKey   `json:"slot-key-to_remove" rlp:"nil"`
	SlotKeyToAdd    *shard.BLSPublicKey   `json:"slot-key-to_add" rlp:"nil"`
	SlotKeyToAddSig *shard.BLSSignature   `json:"slot-key-to-add-sig" rlp:"nil"`
	EPOSStatus      effective.Eligibility `json:"epos-eligibility-status" rlp:"nil"`

	// Delegate/Undelegate

	DelegatorAddress common.Address `json:"delegator_address"`
	// ValidatorAddress common.Address `json:"validator_address"`
	// Amount           *big.Int       `json:"amount"`

	// CollectRewards
	// DelegatorAddress common.Address `json:"delegator_address"`
}

// GetStakingTransaction ...
func GetStakingTransaction(tx *staking.StakingTransaction, addressBlock *types.Block) (*StakingTransaction, error) {
	// TODO: legacy. To prevent breaking current clients of explorer staking txns rpc apis, keep unnecessary rlp encoding here
	// Will be removed when the current client (only ostn block explorer) migrates
	msg, err := core2.StakingToMessage(tx, addressBlock.Header().Number())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
	}

	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.Gas()))
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

	switch tx.StakingType() {
	case staking.DirectiveCreateValidator:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCreateValidator)
		if err != nil {
			return nil, err
		}

		if _, ok := stkMsg.(*staking.CreateValidator); !ok {
			return nil, ErrInvalidMsgForStakingDirective
		}

		createValidatorMsg := stkMsg.(*staking.CreateValidator)
		if bytes.Compare(msg.From().Bytes(), createValidatorMsg.ValidatorAddress.Bytes()) != 0 {
			return nil, ErrInvalidSender
		}

		// for fetching sent/received txns history, create validator txn treated as a self transaction
		validatorAddress := ""
		if validatorAddress, err = common2.AddressToBech32(createValidatorMsg.ValidatorAddress); err != nil {
			return nil, err
		}
		txn.To = validatorAddress

		return &StakingTransaction{
			Transaction:        txn,
			ValidatorAddress:   createValidatorMsg.ValidatorAddress,
			Description:        createValidatorMsg.Description,
			CommissionRates:    createValidatorMsg.CommissionRates,
			MinSelfDelegation:  createValidatorMsg.MinSelfDelegation,
			MaxTotalDelegation: createValidatorMsg.MaxTotalDelegation,
			SlotPubKeys:        createValidatorMsg.SlotPubKeys,
			SlotKeySigs:        createValidatorMsg.SlotKeySigs,
			Amount:             createValidatorMsg.Amount,
		}, nil
	case staking.DirectiveEditValidator:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveEditValidator)
		if err != nil {
			return nil, err
		}

		if _, ok := stkMsg.(*staking.EditValidator); !ok {
			return nil, ErrInvalidMsgForStakingDirective
		}

		editValidatorMsg := stkMsg.(*staking.EditValidator)
		if bytes.Compare(msg.From().Bytes(), editValidatorMsg.ValidatorAddress.Bytes()) != 0 {
			return nil, ErrInvalidSender
		}

		// for fetching sent/received txns history, edit validator txn treated as a self transaction
		validatorAddress := ""
		if validatorAddress, err = common2.AddressToBech32(editValidatorMsg.ValidatorAddress); err != nil {
			return nil, err
		}
		txn.To = validatorAddress

		return &StakingTransaction{
			Transaction:        txn,
			ValidatorAddress:   editValidatorMsg.ValidatorAddress,
			Description:        editValidatorMsg.Description,
			CommissionRate:     editValidatorMsg.CommissionRate,
			MinSelfDelegation:  editValidatorMsg.MinSelfDelegation,
			MaxTotalDelegation: editValidatorMsg.MaxTotalDelegation,
			SlotKeyToRemove:    editValidatorMsg.SlotKeyToRemove,
			SlotKeyToAdd:       editValidatorMsg.SlotKeyToAdd,
			SlotKeyToAddSig:    editValidatorMsg.SlotKeyToAddSig,
			EPOSStatus:         editValidatorMsg.EPOSStatus,
		}, nil
	case staking.DirectiveDelegate:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
		if err != nil {
			return nil, err
		}

		if _, ok := stkMsg.(*staking.Delegate); !ok {
			return nil, ErrInvalidMsgForStakingDirective
		}

		delegateMsg := stkMsg.(*staking.Delegate)
		if bytes.Compare(msg.From().Bytes(), delegateMsg.DelegatorAddress.Bytes()) != 0 {
			return nil, ErrInvalidSender
		}

		validatorAddress := ""
		if validatorAddress, err = common2.AddressToBech32(delegateMsg.ValidatorAddress); err != nil {
			return nil, err
		}
		txn.To = validatorAddress

		return &StakingTransaction{
			Transaction:      txn,
			ValidatorAddress: delegateMsg.ValidatorAddress,
			DelegatorAddress: delegateMsg.DelegatorAddress,
			Amount:           delegateMsg.Amount,
		}, nil
	case staking.DirectiveUndelegate:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveUndelegate)
		if err != nil {
			return nil, err
		}

		if _, ok := stkMsg.(*staking.Undelegate); !ok {
			return nil, ErrInvalidMsgForStakingDirective
		}

		undelegateMsg := stkMsg.(*staking.Undelegate)
		if bytes.Compare(msg.From().Bytes(), undelegateMsg.DelegatorAddress.Bytes()) != 0 {
			return nil, ErrInvalidSender
		}

		validatorAddress := ""
		if validatorAddress, err = common2.AddressToBech32(undelegateMsg.ValidatorAddress); err != nil {
			return nil, err
		}
		txn.To = validatorAddress

		return &StakingTransaction{
			Transaction:      txn,
			ValidatorAddress: undelegateMsg.ValidatorAddress,
			DelegatorAddress: undelegateMsg.DelegatorAddress,
			Amount:           undelegateMsg.Amount,
		}, nil
	case staking.DirectiveCollectRewards:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCollectRewards)
		if err != nil {
			return nil, err
		}

		if _, ok := stkMsg.(*staking.CollectRewards); !ok {
			return nil, ErrInvalidMsgForStakingDirective
		}

		collectRewardsMsg := stkMsg.(*staking.CollectRewards)
		if bytes.Compare(msg.From().Bytes(), collectRewardsMsg.DelegatorAddress.Bytes()) != 0 {
			return nil, ErrInvalidSender
		}

		// for fetching sent/received txns history, collect reward txn treated as a self transaction
		delegatorAddress := ""
		if delegatorAddress, err = common2.AddressToBech32(collectRewardsMsg.DelegatorAddress); err != nil {
			return nil, err
		}
		txn.To = delegatorAddress

		return &StakingTransaction{
			Transaction:      txn,
			DelegatorAddress: collectRewardsMsg.DelegatorAddress,
		}, nil
	default:
		return nil, staking.ErrInvalidStakingKind
	}
}
