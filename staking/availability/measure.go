package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	measure                    = numeric.NewDec(2).Quo(numeric.NewDec(3))
	errValidatorEpochDeviation = errors.New(
		"validator snapshot epoch not exactly one epoch behind",
	)
	errNegativeSign = errors.New("impossible period of signing")
	// ErrDivByZero ..
	ErrDivByZero = errors.New("toSign of availability cannot be 0, mistake in protocol")
)

// BlockSigners ..
func BlockSigners(
	bitmap []byte, parentCommittee *shard.Committee,
) (shard.SlotList, shard.SlotList, error) {
	committerKeys, err := parentCommittee.BLSPublicKeys()

	if err != nil {
		return nil, nil, ctxerror.New(
			"cannot convert a BLS public key",
		).WithCause(err)
	}

	mask, err := bls2.NewMask(committerKeys, nil)
	if err != nil {
		return nil, nil, ctxerror.New(
			"cannot create group sig mask",
		).WithCause(err)
	}
	if err := mask.SetMask(bitmap); err != nil {
		return nil, nil, ctxerror.New(
			"cannot set group sig mask bits",
		).WithCause(err)
	}

	payable, missing := shard.SlotList{}, shard.SlotList{}

	for idx, member := range parentCommittee.Slots {
		switch signed, err := mask.IndexEnabled(idx); true {
		case err != nil:
			return nil, nil, ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		case signed:
			payable = append(payable, member)
		default:
			missing = append(missing, member)
		}
	}
	return payable, missing, nil
}

// BallotResult returns
// (parentCommittee.Slots, payable, missing, err)
func BallotResult(
	bc engine.ChainReader, header *block.Header, shardID uint32,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	// TODO ek – retrieving by parent number (blockNum - 1) doesn't work,
	//  while it is okay with hash.  Sounds like DB inconsistency.
	//  Figure out why.
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return nil, nil, nil, ctxerror.New(
			"cannot find parent block header in DB",
			"parentHash", header.ParentHash(),
		)
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return nil, nil, nil, ctxerror.New(
			"cannot read shard state", "epoch", parentHeader.Epoch(),
		).WithCause(err)
	}

	parentCommittee, err := parentShardState.FindCommitteeByID(shardID)

	if err != nil {
		return nil, nil, nil, ctxerror.New(
			"cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number(),
			"shardID", parentHeader.ShardID(),
		)
	}

	payable, missing, err := BlockSigners(
		header.LastCommitBitmap(), parentCommittee,
	)
	return parentCommittee.Slots, payable, missing, err
}

func bumpCount(
	bc Reader,
	state *state.DB,
	signers shard.SlotList,
	didSign bool,
	stakedAddrSet map[common.Address]struct{},
) error {
	for i := range signers {
		addr := signers[i].EcdsaAddress
		// NOTE if the signer address is not part of the staked addrs,
		// then it must be a harmony operated node running,
		// hence keep on going
		if _, isAddrForStaked := stakedAddrSet[addr]; !isAddrForStaked {
			continue
		}

		wrapper, err := state.ValidatorWrapper(addr)
		if err != nil {
			return err
		}

		wrapper.Counters.NumBlocksToSign.Add(
			wrapper.Counters.NumBlocksToSign, common.Big1,
		)

		if didSign {
			wrapper.Counters.NumBlocksSigned.Add(
				wrapper.Counters.NumBlocksSigned, common.Big1,
			)
		}

		if err := compute(bc, state, wrapper); err != nil {
			return err
		}

		if err := state.UpdateValidatorWrapper(
			addr, wrapper,
		); err != nil {
			return err
		}
	}
	return nil
}

// IncrementValidatorSigningCounts ..
func IncrementValidatorSigningCounts(
	bc Reader,
	staked *shard.StakedSlots,
	state *state.DB,
	signers, missing shard.SlotList,
) error {
	if err := bumpCount(
		bc, state, signers, true, staked.LookupSet,
	); err != nil {
		return err
	}
	return bumpCount(bc, state, missing, false, staked.LookupSet)
}

// Reader ..
type Reader interface {
	ReadValidatorSnapshot(
		addr common.Address,
	) (*staking.ValidatorWrapper, error)
}

// ComputeCurrentSigning returns (signed, toSign, quotient, error)
func ComputeCurrentSigning(
	snapshot, wrapper *staking.ValidatorWrapper,
) (*big.Int, *big.Int, numeric.Dec, error) {
	statsNow, snapSigned, snapToSign :=
		wrapper.Counters,
		snapshot.Counters.NumBlocksSigned,
		snapshot.Counters.NumBlocksToSign

	signed, toSign :=
		new(big.Int).Sub(statsNow.NumBlocksSigned, snapSigned),
		new(big.Int).Sub(statsNow.NumBlocksToSign, snapToSign)

	if toSign.Cmp(common.Big0) == 0 {
		utils.Logger().Info().
			Msg("toSign is 0, perhaps did not receive crosslink proving signing")
		return signed, toSign, numeric.ZeroDec(), nil
	}

	if signed.Sign() == -1 {
		return nil, nil, numeric.ZeroDec(), errors.Wrapf(
			errNegativeSign, "diff for signed period wrong: stat %s, snapshot %s",
			statsNow.NumBlocksSigned.String(), snapSigned.String(),
		)
	}

	if toSign.Sign() == -1 {
		return nil, nil, numeric.ZeroDec(), errors.Wrapf(
			errNegativeSign, "diff for toSign period wrong: stat %s, snapshot %s",
			statsNow.NumBlocksToSign.String(), snapToSign.String(),
		)
	}

	s1, s2 :=
		numeric.NewDecFromBigInt(signed), numeric.NewDecFromBigInt(toSign)

	quotient := s1.Quo(s2)
	return signed, toSign, quotient, nil
}

// IsBelowSigningThreshold ..
func IsBelowSigningThreshold(quotient numeric.Dec) bool {
	return quotient.LTE(measure)
}

// compute sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func compute(
	bc Reader,
	state *state.DB,
	wrapper *staking.ValidatorWrapper,
) error {
	utils.Logger().Info().Msg("begin compute for availability")

	snapshot, err := bc.ReadValidatorSnapshot(wrapper.Address)
	if err != nil {
		return err
	}

	signed, toSign, quotient, err := ComputeCurrentSigning(snapshot, wrapper)

	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("signed", signed.String()).
		Str("to-sign", toSign.String()).
		Str("percentage-signed", quotient.String()).
		Bool("meets-threshold", quotient.LTE(measure)).
		Msg("check if signing percent is meeting required threshold")

	const missedTooManyBlocks = true

	switch IsBelowSigningThreshold(quotient) {
	case missedTooManyBlocks:
		wrapper.EPOSStatus = effective.Inactive
		utils.Logger().Info().
			Str("threshold", measure.String()).
			Msg("validator failed availability threshold, set to inactive")
	default:
		wrapper.EPOSStatus = effective.Active
	}

	return nil
}
