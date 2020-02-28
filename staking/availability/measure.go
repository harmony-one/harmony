package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	measure                    = new(big.Int).Div(common.Big2, common.Big3)
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
	committerKeys := []*bls.PublicKey{}

	for _, member := range parentCommittee.Slots {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, nil, ctxerror.New(
				"cannot convert BLS public key",
				"blsPublicKey",
				member.BlsPublicKey,
			).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
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
	parentCommittee := parentShardState.FindCommitteeByID(shardID)

	if parentCommittee == nil {
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
	chain engine.ChainReader,
	state *state.DB,
	signers shard.SlotList,
	didSign bool,
	stakedAddrSet map[common.Address]struct{},
) error {
	l := utils.Logger().Info()
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

		l.RawJSON("validator", []byte(wrapper.String())).
			Msg("about to adjust counters")

		wrapper.Counters.NumBlocksToSign.Add(
			wrapper.Counters.NumBlocksToSign, common.Big1,
		)

		if didSign {
			wrapper.Counters.NumBlocksSigned.Add(
				wrapper.Counters.NumBlocksSigned, common.Big1,
			)
		}

		l.RawJSON("validator", []byte(wrapper.String())).
			Msg("bumped signing counters")

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
	chain engine.ChainReader, header *block.Header,
	shardID uint32, state *state.DB,
	stakedAddrSet map[common.Address]struct{},
) error {
	l := utils.Logger().Info().Str("candidate-header", header.String())
	_, signers, missing, err := BallotResult(chain, header, shardID)
	if err != nil {
		return err
	}
	l.Msg("bumping signing counters for non-missing signers")
	if err := bumpCount(
		chain, state, signers, true, stakedAddrSet,
	); err != nil {
		return err
	}
	l.Msg("bumping missed signing counter ")
	return bumpCount(chain, state, missing, false, stakedAddrSet)
}

// SetInactiveUnavailableValidators sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func SetInactiveUnavailableValidators(
	bc engine.ChainReader, state *state.DB,
) error {
	addrs, err := bc.ReadElectedValidatorList()
	if err != nil {
		return err
	}

	now := bc.CurrentHeader().Epoch()
	for i := range addrs {
		snapshot, err := bc.ReadValidatorSnapshot(addrs[i])
		if err != nil {
			return err
		}

		wrapper, err := state.ValidatorWrapper(addrs[i])

		if err != nil {
			return err
		}

		statsNow, snapEpoch, snapSigned, snapToSign :=
			wrapper.Counters,
			snapshot.LastEpochInCommittee,
			snapshot.Counters.NumBlocksSigned,
			snapshot.Counters.NumBlocksToSign

		l := utils.Logger().Info().
			RawJSON("snapshot", []byte(snapshot.String())).
			RawJSON("current", []byte(wrapper.String()))

		l.Msg("begin checks for availability")

		if snapEpoch.Cmp(common.Big0) == 0 {
			l.Msg("pass newly joined validator for inactivity check")
			continue
		}

		if d := new(big.Int).Sub(now, snapEpoch); d.Cmp(common.Big1) != 0 {
			return errors.Wrapf(
				errValidatorEpochDeviation, "bc %s, snapshot %s",
				now.String(), snapEpoch.String(),
			)
		}

		signed, toSign :=
			new(big.Int).Sub(statsNow.NumBlocksSigned, snapSigned),
			new(big.Int).Sub(statsNow.NumBlocksToSign, snapToSign)

		if signed.Sign() == -1 {
			return errors.Wrapf(
				errNegativeSign, "diff for signed period wrong: stat %s, snapshot %s",
				statsNow.NumBlocksSigned.String(), snapSigned.String(),
			)
		}

		if toSign.Sign() == -1 {
			return errors.Wrapf(
				errNegativeSign, "diff for toSign period wrong: stat %s, snapshot %s",
				statsNow.NumBlocksToSign.String(), snapToSign.String(),
			)
		}

		if toSign.Cmp(common.Big0) == 0 {
			return ErrDivByZero
		}

		if r := new(big.Int).Div(signed, toSign); r.Cmp(measure) == -1 {
			wrapper.Active = false
			l.Str("threshold", measure.String()).
				Msg("validator failed availability threshold, set to inactive")
			if err := state.UpdateValidatorWrapper(addrs[i], wrapper); err != nil {
				return err
			}
		}
	}

	return nil
}
