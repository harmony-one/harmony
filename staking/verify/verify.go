package verify

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
)

// AggregateSigForCommittee ..
func AggregateSigForCommittee(
	committee *shard.Committee,
	aggSignature *bls.Sign,
	hash common.Hash,
	blockNum uint64,
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

	decider := quorum.NewDecider(quorum.SuperMajorityStake)
	decider.SetMyPublicKeyProvider(func() (*multibls.PublicKey, error) {
		return nil, nil
	})
	if _, err := decider.SetVoters(committee.Slots); err != nil {
		return err
	}
	// TODO make this one give back how much there is so far
	if !decider.IsQuorumAchievedByMask(mask) {
		// TODO proper error
		return nil
	}

	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, blockNum)
	commitPayload := append(blockNumBytes, hash[:]...)
	if !aggSignature.VerifyHash(mask.AggregatePublic, commitPayload) {
		// TODO proper error
		return nil
	}

	return nil
}
