package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	measure                    = new(big.Int).Div(big.NewInt(2), big.NewInt(3))
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
	state *state.DB, onlyConsider map[common.Address]struct{},
	signers shard.SlotList, didSign bool,
) error {
	epoch := chain.CurrentHeader().Epoch()
	for i := range signers {
		addr := signers[i].EcdsaAddress
		if _, ok := onlyConsider[addr]; !ok {
			continue
		}
		wrapper := state.GetStakingInfo(addr)
		if wrapper == nil {
			return ctxerror.New("validator info not found", common2.MustAddressToBech32(addr), "at root:", chain.CurrentHeader().Root().Hex())
		}
		wrapper.Snapshot.NumBlocksToSign.Add(
			wrapper.Snapshot.NumBlocksToSign, common.Big1,
		)
		if didSign {
			wrapper.Snapshot.NumBlocksSigned.Add(
				wrapper.Snapshot.NumBlocksSigned, common.Big1,
			)
		}
		wrapper.Snapshot.Epoch = epoch
		if err := state.UpdateStakingInfo(addr, wrapper); err != nil {
			return err
		}
	}
	return nil
}

// IncrementValidatorSigningCounts ..
func IncrementValidatorSigningCounts(
	chain engine.ChainReader, header *block.Header,
	shardID uint32, state *state.DB, onlyConsider map[common.Address]struct{},
) error {
	_, signers, missing, err := BallotResult(chain, header, shardID)

	if err != nil {
		return err
	}
	if err := bumpCount(chain, state, onlyConsider, signers, true); err != nil {
		return err
	}
	if err := bumpCount(chain, state, onlyConsider, missing, false); err != nil {
		return err
	}
	return nil
}

// SetInactiveUnavailableValidators sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func SetInactiveUnavailableValidators(
	bc engine.ChainReader, state *state.DB,
	onlyConsider map[common.Address]struct{},
) error {
	addrs, err := bc.ReadActiveValidatorList()
	if err != nil {
		return err
	}

	now := bc.CurrentHeader().Epoch()
	for i := range addrs {

		if _, ok := onlyConsider[addrs[i]]; !ok {
			continue
		}

		snapshot, err := bc.ReadValidatorSnapshot(addrs[i])

		if err != nil {
			return err
		}

		wrapper := state.GetStakingInfo(addrs[i])
		if wrapper == nil {
			return ctxerror.New("validator info not found", common2.MustAddressToBech32(addrs[i]), "at root:", bc.CurrentHeader().Root().Hex())
		}

		stats := wrapper.Snapshot
		snapEpoch := snapshot.Snapshot.Epoch
		snapSigned := snapshot.Snapshot.NumBlocksSigned
		snapToSign := snapshot.Snapshot.NumBlocksToSign

		if d := new(big.Int).Sub(now, snapEpoch); d.Cmp(common.Big1) != 0 {
			return errors.Wrapf(
				errValidatorEpochDeviation, "bc %s, snapshot %s",
				now.String(), snapEpoch.String(),
			)
		}

		signed := new(big.Int).Sub(stats.NumBlocksSigned, snapSigned)
		toSign := new(big.Int).Sub(stats.NumBlocksToSign, snapToSign)

		if signed.Sign() == -1 {
			return errors.Wrapf(
				errNegativeSign, "diff for signed period wrong: stat %s, snapshot %s",
				stats.NumBlocksSigned.String(), snapSigned.String(),
			)
		}

		if toSign.Sign() == -1 {
			return errors.Wrapf(
				errNegativeSign, "diff for toSign period wrong: stat %s, snapshot %s",
				stats.NumBlocksToSign.String(), snapToSign.String(),
			)
		}

		if toSign.Cmp(common.Big0) == 0 {
			return ErrDivByZero
		}

		if r := new(big.Int).Div(signed, toSign); r.Cmp(measure) == -1 {
			wrapper.Active = false
			if err := state.UpdateStakingInfo(addrs[i], wrapper); err != nil {
				return err
			}
		}
	}
	return nil
}
