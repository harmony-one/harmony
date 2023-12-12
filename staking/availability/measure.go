package availability

import (
	"math/big"

	"github.com/harmony-one/harmony/core/state"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	measure = numeric.NewDec(2).Quo(numeric.NewDec(3))

	minCommissionRateEra1 = numeric.MustNewDecFromStr("0.05")
	minCommissionRateEra2 = numeric.MustNewDecFromStr("0.07")
	// ErrDivByZero ..
	ErrDivByZero = errors.New("toSign of availability cannot be 0, mistake in protocol")
)

// Returns the minimum commission rate between the two options.
// The later rate supersedes the earlier rate.
// If neither is applicable, returns 0.
func MinCommissionRate(era1, era2 bool) numeric.Dec {
	if era2 {
		return minCommissionRateEra2
	}
	if era1 {
		return minCommissionRateEra1
	}
	return numeric.ZeroDec()
}

// BlockSigners ..
func BlockSigners(
	bitmap []byte, parentCommittee *shard.Committee,
) (shard.SlotList, shard.SlotList, error) {
	committerKeys, err := parentCommittee.BLSPublicKeys()
	if err != nil {
		return nil, nil, err
	}
	mask := bls.NewMask(committerKeys)
	if err := mask.SetMask(bitmap); err != nil {
		return nil, nil, err
	}

	payable, missing := shard.SlotList{}, shard.SlotList{}

	for idx, member := range parentCommittee.Slots {
		switch signed, err := mask.IndexEnabled(idx); true {
		case err != nil:
			return nil, nil, err
		case signed:
			payable = append(payable, member)
		default:
			missing = append(missing, member)
		}
	}
	return payable, missing, nil
}

// BallotResult returns
// (parentCommittee.Slots, payable, missings, err)
func BallotResult(
	parentHeader, header RoundHeader, parentShardState *shard.State, shardID uint32,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	parentCommittee, err := parentShardState.FindCommitteeByID(shardID)

	if err != nil {
		return nil, nil, nil, errors.Errorf(
			"cannot find shard in the shard state %d %d",
			parentHeader.Number(),
			parentHeader.ShardID(),
		)
	}

	payable, missing, err := BlockSigners(
		header.LastCommitBitmap(), parentCommittee,
	)
	return parentCommittee.Slots, payable, missing, err
}

type signerKind struct {
	didSign   bool
	committee shard.SlotList
}

func bumpCount(
	bc Reader,
	state ValidatorState,
	signers []signerKind,
	stakedAddrSet map[common.Address]struct{},
) error {
	for _, subset := range signers {
		for i := range subset.committee {
			addr := subset.committee[i].EcdsaAddress
			// NOTE if the signer address is not part of the staked addrs,
			// then it must be a harmony operated node running,
			// hence keep on going
			if _, isAddrForStaked := stakedAddrSet[addr]; !isAddrForStaked {
				continue
			}

			wrapper, err := state.ValidatorWrapper(addr, true, false)
			if err != nil {
				return err
			}

			wrapper.Counters.NumBlocksToSign.Add(
				wrapper.Counters.NumBlocksToSign, common.Big1,
			)

			if subset.didSign {
				wrapper.Counters.NumBlocksSigned.Add(
					wrapper.Counters.NumBlocksSigned, common.Big1,
				)
			}
		}
	}

	return nil
}

// IncrementValidatorSigningCounts ..
func IncrementValidatorSigningCounts(
	bc Reader,
	staked *shard.StakedSlots,
	state ValidatorState,
	signers, missing shard.SlotList,
) error {
	return bumpCount(
		bc, state, []signerKind{{false, missing}, {true, signers}},
		staked.LookupSet,
	)
}

// ComputeCurrentSigning returns (signed, toSign, quotient, error)
func ComputeCurrentSigning(
	snapshot, wrapper *staking.ValidatorWrapper,
) *staking.Computed {
	statsNow, snapSigned, snapToSign :=
		wrapper.Counters,
		snapshot.Counters.NumBlocksSigned,
		snapshot.Counters.NumBlocksToSign

	signed, toSign :=
		new(big.Int).Sub(statsNow.NumBlocksSigned, snapSigned),
		new(big.Int).Sub(statsNow.NumBlocksToSign, snapToSign)

	computed := staking.NewComputed(
		signed, toSign, 0, numeric.ZeroDec(), true,
	)

	if toSign.Cmp(common.Big0) == 0 {
		return computed
	}

	if signed.Sign() == -1 {
		// Shouldn't happen
		utils.Logger().Error().Msg("negative number of signed blocks")
	}

	if toSign.Sign() == -1 {
		// Shouldn't happen
		utils.Logger().Error().Msg("negative number of blocks to sign")
	}

	s1, s2 := numeric.NewDecFromBigInt(signed), numeric.NewDecFromBigInt(toSign)
	computed.Percentage = s1.Quo(s2)
	computed.IsBelowThreshold = IsBelowSigningThreshold(computed.Percentage)
	return computed
}

// IsBelowSigningThreshold ..
func IsBelowSigningThreshold(quotient numeric.Dec) bool {
	return quotient.LTE(measure)
}

// ComputeAndMutateEPOSStatus sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func ComputeAndMutateEPOSStatus(
	bc Reader,
	state ValidatorState,
	addr common.Address,
) error {
	utils.Logger().Info().Msg("begin compute for availability")

	wrapper, err := state.ValidatorWrapper(addr, true, false)
	if err != nil {
		return err
	}
	if wrapper.Status == effective.Banned {
		utils.Logger().Debug().Msg("Can't update EPoS status on a banned validator")
		return nil
	}

	snapshot, err := bc.ReadValidatorSnapshot(wrapper.Address)
	if err != nil {
		return err
	}

	computed := ComputeCurrentSigning(snapshot.Validator, wrapper)

	utils.Logger().
		Info().Msg("check if signing percent is meeting required threshold")

	const missedTooManyBlocks = true

	switch computed.IsBelowThreshold {
	case missedTooManyBlocks:
		wrapper.Status = effective.Inactive
		utils.Logger().Info().
			Str("threshold", measure.String()).
			Interface("computed", computed).
			Str("validator", snapshot.Validator.Address.String()).
			Msg("validator failed availability threshold, set to inactive")
	default:
		// Default is no-op so validator who wants
		// to leave the committee can actually leave.
	}

	return nil
}

// UpdateMinimumCommissionFee update the validator commission fee to the minRate
// if the validator has a lower commission rate and promoPeriod epochs have passed after
// the validator was first elected. It returns true if the commission was updated
func UpdateMinimumCommissionFee(
	electionEpoch *big.Int,
	state *state.DB,
	addr common.Address,
	minRate numeric.Dec,
	promoPeriod uint64,
) (bool, error) {
	utils.Logger().Info().Msg("begin update min commission fee")

	wrapper, err := state.ValidatorWrapper(addr, true, false)
	if err != nil {
		return false, err
	}

	firstElectionEpoch := state.GetValidatorFirstElectionEpoch(addr)

	// convert all to uint64 for easy maths
	// this can take decades of time without overflowing
	first := firstElectionEpoch.Uint64()
	election := electionEpoch.Uint64()
	if first != 0 && election-first >= promoPeriod && election >= first {
		if wrapper.Rate.LT(minRate) {
			utils.Logger().Info().
				Str("addr", addr.Hex()).
				Str("old rate", wrapper.Rate.String()).
				Str("firstElectionEpoch", firstElectionEpoch.String()).
				Msg("updating min commission rate")
			wrapper.Rate.SetBytes(minRate.Bytes())
			return true, nil
		}
	}
	return false, nil
}

// UpdateMaxCommissionFee makes sure the max-rate is at least higher than the rate + max-rate-change.
func UpdateMaxCommissionFee(state *state.DB, addr common.Address, minRate numeric.Dec) (bool, error) {
	utils.Logger().Info().Msg("begin update max commission fee")

	wrapper, err := state.ValidatorWrapper(addr, true, false)
	if err != nil {
		return false, err
	}

	minMaxRate := minRate.Add(wrapper.MaxChangeRate)

	if wrapper.MaxRate.LT(minMaxRate) {
		utils.Logger().Info().
			Str("addr", addr.Hex()).
			Str("old max-rate", wrapper.MaxRate.String()).
			Str("new max-rate", minMaxRate.String()).
			Msg("updating max commission rate")
		wrapper.MaxRate.SetBytes(minMaxRate.Bytes())
		return true, nil
	}

	return false, nil
}
