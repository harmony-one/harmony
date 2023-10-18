package core

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/staking/availability"

	"github.com/harmony-one/harmony/internal/params"

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
	addrs []common.Address, state vm.StateDB,
	validator common.Address, identity string, blsKeys []bls.SerializedPublicKey,
) error {
	checkIdentity := identity != ""
	checkBlsKeys := len(blsKeys) != 0

	blsKeyMap := map[bls.SerializedPublicKey]struct{}{}
	for _, key := range blsKeys {
		blsKeyMap[key] = struct{}{}
	}

	for _, addr := range addrs {
		if !bytes.Equal(validator.Bytes(), addr.Bytes()) {
			wrapper, err := state.ValidatorWrapper(addr, true, false)

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
	addrs, err := chainContext.ReadValidatorList()
	if err != nil {
		return nil, err
	}
	if err := checkDuplicateFields(
		addrs, stateDB,
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
	addrs, err := chainContext.ReadValidatorList()
	if err != nil {
		return nil, err
	}
	if err := checkDuplicateFields(
		addrs, stateDB,
		msg.ValidatorAddress,
		msg.Identity,
		newBlsKeys); err != nil {
		return nil, err
	}
	// request a copy, but delegations are not being changed so do not deep copy them
	wrapper, err := stateDB.ValidatorWrapper(msg.ValidatorAddress, false, false)
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

	minRate := availability.MinCommissionRate(
		chainContext.Config().IsMinCommissionRate(epoch),
		chainContext.Config().IsHIP30(epoch),
	)
	if newRate.LT(minRate) {
		firstEpoch := stateDB.GetValidatorFirstElectionEpoch(msg.ValidatorAddress)
		promoPeriod := chainContext.Config().MinCommissionPromoPeriod.Int64()
		if firstEpoch.Uint64() != 0 && big.NewInt(0).Sub(epoch, firstEpoch).Int64() >= promoPeriod {
			return nil,
				errors.Errorf(
					"%s %d%%",
					errCommissionRateChangeTooLowT,
					minRate.MulInt64(100).Int64(),
				)
		}
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
const oneHundred = 100

var (
	oneAsBigInt             = big.NewInt(denominations.One)
	minimumDelegation       = new(big.Int).Mul(oneAsBigInt, big.NewInt(oneThousand))
	minimumDelegationV2     = new(big.Int).Mul(oneAsBigInt, big.NewInt(oneHundred))
	errDelegationTooSmall   = errors.New("minimum delegation amount for a delegator has to be greater than or equal to 1000 ONE")
	errDelegationTooSmallV2 = errors.New("minimum delegation amount for a delegator has to be greater than or equal to 100 ONE")
)

// VerifyAndDelegateFromMsg verifies the delegate message using the stateDB
// and returns the balance to be deducted by the delegator as well as the
// validatorWrapper with the delegation applied to it.
//
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndDelegateFromMsg(
	stateDB vm.StateDB, epoch *big.Int, msg *staking.Delegate, delegations []staking.DelegationIndex, chainConfig *params.ChainConfig,
) ([]*staking.ValidatorWrapper, *big.Int, map[common.Address]*big.Int, error) {
	if stateDB == nil {
		return nil, nil, nil, errStateDBIsMissing
	}
	if !stateDB.IsValidator(msg.ValidatorAddress) {
		return nil, nil, nil, errValidatorNotExist
	}
	if msg.Amount.Sign() == -1 {
		return nil, nil, nil, errNegativeAmount
	}
	if msg.Amount.Cmp(minimumDelegation) < 0 {
		if chainConfig.IsMinDelegation100(epoch) {
			if msg.Amount.Cmp(minimumDelegationV2) < 0 {
				return nil, nil, nil, errDelegationTooSmallV2
			}
		} else {
			return nil, nil, nil, errDelegationTooSmall
		}
	}

	updatedValidatorWrappers := []*staking.ValidatorWrapper{}
	delegateBalance := big.NewInt(0).Set(msg.Amount)
	fromLockedTokens := map[common.Address]*big.Int{}

	var delegateeWrapper *staking.ValidatorWrapper
	if chainConfig.IsRedelegation(epoch) {
		// Check if we can use tokens in undelegation to delegate (redelegate)
		for i := range delegations {
			delegationIndex := &delegations[i]
			// request a copy, and since delegations will be changed, copy them too
			wrapper, err := stateDB.ValidatorWrapper(delegationIndex.ValidatorAddress, false, true)
			if err != nil {
				return nil, nil, nil, err
			}
			if uint64(len(wrapper.Delegations)) <= delegationIndex.Index {
				utils.Logger().Warn().
					Str("validator", delegationIndex.ValidatorAddress.String()).
					Uint64("delegation index", delegationIndex.Index).
					Int("delegations length", len(wrapper.Delegations)).
					Msg("Delegation index out of bound")
				return nil, nil, nil, errors.New("Delegation index out of bound")
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
					return nil, nil, nil, err
				}

				if bytes.Equal(delegationIndex.ValidatorAddress[:], msg.ValidatorAddress[:]) {
					delegateeWrapper = wrapper
				}
				updatedValidatorWrappers = append(updatedValidatorWrappers, wrapper)
				fromLockedTokens[delegationIndex.ValidatorAddress] = big.NewInt(0).Sub(startBalance, delegateBalance)
			}
		}
	}

	if delegateeWrapper == nil {
		var err error
		// request a copy, and since delegations will be changed, copy them too
		delegateeWrapper, err = stateDB.ValidatorWrapper(msg.ValidatorAddress, false, true)
		if err != nil {
			return nil, nil, nil, err
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
				return nil, nil, nil, err
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
			return nil, nil, nil, err
		}
	}

	if delegateBalance.Cmp(big.NewInt(0)) == 0 {
		// delegation fully from undelegated tokens, no need to deduct from balance.
		return updatedValidatorWrappers, big.NewInt(0), fromLockedTokens, nil
	}

	// Still need to deduct tokens from balance for delegation
	// Check if there is enough liquid token to delegate
	if !CanTransfer(stateDB, msg.DelegatorAddress, delegateBalance) {
		return nil, nil, nil, errors.Wrapf(
			errInsufficientBalanceForStake, "totalRedelegatable: %v, balance: %v; trying to stake %v",
			big.NewInt(0).Sub(msg.Amount, delegateBalance), stateDB.GetBalance(msg.DelegatorAddress), msg.Amount)
	}

	return updatedValidatorWrappers, delegateBalance, fromLockedTokens, nil
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

	wrapper, err := stateDB.ValidatorWrapper(msg.ValidatorAddress, false, true)
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

// VerifyAndMigrateFromMsg verifies and transfers all delegations of
// msg.From to msg.To. Returns all modified validator wrappers and delegate msgs
// for metadata
// Note that this function never updates the stateDB, it only reads from stateDB.
func VerifyAndMigrateFromMsg(
	stateDB vm.StateDB,
	msg *staking.MigrationMsg,
	fromDelegations []staking.DelegationIndex,
) ([]*staking.ValidatorWrapper,
	[]interface{},
	error) {
	if bytes.Equal(msg.From.Bytes(), msg.To.Bytes()) {
		return nil, nil, errors.New("From and To are the same address")
	}
	if len(fromDelegations) == 0 {
		return nil, nil, errors.New("No delegations to migrate")
	}
	modifiedWrappers := make([]*staking.ValidatorWrapper, 0)
	stakeMsgs := make([]interface{}, 0)
	// iterate over all delegationIndexes by `From`
	for i := range fromDelegations {
		delegationIndex := &fromDelegations[i]
		// find the wrapper for each delegationIndex
		// request a copy, and since delegations will be changed, copy them too
		wrapper, err := stateDB.ValidatorWrapper(delegationIndex.ValidatorAddress, false, true)
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
		// and then find matching delegation to remove from wrapper
		foundDelegation := &wrapper.Delegations[delegationIndex.Index] // note: pointer
		if !bytes.Equal(foundDelegation.DelegatorAddress.Bytes(), msg.From.Bytes()) {
			return nil, nil, errors.New(fmt.Sprintf("Expected %s but got %s",
				msg.From.Hex(),
				foundDelegation.DelegatorAddress.Hex()))
		}
		// Skip delegations with zero amount and empty undelegation
		if foundDelegation.Amount.Cmp(common.Big0) == 0 && len(foundDelegation.Undelegations) == 0 {
			continue
		}
		delegationAmountToMigrate := big.NewInt(0).Add(foundDelegation.Amount, big.NewInt(0))
		undelegationsToMigrate := foundDelegation.Undelegations
		// when undelegating we don't remove, just set the amount to zero
		// to be coherent, do the same thing here (effective on wrapper since pointer)
		foundDelegation.Amount = big.NewInt(0)
		foundDelegation.Undelegations = make([]staking.Undelegation, 0)
		// find `To` and give it to them
		totalAmount := big.NewInt(0)
		found := false
		for i := range wrapper.Delegations {
			delegation := &wrapper.Delegations[i]
			if bytes.Equal(delegation.DelegatorAddress.Bytes(), msg.To.Bytes()) {
				found = true
				// add to existing delegation
				totalAmount = delegation.Amount.Add(delegation.Amount, delegationAmountToMigrate)
				// and the undelegations
				for _, undelegationToMigrate := range undelegationsToMigrate {
					exist := false
					for _, entry := range delegation.Undelegations {
						if entry.Epoch.Cmp(undelegationToMigrate.Epoch) == 0 {
							exist = true
							entry.Amount.Add(entry.Amount, undelegationToMigrate.Amount)
							break
						}
					}
					if !exist {
						delegation.Undelegations = append(delegation.Undelegations,
							undelegationToMigrate)
					}
				}
				// Always sort the undelegate by epoch in increasing order
				sort.SliceStable(
					delegation.Undelegations,
					func(i, j int) bool {
						return delegation.Undelegations[i].Epoch.Cmp(delegation.Undelegations[j].Epoch) < 0
					},
				)
				break
			}
		}
		if !found { // add the delegation
			wrapper.Delegations = append(
				wrapper.Delegations, staking.NewDelegation(
					msg.To, delegationAmountToMigrate,
				),
			)
			totalAmount = delegationAmountToMigrate
		}
		if err := wrapper.SanityCheck(); err != nil {
			// allow self delegation to go below min self delegation
			// but set the status to inactive
			if errors.Cause(err) == staking.ErrInvalidSelfDelegation {
				wrapper.Status = effective.Inactive
			} else {
				return nil, nil, err
			}
		}
		modifiedWrappers = append(modifiedWrappers, wrapper)
		delegate := &staking.Delegate{
			ValidatorAddress: wrapper.Address,
			DelegatorAddress: msg.To,
			Amount:           totalAmount,
		}
		stakeMsgs = append(stakeMsgs, delegate)
	}
	return modifiedWrappers, stakeMsgs, nil
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
		// request a copy, and since delegations will be changed (.Reward.Set), copy them too
		wrapper, err := stateDB.ValidatorWrapper(delegation.ValidatorAddress, false, true)
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
