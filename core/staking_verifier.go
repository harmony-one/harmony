package core

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core/vm"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	errStateDBIsMissing    = errors.New("no stateDB was provided")
	errChainContextMissing = errors.New("no chain context was provided")
	errEpochMissing        = errors.New("no epoch was provided")
	errBlockNumMissing     = errors.New("no block number was provided")
)

// TODO: add unit tests to check staking msg verification

// VerifyAndCreateValidatorFromMsg verifies the create validator message using
// the stateDB, epoch, & blocknumber and returns the validatorWrapper created
// in the process.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndCreateValidatorFromMsg(
	stateDB vm.StateDB, epoch *big.Int, blockNum *big.Int, msg *staking.CreateValidator,
) (*staking.ValidatorWrapper, error) {

	if stateDB == nil {
		return nil, errStateDBIsMissing
	}
	if epoch == nil {
		return nil, errEpochMissing
	}
	if blockNum == nil {
		return nil, errBlockNumMissing
	}
	if msg.Amount.Sign() == -1 {
		return nil, errNegativeAmount
	}
	if stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, errors.Wrapf(
			errValidatorExist, common2.MustAddressToBech32(msg.ValidatorAddress),
		)
	}
	if !CanTransfer(stateDB, msg.ValidatorAddress, msg.Amount) {
		return nil, errInsufficientBalanceForStake
	}
	v, err := staking.CreateValidatorFromNewMsg(msg, blockNum, epoch)
	if err != nil {
		return nil, err
	}
	wrapper := &staking.ValidatorWrapper{}
	wrapper.Validator = *v
	wrapper.Delegations = []staking.Delegation{
		staking.NewDelegation(v.Address, msg.Amount),
	}
	zero := big.NewInt(0)
	wrapper.Counters.NumBlocksSigned = zero
	wrapper.Counters.NumBlocksToSign = zero
	wrapper.BlockReward = big.NewInt(0)
	maxBLSKeyAllowed := shard.ExternalSlotsAvailableForEpoch(epoch) / 3
	if err := wrapper.SanityCheck(maxBLSKeyAllowed); err != nil {
		return nil, err
	}
	return wrapper, nil
}

// VerifyAndEditValidatorFromMsg verifies the edit validator message using
// the stateDB, chainContext and returns the edited validatorWrapper.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndEditValidatorFromMsg(
	stateDB vm.StateDB, chainContext ChainContext,
	epoch, blockNum *big.Int, msg *staking.EditValidator,
) (*staking.ValidatorWrapper, error) {

	if stateDB == nil {
		return nil, errStateDBIsMissing
	}
	if chainContext == nil {
		return nil, errChainContextMissing
	}
	if blockNum == nil {
		return nil, errBlockNumMissing
	}
	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, errValidatorNotExist
	}
	wrapper, err := stateDB.ValidatorWrapperCopy(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	if err := staking.UpdateValidatorFromEditMsg(&wrapper.Validator, msg, epoch); err != nil {
		return nil, err
	}
	newRate := wrapper.Validator.Rate
	if newRate.GT(wrapper.Validator.MaxRate) {
		return nil, errCommissionRateChangeTooHigh
	}

	snapshotValidator, err := chainContext.ReadValidatorSnapshot(wrapper.Address)
	if err != nil {
		return nil, errors.WithMessage(err, "Validator snapshot not found.")
	}
	rateAtBeginningOfEpoch := snapshotValidator.Validator.Rate

	if rateAtBeginningOfEpoch.IsNil() ||
		(!newRate.IsNil() && !rateAtBeginningOfEpoch.Equal(newRate)) {
		wrapper.Validator.UpdateHeight = blockNum
	}

	if newRate.Sub(rateAtBeginningOfEpoch).Abs().GT(
		wrapper.Validator.MaxChangeRate,
	) {
		return nil, errCommissionRateChangeTooFast
	}
	maxBLSKeyAllowed := shard.ExternalSlotsAvailableForEpoch(epoch) / 3
	if err := wrapper.SanityCheck(maxBLSKeyAllowed); err != nil {
		return nil, err
	}
	return wrapper, nil
}

const oneThousand = 1000

var (
	oneAsBigInt           = big.NewInt(denominations.One)
	minimumDelegation     = new(big.Int).Mul(oneAsBigInt, big.NewInt(oneThousand))
	errDelegationTooSmall = errors.New("minimum delegation amount for a delegator has to be at least 1000 ONE")
)

// VerifyAndDelegateFromMsg verifies the delegate message using the stateDB
// and returns the balance to be deducted by the delegator as well as the
// validatorWrapper with the delegation applied to it.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndDelegateFromMsg(
	stateDB vm.StateDB, msg *staking.Delegate,
) (*staking.ValidatorWrapper, *big.Int, error) {
	if stateDB == nil {
		return nil, nil, errStateDBIsMissing
	}
	if msg.Amount.Sign() == -1 {
		return nil, nil, errNegativeAmount
	}
	if msg.Amount.Cmp(minimumDelegation) < 0 {
		return nil, nil, errDelegationTooSmall
	}
	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, nil, errValidatorNotExist
	}
	wrapper, err := stateDB.ValidatorWrapperCopy(msg.ValidatorAddress)
	if err != nil {
		return nil, nil, err
	}

	// Check if there is enough liquid token to delegate
	if !CanTransfer(stateDB, msg.DelegatorAddress, msg.Amount) {
		return nil, nil, errors.Wrapf(
			errInsufficientBalanceForStake, "had %v, tried to stake %v",
			stateDB.GetBalance(msg.DelegatorAddress), msg.Amount)
	}

	// Check for existing delegation
	for i := range wrapper.Delegations {
		delegation := &wrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), msg.DelegatorAddress.Bytes()) {
			delegation.Amount.Add(delegation.Amount, msg.Amount)
			if err := wrapper.SanityCheck(
				staking.DoNotEnforceMaxBLS,
			); err != nil {
				return nil, nil, err
			}
			return wrapper, msg.Amount, nil
		}
	}

	// Add new delegation
	wrapper.Delegations = append(
		wrapper.Delegations, staking.NewDelegation(
			msg.DelegatorAddress, msg.Amount,
		),
	)
	if err := wrapper.SanityCheck(staking.DoNotEnforceMaxBLS); err != nil {
		return nil, nil, err
	}
	return wrapper, msg.Amount, nil
}

// VerifyAndUndelegateFromMsg verifies the undelegate validator message
// using the stateDB & chainContext and returns the edited validatorWrapper
// with the undelegation applied to it.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndUndelegateFromMsg(
	stateDB vm.StateDB, epoch *big.Int, msg *staking.Undelegate,
) (*staking.ValidatorWrapper, error) {
	if stateDB == nil {
		return nil, errStateDBIsMissing
	}
	if epoch == nil {
		return nil, errEpochMissing
	}

	if msg.Amount.Sign() == -1 {
		return nil, errNegativeAmount
	}

	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, errValidatorNotExist
	}

	wrapper, err := stateDB.ValidatorWrapperCopy(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}

	for i := range wrapper.Delegations {
		delegation := &wrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), msg.DelegatorAddress.Bytes()) {
			if err := delegation.Undelegate(epoch, msg.Amount); err != nil {
				return nil, err
			}
			if err := wrapper.SanityCheck(
				staking.DoNotEnforceMaxBLS,
			); err != nil {
				// allow self delegation to go below min self delegation
				// but set the status to inactive
				if errors.Cause(err) == staking.ErrInvalidSelfDelegation {
					wrapper.Status = effective.Inactive
				} else {
					return nil, err
				}
			}
			return wrapper, nil
		}
	}
	return nil, errNoDelegationToUndelegate
}

// VerifyAndCollectRewardsFromDelegation verifies and collects rewards
// from the given delegation slice using the stateDB. It returns all of the
// edited validatorWrappers and the sum total of the rewards.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndCollectRewardsFromDelegation(
	stateDB vm.StateDB, delegations []staking.DelegationIndex,
) ([]*staking.ValidatorWrapper, *big.Int, error) {
	if stateDB == nil {
		return nil, nil, errStateDBIsMissing
	}
	updatedValidatorWrappers := []*staking.ValidatorWrapper{}
	totalRewards := big.NewInt(0)
	for i := range delegations {
		delegation := &delegations[i]
		wrapper, err := stateDB.ValidatorWrapperCopy(delegation.ValidatorAddress)
		if err != nil {
			return nil, nil, err
		}
		if uint64(len(wrapper.Delegations)) > delegation.Index {
			delegation := &wrapper.Delegations[delegation.Index]
			if delegation.Reward.Cmp(common.Big0) > 0 {
				totalRewards.Add(totalRewards, delegation.Reward)
				delegation.Reward.SetUint64(0)
			}
		} else {
			utils.Logger().Warn().
				Str("validator", delegation.ValidatorAddress.String()).
				Uint64("delegation index", delegation.Index).
				Int("delegations length", len(wrapper.Delegations)).
				Msg("Delegation index out of bound")
			return nil, nil, errors.New("Delegation index out of bound")
		}
		updatedValidatorWrappers = append(updatedValidatorWrappers, wrapper)
	}
	if totalRewards.Int64() == 0 {
		return nil, nil, errNoRewardsToCollect
	}
	return updatedValidatorWrappers, totalRewards, nil
}
