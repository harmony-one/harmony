package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	measure                    = new(big.Int).Div(big.NewInt(2), big.NewInt(3))
	errValidatorEpochDeviation = errors.New("validator snapshot epoch not exactly one epoch behind")
	errNegativeSign            = errors.New("impossible period of signing")
)

func blockSigners(
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

// BallotResult ..
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

	payable, missing, err := blockSigners(header.LastCommitBitmap(), parentCommittee)
	return parentCommittee.Slots, payable, missing, err
}

// IncrementValidatorSigningCounts ..
func IncrementValidatorSigningCounts(
	bc engine.ChainReader, header *block.Header, shardID uint32, state *state.DB,
) error {
	_, signers, _, err := BallotResult(bc, header, shardID)
	if err != nil {
		return err
	}

	for i := range signers {
		addr := signers[i].EcdsaAddress
		wrapper, err := bc.ReadValidatorInformation(addr)
		if err != nil {
			return err
		}
		one := big.NewInt(1)

		wrapper.Snapshot.NumBlocksToSign.Add(wrapper.Snapshot.NumBlocksToSign, one)
		wrapper.Snapshot.NumBlocksSigned.Add(wrapper.Snapshot.NumBlocksSigned, one)
		wrapper.Snapshot.Epoch = bc.CurrentHeader().Epoch()
		if err := state.UpdateStakingInfo(addr, wrapper); err != nil {
			return err
		}
	}

	return nil
}

// SetInactiveUnavailableValidators sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func SetInactiveUnavailableValidators(
	addrs []common.Address, batch ethdb.Batch, bc engine.ChainReader,
) error {
	one, now := big.NewInt(1), bc.CurrentHeader().Epoch()
	for i := range addrs {
		snapshot, err := bc.ReadValidatorSnapshot(addrs[i])

		if err != nil {
			return err
		}

		wrapper, err := bc.ReadValidatorInformation(addrs[i])

		if err != nil {
			return err
		}

		stats := wrapper.Snapshot
		snapEpoch := snapshot.Snapshot.Epoch
		snapSigned := snapshot.Snapshot.NumBlocksSigned
		snapToSign := snapshot.Snapshot.NumBlocksToSign

		if diff := new(big.Int).Sub(now, snapEpoch); diff.Cmp(one) != 0 {
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

		if r := new(big.Int).Div(signed, toSign); r.Cmp(measure) == -1 {
			wrapper.Active = false
			if writeErr := rawdb.WriteValidatorData(batch, wrapper); writeErr != nil {
				return writeErr
			}
		}
	}
	return nil
}
