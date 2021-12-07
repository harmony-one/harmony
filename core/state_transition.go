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
	"fmt"
	"math"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	staking2 "github.com/harmony-one/harmony/staking"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	errInvalidSigner               = errors.New("invalid signer for staking transaction")
	errInsufficientBalanceForGas   = errors.New("insufficient balance to pay for gas")
	errInsufficientBalanceForStake = errors.New("insufficient balance to stake")
	errValidatorExist              = errors.New("staking validator already exists")
	errValidatorNotExist           = errors.New("staking validator does not exist")
	errNoDelegationToUndelegate    = errors.New("no delegation to undelegate")
	errCommissionRateChangeTooFast = errors.New("change on commission rate can not be more than max change rate within the same epoch")
	errCommissionRateChangeTooHigh = errors.New("commission rate can not be higher than maximum commission rate")
	errCommissionRateChangeTooLow  = errors.New("commission rate can not be lower than min rate of 5%")
	errNoRewardsToCollect          = errors.New("no rewards to collect")
	errNegativeAmount              = errors.New("amount can not be negative")
	errDupIdentity                 = errors.New("validator identity exists")
	errDupBlsKey                   = errors.New("BLS key exists")
)

var aaPrefix = [...]byte{
	0x33, 0x73, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0x14, 0x60, 0x24, 0x57, 0x36, 0x60, 0x1f, 0x57, 0x00, 0x5b, 0x60, 0x00, 0x80, 0xfd, 0x5b,
}

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
	IsAA() bool
	Sponsor() common.Address
	Type() types.TransactionType
	BlockNum() *big.Int
}

// ExecutionResult is the return value from a transaction committed to the DB
type ExecutionResult struct {
	ReturnData []byte
	UsedGas    uint64
	VMErr      error
}

// Unwrap returns the internal evm error which allows us for further
// analysis outside.
func (result *ExecutionResult) Unwrap() error {
	return result.VMErr
}

// Failed returns the indicator whether the execution is successful or not
func (result *ExecutionResult) Failed() bool { return result.VMErr != nil }

// Return is a helper function to help caller distinguish between revert reason
// and function return. Return returns the data after execution if no error occurs.
func (result *ExecutionResult) Return() []byte {
	if result.VMErr != nil {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// Revert returns the concrete revert reason if the execution is aborted by `REVERT`
// opcode. Note the reason can be nil if no data supplied with revert opcode.
func (result *ExecutionResult) Revert() []byte {
	if result.VMErr != vm.ErrExecutionReverted {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation, homestead, istanbul, isValidatorCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation && homestead {
		gas = params.TxGasContractCreation
	} else if isValidatorCreation {
		gas = params.TxGasValidatorCreation
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
		nonZeroGas := params.TxDataNonZeroGasFrontier
		if istanbul {
			nonZeroGas = params.TxDataNonZeroGasEIP2028
		}
		if (math.MaxUint64-gas)/nonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * nonZeroGas

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
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) (ExecutionResult, error) {
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
		return ErrIntrinsicGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	var mgval *big.Int
	if !st.msg.IsAA() {
		mgval = new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
		if have := st.state.GetBalance(st.msg.From()); have.Cmp(mgval) < 0 {
			return errors.Wrapf(
				errInsufficientBalanceForGas,
				"had: %s but need: %s", have.String(), mgval.String(),
			)
		}
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	if !st.msg.IsAA() {
		st.state.SubBalance(st.msg.Sponsor(), mgval)
	}

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
func (st *StateTransition) TransitionDb() (ExecutionResult, error) {
	if st.msg.IsAA() {
		if st.msg.Nonce() != 0 || st.msg.GasPrice().Sign() != 0 || st.msg.Value().Sign() != 0 {
			return ExecutionResult{}, ErrInvalidAATransaction
		}

		code := st.evm.StateDB.GetCode(st.to())
		if code == nil || len(code) < len(aaPrefix) {
			return ExecutionResult{}, ErrInvalidAAPrefix
		}
		for i := range aaPrefix {
			if code[i] != aaPrefix[i] {
				return ExecutionResult{}, ErrInvalidAAPrefix
			}
		}
		if st.evm.PaygasMode == vm.PaygasNoOp {
			st.evm.PaygasMode = vm.PaygasContinue
		}
	}

	if err := st.preCheck(); err != nil {
		return ExecutionResult{}, err
	}
	msg := st.msg
	sender := vm.AccountRef(msg.From())
	homestead := st.evm.ChainConfig().IsS3(st.evm.EpochNumber) // s3 includes homestead
	istanbul := st.evm.ChainConfig().IsIstanbul(st.evm.EpochNumber)
	contractCreation := msg.To() == nil

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation, homestead, istanbul, false)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err = st.useGas(gas); err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: have %d, want %d", ErrIntrinsicGas, st.gas, gas)
	}

	evm := st.evm

	var ret []byte
	// All VM errors are valid except for insufficient balance, therefore returned separately
	var vmErr error

	if contractCreation {
		ret, _, st.gas, vmErr = evm.Create(sender, st.data, st.gas, st.value)
	} else {
		if !msg.IsAA() {
			// Increment the nonce for the next transaction
			st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		}
		ret, st.gas, vmErr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
		if msg.IsAA() && st.evm.PaygasMode != vm.PaygasNoOp {
			if vmErr != nil {
				return ExecutionResult{}, vmErr
			} else {
				return ExecutionResult{}, ErrNoPaygas
			}
		}
	}
	if vmErr != nil {
		utils.Logger().Debug().Err(vmErr).Msg("VM returned with error")
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.

		if vmErr == vm.ErrInsufficientBalance {
			return ExecutionResult{}, vmErr
		}
	}
	if st.msg.IsAA() {
		st.gasPrice = st.evm.GasPrice
	}
	st.refundGas()

	// Burn Txn Fees after staking epoch
	if !st.evm.ChainConfig().IsStaking(st.evm.EpochNumber) {
		txFee := new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)
		st.state.AddBalance(st.evm.Coinbase, txFee)
	}

	return ExecutionResult{
		ReturnData: ret,
		UsedGas:    st.gasUsed(),
		VMErr:      vmErr,
	}, err
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
	st.state.AddBalance(st.msg.Sponsor(), remaining)

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
	istanbul := st.evm.ChainConfig().IsIstanbul(st.evm.EpochNumber)

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, false, homestead, istanbul, msg.Type() == types.StakeCreateVal)

	if err != nil {
		return 0, err
	}
	if err = st.useGas(gas); err != nil {
		return 0, err
	}

	// Increment the nonce for the next transaction
	st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)

	switch msg.Type() {
	case types.StakeCreateVal:
		stkMsg := &staking.CreateValidator{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().
			Msgf("[DEBUG STAKING] staking type: %s, gas: %d, txn: %+v", msg.Type(), gas, stkMsg)
		if msg.From() != stkMsg.ValidatorAddress {
			return 0, errInvalidSigner
		}
		err = st.verifyAndApplyCreateValidatorTx(stkMsg, msg.BlockNum())
	case types.StakeEditVal:
		stkMsg := &staking.EditValidator{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().
			Msgf("[DEBUG STAKING] staking type: %s, gas: %d, txn: %+v", msg.Type(), gas, stkMsg)
		if msg.From() != stkMsg.ValidatorAddress {
			return 0, errInvalidSigner
		}
		err = st.verifyAndApplyEditValidatorTx(stkMsg, msg.BlockNum())
	case types.Delegate:
		stkMsg := &staking.Delegate{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, gas: %d, txn: %+v", msg.Type(), gas, stkMsg)
		if msg.From() != stkMsg.DelegatorAddress {
			return 0, errInvalidSigner
		}
		err = st.verifyAndApplyDelegateTx(stkMsg)
	case types.Undelegate:
		stkMsg := &staking.Undelegate{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, gas: %d, txn: %+v", msg.Type(), gas, stkMsg)
		if msg.From() != stkMsg.DelegatorAddress {
			return 0, errInvalidSigner
		}
		err = st.verifyAndApplyUndelegateTx(stkMsg)
	case types.CollectRewards:
		stkMsg := &staking.CollectRewards{}
		if err = rlp.DecodeBytes(msg.Data(), stkMsg); err != nil {
			return 0, err
		}
		utils.Logger().Info().Msgf("[DEBUG STAKING] staking type: %s, gas: %d, txn: %+v", msg.Type(), gas, stkMsg)
		if msg.From() != stkMsg.DelegatorAddress {
			return 0, errInvalidSigner
		}
		_, err = st.verifyAndApplyCollectRewards(stkMsg)
	default:
		return 0, staking.ErrInvalidStakingKind
	}
	st.refundGas()

	// Burn Txn Fees
	//txFee := new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)
	//st.state.AddBalance(st.evm.Coinbase, txFee)

	return st.gasUsed(), err
}

func (st *StateTransition) verifyAndApplyCreateValidatorTx(
	createValidator *staking.CreateValidator, blockNum *big.Int,
) error {
	wrapper, err := VerifyAndCreateValidatorFromMsg(
		st.state, st.bc, st.evm.EpochNumber, blockNum, createValidator,
	)
	if err != nil {
		return err
	}
	if err := st.state.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		return err
	}
	st.state.SetValidatorFlag(createValidator.ValidatorAddress)
	st.state.SubBalance(createValidator.ValidatorAddress, createValidator.Amount)
	return nil
}

func (st *StateTransition) verifyAndApplyEditValidatorTx(
	editValidator *staking.EditValidator, blockNum *big.Int,
) error {
	wrapper, err := VerifyAndEditValidatorFromMsg(
		st.state, st.bc, st.evm.EpochNumber, blockNum, editValidator,
	)
	if err != nil {
		return err
	}
	return st.state.UpdateValidatorWrapper(wrapper.Address, wrapper)
}

func (st *StateTransition) verifyAndApplyDelegateTx(delegate *staking.Delegate) error {
	delegations, err := st.bc.ReadDelegationsByDelegator(delegate.DelegatorAddress)
	if err != nil {
		return err
	}
	updatedValidatorWrappers, balanceToBeDeducted, fromLockedTokens, err := VerifyAndDelegateFromMsg(
		st.state, st.evm.EpochNumber, delegate, delegations, st.evm.ChainConfig())
	if err != nil {
		return err
	}

	for _, wrapper := range updatedValidatorWrappers {
		if err := st.state.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
			return err
		}
	}

	st.state.SubBalance(delegate.DelegatorAddress, balanceToBeDeducted)

	if len(fromLockedTokens) > 0 {
		sortedKeys := []common.Address{}
		for key := range fromLockedTokens {
			sortedKeys = append(sortedKeys, key)
		}
		sort.SliceStable(sortedKeys, func(i, j int) bool {
			return bytes.Compare(sortedKeys[i][:], sortedKeys[j][:]) < 0
		})
		// Add log if everything is good
		for _, key := range sortedKeys {
			redelegatedToken, ok := fromLockedTokens[key]
			if !ok {
				return errors.New("Key missing for delegation receipt")
			}
			encodedRedelegationData := []byte{}
			addrBytes := key.Bytes()
			encodedRedelegationData = append(encodedRedelegationData, addrBytes...)
			encodedRedelegationData = append(encodedRedelegationData, redelegatedToken.Bytes()...)
			// The data field format is:
			// [first 20 bytes]: Validator address from which the locked token is used for redelegation.
			// [rest of the bytes]: the bigInt serialized bytes for the token amount.
			st.state.AddLog(&types.Log{
				Address:     delegate.DelegatorAddress,
				Topics:      []common.Hash{staking2.DelegateTopic},
				Data:        encodedRedelegationData,
				BlockNumber: st.evm.BlockNumber.Uint64(),
			})
		}
	}
	return nil
}

func (st *StateTransition) verifyAndApplyUndelegateTx(
	undelegate *staking.Undelegate,
) error {
	wrapper, err := VerifyAndUndelegateFromMsg(st.state, st.evm.EpochNumber, undelegate)
	if err != nil {
		return err
	}
	return st.state.UpdateValidatorWrapper(wrapper.Address, wrapper)
}

func (st *StateTransition) verifyAndApplyCollectRewards(collectRewards *staking.CollectRewards) (*big.Int, error) {
	if st.bc == nil {
		return stakingReward.None, errors.New("[CollectRewards] No chain context provided")
	}
	delegations, err := st.bc.ReadDelegationsByDelegator(collectRewards.DelegatorAddress)
	if err != nil {
		return stakingReward.None, err
	}
	updatedValidatorWrappers, totalRewards, err := VerifyAndCollectRewardsFromDelegation(
		st.state, delegations,
	)
	if err != nil {
		return stakingReward.None, err
	}
	for _, wrapper := range updatedValidatorWrappers {
		if err := st.state.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
			return stakingReward.None, err
		}
	}
	st.state.AddBalance(collectRewards.DelegatorAddress, totalRewards)

	// Add log if everything is good
	st.state.AddLog(&types.Log{
		Address:     collectRewards.DelegatorAddress,
		Topics:      []common.Hash{staking2.CollectRewardsTopic},
		Data:        totalRewards.Bytes(),
		BlockNumber: st.evm.BlockNumber.Uint64(),
	})

	return totalRewards, nil
}
