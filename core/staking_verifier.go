package core

import (
	"bytes"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core/vm"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
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

func checkDuplicateFields(
	bc ChainContext, state vm.StateDB,
	validator common.Address, identity string, blsKeys []bls.SerializedPublicKey,
) error {
	addrs, err := bc.ReadValidatorList()
	if err != nil {
		return err
	}

	checkIdentity := identity != ""
	checkBlsKeys := len(blsKeys) != 0

	blsKeyMap := map[bls.SerializedPublicKey]struct{}{}
	for _, key := range blsKeys {
		blsKeyMap[key] = struct{}{}
	}

	for _, addr := range addrs {
		if !bytes.Equal(validator.Bytes(), addr.Bytes()) {
			wrapper, err := state.ValidatorWrapperCopy(addr)

			if err != nil {
				return err
			}

			if checkIdentity && wrapper.Identity == identity {
				return errors.Wrapf(errDupIdentity, "duplicate identity %s", identity)
			}
			if checkBlsKeys {
				for _, existingKey := range wrapper.SlotPubKeys {
					if _, ok := blsKeyMap[existingKey]; ok {
						return errors.Wrapf(errDupBlsKey, "duplicate bls key %x", existingKey)
					}
				}
			}
		}
	}
	return nil
}

// TODO: add unit tests to check staking msg verification

// VerifyAndCreateValidatorFromMsg verifies the create validator message using
// the stateDB, epoch, & blocknumber and returns the validatorWrapper created
// in the process.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndCreateValidatorFromMsg(
	stateDB vm.StateDB, chainContext ChainContext, epoch *big.Int, blockNum *big.Int, msg *staking.CreateValidator,
) (*staking.ValidatorWrapper, error) {
	if stateDB == nil {
		return nil, errStateDBIsMissing
	}
	if chainContext == nil {
		return nil, errChainContextMissing
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
	if err := checkDuplicateFields(
		chainContext, stateDB,
		msg.ValidatorAddress,
		msg.Identity,
		msg.SlotPubKeys); err != nil {
		return nil, err
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
	wrapper.Counters.NumBlocksSigned = big.NewInt(0)
	wrapper.Counters.NumBlocksToSign = big.NewInt(0)
	wrapper.BlockReward = big.NewInt(0)
	if err := wrapper.SanityCheck(); err != nil {
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
	newBlsKeys := []bls.SerializedPublicKey{}
	if msg.SlotKeyToAdd != nil {
		newBlsKeys = append(newBlsKeys, *msg.SlotKeyToAdd)
	}
	if err := checkDuplicateFields(
		chainContext, stateDB,
		msg.ValidatorAddress,
		msg.Identity,
		newBlsKeys); err != nil {
		return nil, err
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
		return nil, errors.WithMessage(err, "validator snapshot not found.")
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
	if err := wrapper.SanityCheck(); err != nil {
		return nil, err
	}
	return wrapper, nil
}

const oneThousand = 1000

var (
	oneAsBigInt           = big.NewInt(denominations.One)
	minimumDelegation     = new(big.Int).Mul(oneAsBigInt, big.NewInt(oneThousand))
	errDelegationTooSmall = errors.New("minimum delegation amount for a delegator has to be greater than or equal to 1000 ONE")
)

// VerifyAndDelegateFromMsg verifies the delegate message using the stateDB
// and returns the balance to be deducted by the delegator as well as the
// validatorWrapper with the delegation applied to it.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndDelegateFromMsg(
	stateDB vm.StateDB, epoch *big.Int, msg *staking.Delegate, delegations []staking.DelegationIndex, redelegation bool,
) ([]*staking.ValidatorWrapper, *big.Int, error) {
	if stateDB == nil {
		return nil, nil, errStateDBIsMissing
	}
	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, nil, errValidatorNotExist
	}
	if msg.Amount.Sign() == -1 {
		return nil, nil, errNegativeAmount
	}
	if msg.Amount.Cmp(minimumDelegation) < 0 {
		return nil, nil, errDelegationTooSmall
	}

	updatedValidatorWrappers := []*staking.ValidatorWrapper{}
	delegateBalance := big.NewInt(0).Set(msg.Amount)

	var delegateeWrapper *staking.ValidatorWrapper
	if redelegation {
		// Check if we can use tokens in undelegation to delegate (redelegate)
		for i := range delegations {
			delegationIndex := &delegations[i]
			wrapper, err := stateDB.ValidatorWrapperCopy(delegationIndex.ValidatorAddress)
			if err != nil {
				return nil, nil, err
			}
			if uint64(len(wrapper.Delegations)) <= delegationIndex.Index {
				utils.Logger().Warn().
					Str("validator", delegationIndex.ValidatorAddress.String()).
					Uint64("delegation index", delegationIndex.Index).
					Int("delegations length", len(wrapper.Delegations)).
					Msg("Delegation index out of bound")
				return nil, nil, errors.New("Delegation index out of bound")
			}

			delegation := &wrapper.Delegations[delegationIndex.Index]

			startBalance := big.NewInt(0).Set(delegateBalance)
			// Start from the oldest undelegated tokens
			curIndex := 0
			for ; curIndex < len(delegation.Undelegations); curIndex++ {
				if delegation.Undelegations[curIndex].Epoch.Cmp(epoch) >= 0 {
					break
				}
				if delegation.Undelegations[curIndex].Amount.Cmp(delegateBalance) <= 0 {
					delegateBalance.Sub(delegateBalance, delegation.Undelegations[curIndex].Amount)
				} else {
					delegation.Undelegations[curIndex].Amount.Sub(
						delegation.Undelegations[curIndex].Amount, delegateBalance,
					)
					delegateBalance = big.NewInt(0)
					break
				}
			}

			if startBalance.Cmp(delegateBalance) > 0 {
				// Used undelegated token for redelegation
				delegation.Undelegations = delegation.Undelegations[curIndex:]
				if err := wrapper.SanityCheck(); err != nil {
					return nil, nil, err
				}

				if bytes.Equal(delegationIndex.ValidatorAddress[:], msg.ValidatorAddress[:]) {
					delegateeWrapper = wrapper
				}
				updatedValidatorWrappers = append(updatedValidatorWrappers, wrapper)
			}
		}
	}

	if delegateeWrapper == nil {
		var err error
		delegateeWrapper, err = stateDB.ValidatorWrapperCopy(msg.ValidatorAddress)
		if err != nil {
			return nil, nil, err
		}
		updatedValidatorWrappers = append(updatedValidatorWrappers, delegateeWrapper)
	}

	// Add to existing delegation if any
	found := false
	for i := range delegateeWrapper.Delegations {
		delegation := &delegateeWrapper.Delegations[i]
		if bytes.Equal(delegation.DelegatorAddress.Bytes(), msg.DelegatorAddress.Bytes()) {
			delegation.Amount.Add(delegation.Amount, msg.Amount)
			if err := delegateeWrapper.SanityCheck(); err != nil {
				return nil, nil, err
			}
			found = true
		}
	}

	if !found {
		// Add new delegation
		delegateeWrapper.Delegations = append(
			delegateeWrapper.Delegations, staking.NewDelegation(
				msg.DelegatorAddress, msg.Amount,
			),
		)
		if err := delegateeWrapper.SanityCheck(); err != nil {
			return nil, nil, err
		}
	}

	if delegateBalance.Cmp(big.NewInt(0)) == 0 {
		// delegation fully from undelegated tokens, no need to deduct from balance.
		return updatedValidatorWrappers, big.NewInt(0), nil
	}

	// Still need to deduct tokens from balance for delegation
	// Check if there is enough liquid token to delegate
	if !CanTransfer(stateDB, msg.DelegatorAddress, delegateBalance) {
		return nil, nil, errors.Wrapf(
			errInsufficientBalanceForStake, "totalRedelegatable: %v, balance: %v; trying to stake %v",
			big.NewInt(0).Sub(msg.Amount, delegateBalance), stateDB.GetBalance(msg.DelegatorAddress), msg.Amount)
	}

	return updatedValidatorWrappers, delegateBalance, nil
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
			if err := wrapper.SanityCheck(); err != nil {
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
