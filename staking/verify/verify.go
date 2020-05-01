package verify

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	errQuorumVerifyAggSign = errors.New("insufficient voting power to verify aggreate sig")
	errAggregateSigFail    = errors.New("could not verify hash of aggregate signature")
)

// AggregateSigForCommittee ..
func AggregateSigForCommittee(
	chain *core.BlockChain,
	committee *shard.Committee,
	aggSignature *bls.Sign,
	hash common.Hash,
	blockNum, viewID uint64,
	epoch *big.Int,
	bitmap []byte,
) error {
	committerKeys, err := committee.BLSPublicKeys()
	if err != nil {
		return err
	}
	mask, err := bls_cosi.NewMask(committerKeys, nil)
	if err != nil {
		return err
	}
	if err := mask.SetMask(bitmap); err != nil {
		return err
	}

	decider := quorum.NewDecider(
		quorum.SuperMajorityStake, committee.ShardID,
	)
	decider.SetMyPublicKeyProvider(func() (*multibls.PublicKey, error) {
		return nil, nil
	})
	if _, err := decider.SetVoters(committee, epoch); err != nil {
		return err
	}
	if !decider.IsQuorumAchievedByMask(mask) {
		return errQuorumVerifyAggSign
	}

	commitPayload := signature.ConstructCommitPayload(chain, epoch, hash, blockNum, viewID)
	if !aggSignature.VerifyHash(mask.AggregatePublic, commitPayload) {
		return errAggregateSigFail
	}

	return nil
}
