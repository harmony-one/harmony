// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"bytes"
	"errors"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	errInvalidSigner               = errors.New("invalid signer for staking transaction")
	errInsufficientBalanceForGas   = errors.New("insufficient balance to pay for gas")
	errInsufficientBalanceForStake = errors.New("insufficient balance to stake")
	errValidatorExist              = errors.New("staking validator already exists")
	errValidatorNotExist           = errors.New("staking validator does not exist")
	errNoDelegationToUndelegate    = errors.New("no delegation to undelegate")
	errCommissionRateChangeTooFast = errors.New("commission rate can not be changed more than MaxChangeRate within the same epoch")
	errCommissionRateChangeTooHigh = errors.New("commission rate can not be higher than MaxCommissionRate")
	errNoRewardsToCollect          = errors.New("no rewards to collect")
)

/*
StateTransition is the State Transitioning Model which is described as follows:

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
	bc         ChainContext
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	//FromFrontier() (common.Address, error)
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
	Type() types.TransactionType
	BlockNum() *big.Int
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation, homestead bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation && homestead {
		gas = params.TxGasContractCreation
	} else {
		gas = params.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/params.TxDataNonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * params.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			return 0, vm.ErrOutOfGas
		}
		gas += z * params.TxDataZeroGas
	}
	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool, bc ChainContext) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
		bc:       bc,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, uint64, bool, error) {
	return NewStateTransition(evm, msg, gp, nil).TransitionDb()
}

// ApplyStakingMessage computes the new state for staking message
func ApplyStakingMessage(evm *vm.EVM, msg Message, gp *GasPool, bc ChainContext) (uint64, error) {
	return NewStateTransition(evm, msg, gp, bc).StakingTransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return vm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if st.state.GetBalance(st.msg.From()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(st.msg.From(), mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(st.msg.From())

		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the used gas. It returns an error if failed.
// An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, failed bool, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := vm.AccountRef(msg.From())
	homestead := st.evm.ChainConfig().IsS3(st.evm.EpochNumber) // s3 includes homestead
	contractCreation := msg.To() == nil

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation, homestead)
	if err != nil {
		return nil, 0, false, err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, err
	}

	var (
		evm = st.evm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)
	if contractCreation {
		ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
	} else {
		// Increment the nonce for the next transaction
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
	}
	if vmerr != nil {
		utils.Logger().Debug().Err(vmerr).Msg("VM returned with error")
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.

		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, false, vmerr
		}
	}
	st.refundGas()
	// TODO: need to move the gas fee to the general block rewards
	st.state.AddBalance(st.evm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	return ret, st.gasUsed(), vmerr != nil, err
}

func (st *StateTransition) refundGas() {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(st.msg.From(), remaining)

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}

// StakingTransitionDb will transition the state by applying the staking message and
// returning the result including the used gas. It returns an error if failed.
// It is used for staking transaction only
func (st *StateTransition) StakingTransitionDb() (usedGas uint64, err error) {
	if err = st.preCheck(); err != nil {
		return 0, err
	}
	msg := st.msg

	sender := vm.AccountRef(msg.From())
	homestead := st.evm.ChainConfig().IsS3(st.evm.EpochNumber) // s3 includes homestead

	// Pay intrinsic gas
	// TODO: propose staking-specific formula for staking transaction
	gas, err := IntrinsicGas(st.data, false, homestead)
	// TODO Remove this logging
	utils.Logger().Info().Uint64("Using", gas).Msg("Gas cost of staking transaction being processed")

	if err != nil {
		return 0, err
	}
	if err = st.useGas(gas); err != nil {
		return 0, err
	}

	// Increment the nonce for the next transaction
	st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)

	switch msg.Type() {
	case types.StakeNewVal:
		stkMsg := &staking.CreateValidator{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, txn: %+v", msg.Type(), stkMsg)
		if msg.From() != stkMsg.ValidatorAddress {
			return 0, errInvalidSigner
		}
		err = st.applyCreateValidatorTx(stkMsg, msg.BlockNum())

	case types.StakeEditVal:
		stkMsg := &staking.EditValidator{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, txn: %+v", msg.Type(), stkMsg)
		if msg.From() != stkMsg.ValidatorAddress {
			return 0, errInvalidSigner
		}
		err = st.applyEditValidatorTx(stkMsg, msg.BlockNum())

	case types.Delegate:
		stkMsg := &staking.Delegate{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, txn: %+v", msg.Type(), stkMsg)
		if msg.From() != stkMsg.DelegatorAddress {
			return 0, errInvalidSigner
		}
		err = st.applyDelegateTx(stkMsg)

	case types.Undelegate:
		stkMsg := &staking.Undelegate{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, txn: %+v", msg.Type(), stkMsg)
		if msg.From() != stkMsg.DelegatorAddress {
			return 0, errInvalidSigner
		}
		err = st.applyUndelegateTx(stkMsg)
	case types.CollectRewards:
		stkMsg := &staking.CollectRewards{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, txn: %+v", msg.Type(), stkMsg)
		if msg.From() != stkMsg.DelegatorAddress {
			return 0, errInvalidSigner
		}
		err = st.applyCollectRewards(stkMsg)
	default:
		return 0, staking.ErrInvalidStakingKind
	}
	st.refundGas()

	return st.gasUsed(), err
}

func (st *StateTransition) applyCreateValidatorTx(createValidator *staking.CreateValidator, blockNum *big.Int) error {
	if st.state.IsValidator(createValidator.ValidatorAddress) {
		return errValidatorExist
	}

	v, err := staking.CreateValidatorFromNewMsg(createValidator, blockNum)
	if err != nil {
		return err
	}

	delegations := []staking.Delegation{}
	delegations = append(delegations, staking.NewDelegation(v.Address, createValidator.Amount))

	wrapper := staking.ValidatorWrapper{*v, delegations}

	if err := st.state.UpdateStakingInfo(v.Address, &wrapper); err != nil {
		return err
	}
	st.state.SetValidatorFlag(v.Address)
	return nil
}

func (st *StateTransition) applyEditValidatorTx(editValidator *staking.EditValidator, blockNum *big.Int) error {
	if !st.state.IsValidator(editValidator.ValidatorAddress) {
		return errValidatorNotExist
	}

	wrapper := st.state.GetStakingInfo(editValidator.ValidatorAddress)
	if wrapper == nil {
		return errValidatorNotExist
	}

	if err := staking.UpdateValidatorFromEditMsg(&wrapper.Validator, editValidator); err != nil {
		return err
	}
	newRate := wrapper.Validator.Rate

	// TODO: make sure we are reading from the correct snapshot
	snapshotValidator, err := st.bc.ReadValidatorSnapshot(wrapper.Address)
	if err != nil {
		return err
	}
	rateAtBeginningOfEpoch := snapshotValidator.Rate

	if rateAtBeginningOfEpoch.IsNil() || (!newRate.IsNil() && !rateAtBeginningOfEpoch.Equal(newRate)) {
		wrapper.Validator.UpdateHeight = blockNum
	}

	if newRate.Sub(rateAtBeginningOfEpoch).Abs().GT(wrapper.Validator.MaxChangeRate) {
		return errCommissionRateChangeTooFast
	}

	if newRate.GT(wrapper.Validator.MaxRate) {
		return errCommissionRateChangeTooHigh
	}

	if err := st.state.UpdateStakingInfo(wrapper.Address, wrapper); err != nil {
		return err
	}
	return nil
}

func (st *StateTransition) applyDelegateTx(delegate *staking.Delegate) error {
	if !st.state.IsValidator(delegate.ValidatorAddress) {
		return errValidatorNotExist
	}
	wrapper := st.state.GetStakingInfo(delegate.ValidatorAddress)
	if wrapper == nil {
		return errValidatorNotExist
	}

	stateDB := st.state
	delegatorExist := false
	for i := range wrapper.Delegations {
		delegation := &wrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), delegate.DelegatorAddress.Bytes()) {
			delegatorExist = true
			totalInUndelegation := delegation.TotalInUndelegation()
			// If the sum of normal balance and the total amount of tokens in undelegation is greater than the amount to delegate
			if big.NewInt(0).Add(totalInUndelegation, stateDB.GetBalance(delegate.DelegatorAddress)).Cmp(delegate.Amount) >= 0 {
				// Firstly use the tokens in undelegation to delegate (redelegate)
				undelegateAmount := big.NewInt(0).Set(delegate.Amount)
				// Use the latest undelegated token first as it has the longest remaining locking time.
				i := len(delegation.Undelegations) - 1
				for ; i >= 0; i-- {
					if delegation.Undelegations[i].Amount.Cmp(undelegateAmount) <= 0 {
						undelegateAmount.Sub(undelegateAmount, delegation.Undelegations[i].Amount)
					} else {
						delegation.Undelegations[i].Amount.Sub(delegation.Undelegations[i].Amount, undelegateAmount)
						break
					}
				}
				delegation.Undelegations = delegation.Undelegations[:i+1]

				delegation.Amount.Add(delegation.Amount, delegate.Amount)
				err := stateDB.UpdateStakingInfo(wrapper.Validator.Address, wrapper)

				// Secondly, if all locked token are used, try use the balance.
				if err == nil && undelegateAmount.Cmp(big.NewInt(0)) > 0 {
					stateDB.SubBalance(delegate.DelegatorAddress, delegate.Amount)
				}
				return err
			}
			return errInsufficientBalanceForStake
		}
	}

	if !delegatorExist {
		if CanTransfer(stateDB, delegate.DelegatorAddress, delegate.Amount) {
			newDelegator := staking.NewDelegation(delegate.DelegatorAddress, delegate.Amount)
			wrapper.Delegations = append(wrapper.Delegations, newDelegator)

			if err := stateDB.UpdateStakingInfo(wrapper.Validator.Address, wrapper); err == nil {
				stateDB.SubBalance(delegate.DelegatorAddress, delegate.Amount)
			} else {
				return err
			}
		}
	}

	return nil
}

func (st *StateTransition) applyUndelegateTx(undelegate *staking.Undelegate) error {
	if !st.state.IsValidator(undelegate.ValidatorAddress) {
		return errValidatorNotExist
	}
	wrapper := st.state.GetStakingInfo(undelegate.ValidatorAddress)
	if wrapper == nil {
		return errValidatorNotExist
	}

	stateDB := st.state
	delegatorExist := false
	for i := range wrapper.Delegations {
		delegation := &wrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), undelegate.DelegatorAddress.Bytes()) {
			delegatorExist = true

			err := delegation.Undelegate(st.evm.EpochNumber, undelegate.Amount)
			if err != nil {
				return err
			}
			err = stateDB.UpdateStakingInfo(wrapper.Validator.Address, wrapper)
			return err
		}
	}
	if !delegatorExist {
		return errNoDelegationToUndelegate
	}
	return nil
}

func (st *StateTransition) applyCollectRewards(collectRewards *staking.CollectRewards) error {
	if st.bc == nil {
		return errors.New("[CollectRewards] No chain context provided")
	}
	chainContext := st.bc
	delegations, err := chainContext.ReadDelegationsByDelegator(collectRewards.DelegatorAddress)

	if err != nil {
		return err
	}

	totalRewards := big.NewInt(0)
	for i := range delegations {
		wrapper := st.state.GetStakingInfo(delegations[i].ValidatorAddress)
		if wrapper == nil {
			return errValidatorNotExist
		}

		if uint64(len(wrapper.Delegations)) > delegations[i].Index {
			delegation := &wrapper.Delegations[delegations[i].Index]
			if delegation.Reward.Cmp(big.NewInt(0)) > 0 {
				totalRewards.Add(totalRewards, delegation.Reward)
			}

			delegation.Reward.SetUint64(0)
		}

		err = st.state.UpdateStakingInfo(wrapper.Validator.Address, wrapper)
		if err != nil {
			return err
		}
	}
	if totalRewards.Int64() == 0 {
		return errNoRewardsToCollect
	}
	st.state.AddBalance(collectRewards.DelegatorAddress, totalRewards)
	return nil
}
