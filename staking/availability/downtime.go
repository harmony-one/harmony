package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	// downtimeSlashThreshold is the signing ratio that separates a validator which took
	// part in the epoch from one which did not. Above it a validator is treated as
	// present, however unreliable, and the loss of its seat and its rewards is the whole
	// consequence. At or below it the validator contributed close to nothing to the
	// shard it was elected to secure, and forfeits stake as well. It sits strictly below
	// the availability threshold so that losing a seat and forfeiting stake stay
	// separate outcomes.
	downtimeSlashThreshold = numeric.NewDec(1).Quo(numeric.NewDec(3))
	// downtimeSlashRate is the share of its own stake a validator forfeits for one epoch
	// spent absent. It is small by design: the seat and the epoch's rewards already
	// account for most of the cost, and the rate is meant to make repeated absence
	// steadily more expensive rather than to settle it in a single epoch.
	downtimeSlashRate = numeric.MustNewDecFromStr("0.001")
)

// DowntimeSlashThreshold returns the signing ratio at or below which an elected
// validator is treated as absent for the epoch.
func DowntimeSlashThreshold() numeric.Dec {
	return downtimeSlashThreshold
}

// DowntimeSlashRate returns the share of self stake forfeited per absent epoch.
func DowntimeSlashRate() numeric.Dec {
	return downtimeSlashRate
}

// ComputeAndMutateDowntimeSlash forfeits a share of an elected validator's own stake
// when its signing ratio for the epoch was at or below the downtime threshold, and
// returns the amount taken. It returns nil when the validator owes nothing.
//
// Only the validator's own delegation is reduced. Uptime is the operator's
// responsibility rather than the delegators', and confining the cost to self stake keeps
// the incentive on the party that can act on it. The amount is burned rather than paid
// out, because absence is read from the block bitmaps by every node alike and so has no
// single witness to reward.
//
// It runs after ComputeAndMutateEPOSStatus for the same validator, which settles the
// EPoS status for the epoch. The minimum self-delegation rule is enforced only while a
// validator is Active, so the status is settled before the stake is reduced.
func ComputeAndMutateDowntimeSlash(
	bc Reader,
	state ValidatorState,
	addr common.Address,
	epoch *big.Int,
) (*big.Int, error) {
	if !bc.Config().IsDowntimeSlash(epoch) {
		return nil, nil
	}

	wrapper, err := state.ValidatorWrapper(addr, true, false)
	if err != nil {
		return nil, err
	}
	// A banned validator is already out of the network permanently and its stake has
	// been settled by the slash that banned it.
	if wrapper.Status == effective.Banned {
		return nil, nil
	}

	snapshot, err := bc.ReadValidatorSnapshot(addr)
	if err != nil {
		return nil, err
	}
	computed := ComputeCurrentSigning(
		snapshot.Validator, wrapper, bc.Config().IsHIP32(snapshot.Epoch),
	)
	if !computed.IsBelowThreshold {
		return nil, nil
	}
	// A validator with no blocks assigned over the epoch has no signing ratio to read,
	// so there is nothing to measure it against.
	if computed.ToSign == nil || computed.ToSign.Sign() <= 0 {
		return nil, nil
	}
	if computed.Percentage.GT(downtimeSlashThreshold) {
		return nil, nil
	}

	slashed, ok := reduceSelfStake(wrapper, downtimeSlashRate)
	if !ok {
		return nil, nil
	}
	state.MarkValidatorWrapperDirty(addr)

	utils.Logger().Info().
		Str("validator", addr.Hex()).
		Str("signing-percentage", computed.Percentage.String()).
		Str("threshold", downtimeSlashThreshold.String()).
		Str("rate", downtimeSlashRate.String()).
		Str("slashed", slashed.String()).
		Uint64("epoch", epoch.Uint64()).
		Msg("validator absent for the epoch, forfeiting a share of self stake")

	return slashed, nil
}

// reduceSelfStake takes the given share of the validator's own delegation and reports
// the amount taken. The second return is false when there is nothing to take, which
// covers a validator holding no self delegation and a stake small enough that the share
// truncates to zero.
//
// The wrapper is left exactly as it was found unless the reduced form satisfies
// SanityCheck, so the caller can mark it dirty knowing the write will be accepted.
func reduceSelfStake(
	wrapper *staking.ValidatorWrapper, rate numeric.Dec,
) (*big.Int, bool) {
	// NOTE invariant: the first delegation is the validator's own stake.
	if len(wrapper.Delegations) == 0 {
		return nil, false
	}
	selfStake := wrapper.Delegations[0].Amount
	if selfStake == nil || selfStake.Sign() <= 0 {
		return nil, false
	}

	slashed := numeric.NewDecFromBigInt(selfStake).Mul(rate).TruncateInt()
	if slashed.Sign() <= 0 {
		return nil, false
	}
	if slashed.Cmp(selfStake) > 0 {
		slashed.Set(selfStake)
	}

	priorStake, priorStatus := new(big.Int).Set(selfStake), wrapper.Status
	selfStake.Sub(selfStake, slashed)
	// The minimum self-delegation rule applies to Active validators, which a validator
	// reaching here has already ceased to be for the coming epoch.
	if wrapper.Status == effective.Active {
		wrapper.Status = effective.Inactive
	}
	if err := wrapper.SanityCheck(); err != nil {
		selfStake.Set(priorStake)
		wrapper.Status = priorStatus
		utils.Logger().Warn().Err(err).
			Str("validator", wrapper.Address.Hex()).
			Msg("leaving self stake unchanged, reduced form would not satisfy the validator rules")
		return nil, false
	}
	return slashed, true
}
