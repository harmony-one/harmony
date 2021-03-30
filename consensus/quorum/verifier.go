package quorum

import (
	"math/big"

	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

// Verifier is the interface to verify the whether the quorum is achieved by mask at each epoch.
// TODO: Add some unit tests to make sure Verifier get exactly the same result as Decider
type Verifier interface {
	IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool
}

// NewVerifier creates the quorum verifier for the given committee, epoch and whether the scenario
// is staking.
func NewVerifier(committee *shard.Committee, epoch *big.Int, isStaking bool) (Verifier, error) {
	if isStaking {
		return newStakeVerifier(committee, epoch)
	}
	return newUniformVerifier(committee)
}

// stakeVerifier is the quorum verifier for staking period. Each validator has staked token as
// a voting power of the final result.
type stakeVerifier struct {
	r votepower.Roster
}

// newStakeVerifier creates a stake verifier from the given committee
func newStakeVerifier(committee *shard.Committee, epoch *big.Int) (*stakeVerifier, error) {
	r, err := votepower.Compute(committee, epoch)
	if err != nil {
		return nil, errors.Wrap(err, "compute staking vote-power")
	}
	return &stakeVerifier{
		r: *r,
	}, nil
}

// IsQuorumAchievedByMask returns whether the quorum is achieved with the provided mask
func (sv *stakeVerifier) IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool {
	if mask == nil {
		return false
	}
	vp := sv.r.VotePowerByMask(mask)
	return vp.GT(sv.threshold())
}

func (sv *stakeVerifier) threshold() numeric.Dec {
	return twoThird
}

// uniformVerifier is the quorum verifier for non-staking period. All nodes has a uniform voting power.
type uniformVerifier struct {
	pubKeyCnt int64
}

func newUniformVerifier(committee *shard.Committee) (*uniformVerifier, error) {
	keys, err := committee.BLSPublicKeys()
	if err != nil {
		return nil, err
	}
	return &uniformVerifier{
		pubKeyCnt: int64(len(keys)),
	}, nil
}

// IsQuorumAchievedByMask returns whether the quorum is achieved with the provided mask,
// which is whether more than (2/3+1) nodes is included in mask.
func (uv *uniformVerifier) IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool {
	got := int64(len(mask.Publics))
	exp := uv.thresholdKeyCount()
	// Theoretically speaking, greater or equal will do the work. But current logic is more strict
	// without equal, thus conform to current logic implemented.
	// (engineImpl.VerifySeal, uniformVoteWeight.IsQuorumAchievedByMask)
	return got > exp
}

func (uv *uniformVerifier) thresholdKeyCount() int64 {
	return uv.pubKeyCnt*2/3 + 1
}
