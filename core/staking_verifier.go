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
) error {

	if stateDB == nil {
		return errStateDBIsMissing
	}
	if epoch == nil {
		return errEpochMissing
	}
	if blockNum == nil {
		return errBlockNumMissing
	}
	if msg.Amount.Sign() == -1 {
		return errNegativeAmount
	}
	if stateDB.IsValidator(msg.ValidatorAddress) {
		return errors.Wrapf(
			errValidatorExist, common2.MustAddressToBech32(msg.ValidatorAddress),
		)
	}
	if !CanTransfer(stateDB, msg.ValidatorAddress, msg.Amount) {
		return errInsufficientBalanceForStake
	}
	v, err := staking.CreateValidatorFromNewMsg(msg, blockNum, epoch)
	if err != nil {
		return err
	}
	wrapper, err := stateDB.NewValidatorWrapper(msg.ValidatorAddress)
	if err != nil {
		return err
	}
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
		return err
	}
	return nil
}

// VerifyAndEditValidatorFromMsg verifies the edit validator message using
// the stateDB, chainContext and returns the edited validatorWrapper.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndEditValidatorFromMsg(
	stateDB vm.StateDB, chainContext ChainContext,
	epoch, blockNum *big.Int, msg *staking.EditValidator,
) error {

	if stateDB == nil {
		return errStateDBIsMissing
	}
	if chainContext == nil {
		return errChainContextMissing
	}
	if blockNum == nil {
		return errBlockNumMissing
	}
	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return errValidatorNotExist
	}
	wrapper, err := stateDB.ValidatorWrapper(msg.ValidatorAddress)
	if err != nil {
		return err
	}
	if err := staking.UpdateValidatorFromEditMsg(&wrapper.Validator, msg, epoch); err != nil {
		return err
	}
	newRate := wrapper.Validator.Rate
	if newRate.GT(wrapper.Validator.MaxRate) {
		return errCommissionRateChangeTooHigh
	}

	snapshotValidator, err := chainContext.ReadValidatorSnapshot(wrapper.Address)
	if err != nil {
		return errors.WithMessage(err, "Validator snapshot not found.")
	}
	rateAtBeginningOfEpoch := snapshotValidator.Validator.Rate

	if rateAtBeginningOfEpoch.IsNil() ||
		(!newRate.IsNil() && !rateAtBeginningOfEpoch.Equal(newRate)) {
		wrapper.Validator.UpdateHeight = blockNum
	}

	if newRate.Sub(rateAtBeginningOfEpoch).Abs().GT(
		wrapper.Validator.MaxChangeRate,
	) {
		return errCommissionRateChangeTooFast
	}
	maxBLSKeyAllowed := shard.ExternalSlotsAvailableForEpoch(epoch) / 3
	if err := wrapper.SanityCheck(maxBLSKeyAllowed); err != nil {
		return err
	}
	return nil
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
) (*big.Int, error) {
	if stateDB == nil {
		return nil, errStateDBIsMissing
	}
	if msg.Amount.Sign() == -1 {
		return nil, errNegativeAmount
	}
	if msg.Amount.Cmp(minimumDelegation) < 0 {
		return nil, errDelegationTooSmall
	}
	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, errValidatorNotExist
	}
	wrapper, err := stateDB.ValidatorWrapper(msg.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	// Check for redelegation
	for i := range wrapper.Delegations {
		delegation := &wrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), msg.DelegatorAddress.Bytes()) {
			totalInUndelegation := delegation.TotalInUndelegation()
			balance := stateDB.GetBalance(msg.DelegatorAddress)
			// If the sum of normal balance and the total amount of tokens in undelegation is greater than the amount to delegate
			if big.NewInt(0).Add(totalInUndelegation, balance).Cmp(msg.Amount) >= 0 {
				// Check if it can use tokens in undelegation to delegate (redelegate)
				delegateBalance := big.NewInt(0).Set(msg.Amount)
				// Use the latest undelegated token first as it has the longest remaining locking time.
				i := len(delegation.Undelegations) - 1
				for ; i >= 0; i-- {
					if delegation.Undelegations[i].Amount.Cmp(delegateBalance) <= 0 {
						delegateBalance.Sub(delegateBalance, delegation.Undelegations[i].Amount)
					} else {
						delegation.Undelegations[i].Amount.Sub(
							delegation.Undelegations[i].Amount, delegateBalance,
						)
						delegateBalance = big.NewInt(0)
						break
					}
				}
				delegation.Undelegations = delegation.Undelegations[:i+1]
				delegation.Amount.Add(delegation.Amount, msg.Amount)
				if err := wrapper.SanityCheck(
					staking.DoNotEnforceMaxBLS,
				); err != nil {
					return nil, err
				}
				if delegateBalance.Cmp(big.NewInt(0)) < 0 {
					return nil, errNegativeAmount // shouldn't really happen
				}
				// Return remaining balance to be deducted for delegation
				if !CanTransfer(stateDB, msg.DelegatorAddress, delegateBalance) {
					return nil, errors.Wrapf(
						errInsufficientBalanceForStake, "had %v, tried to stake %v",
						stateDB.GetBalance(msg.DelegatorAddress), delegateBalance)
				}
				return delegateBalance, nil
			}
			return nil, errors.Wrapf(
				errInsufficientBalanceForStake,
				"total-delegated %s own-current-balance %s amount-to-delegate %s",
				totalInUndelegation.String(),
				balance.String(),
				msg.Amount.String(),
			)
		}
	}
	// If no redelegation, create new delegation
	if !CanTransfer(stateDB, msg.DelegatorAddress, msg.Amount) {
		return nil, errors.Wrapf(
			errInsufficientBalanceForStake, "had %v, tried to stake %v",
			stateDB.GetBalance(msg.DelegatorAddress), msg.Amount)
	}
	wrapper.Delegations = append(
		wrapper.Delegations, staking.NewDelegation(
			msg.DelegatorAddress, msg.Amount,
		),
	)
	if err := wrapper.SanityCheck(staking.DoNotEnforceMaxBLS); err != nil {
		return nil, err
	}
	return msg.Amount, nil
}

// VerifyAndUndelegateFromMsg verifies the undelegate validator message
// using the stateDB & chainContext and returns the edited validatorWrapper
// with the undelegation applied to it.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndUndelegateFromMsg(
	stateDB vm.StateDB, epoch *big.Int, msg *staking.Undelegate,
) error {
	if stateDB == nil {
		return errStateDBIsMissing
	}
	if epoch == nil {
		return errEpochMissing
	}

	if msg.Amount.Sign() == -1 {
		return errNegativeAmount
	}

	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return errValidatorNotExist
	}

	wrapper, err := stateDB.ValidatorWrapper(msg.ValidatorAddress)
	if err != nil {
		return err
	}

	for i := range wrapper.Delegations {
		delegation := &wrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), msg.DelegatorAddress.Bytes()) {
			if err := delegation.Undelegate(epoch, msg.Amount); err != nil {
				return err
			}
			if err := wrapper.SanityCheck(
				staking.DoNotEnforceMaxBLS,
			); err != nil {
				// allow self delegation to go below min self delegation
				// but set the status to inactive
				if errors.Cause(err) == staking.ErrInvalidSelfDelegation {
					wrapper.Status = effective.Inactive
				} else {
					return err
				}
			}
			return nil
		}
	}
	return errNoDelegationToUndelegate
}

// VerifyAndCollectRewardsFromDelegation verifies and collects rewards
// from the given delegation slice using the stateDB. It returns all of the
// edited validatorWrappers and the sum total of the rewards.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndCollectRewardsFromDelegation(
	stateDB vm.StateDB, delegations []staking.DelegationIndex,
) (*big.Int, error) {
	if stateDB == nil {
		return nil, errStateDBIsMissing
	}
	totalRewards := big.NewInt(0)
	for i := range delegations {
		delegation := &delegations[i]
		wrapper, err := stateDB.ValidatorWrapper(delegation.ValidatorAddress)
		if err != nil {
			return nil, err
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
			return nil, errors.New("Delegation index out of bound")
		}
	}
	if totalRewards.Int64() == 0 {
		return nil, errNoRewardsToCollect
	}
	return totalRewards, nil
}
