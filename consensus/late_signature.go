package consensus

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/prometheus/client_golang/prometheus"
)

// recordLastCommitSent stores the block number and hash of the last COMMIT
// this node broadcast successfully.
func (consensus *Consensus) recordLastCommitSent(blockNum uint64, blockHash common.Hash) {
	atomic.StoreUint64(&consensus.lastCommitSentBlockNum, blockNum)
	consensus.lastCommitSentHash.Store(blockHash)
}

// checkOwnCommitInclusion logs and counts when a local COMMIT key is absent
// from the final COMMITTED bitmap for a block this node previously signed.
func (consensus *Consensus) checkOwnCommitInclusion(blockNum uint64, blockHash common.Hash, mask *bls.Mask) {
	if mask == nil {
		return
	}
	if atomic.LoadUint64(&consensus.lastCommitSentBlockNum) != blockNum {
		return
	}
	v := consensus.lastCommitSentHash.Load()
	if v == nil {
		return
	}
	sentHash, ok := v.(common.Hash)
	if !ok || sentHash != blockHash {
		return
	}

	priKeys, err := consensus.getPriKeysInCommittee()
	if err != nil || len(priKeys) == 0 {
		return
	}
	localPubs := make([]bls.SerializedPublicKey, 0, len(priKeys))
	for _, key := range priKeys {
		localPubs = append(localPubs, key.Pub.Bytes)
	}

	reportLocalCommitInclusions(mask, localPubs, func(pub bls.SerializedPublicKey) {
		consensus.getLogger().Warn().
			Uint64("blockNum", blockNum).
			Str("blockHash", blockHash.Hex()).
			Str("blsPubKey", pub.Hex()).
			Msg("[OnCommitted] local commit signature not included in final commit bitmap")
	})
}

// reportLocalCommitInclusions counts each local committee key present in the
// COMMITTED participant set toward signature_total, and late_signature when
// the key is disabled in the bitmap.
func reportLocalCommitInclusions(mask *bls.Mask, localPubs []bls.SerializedPublicKey, onMissing func(bls.SerializedPublicKey)) {
	for _, pub := range localPubs {
		ok, err := mask.KeyEnabled(pub)
		if err != nil {
			continue
		}
		consensusSignatureTotalCounterVec.With(prometheus.Labels{
			"role":  "validator",
			"phase": "committed",
		}).Inc()
		if ok {
			continue
		}
		if onMissing != nil {
			onMissing(pub)
		}
		consensusLateSignatureCounterVec.With(prometheus.Labels{
			"role":  "validator",
			"phase": "committed",
		}).Inc()
	}
}

// excludedLocalCommitKeys returns local committee keys that are present in the
// participant set but disabled in the commit bitmap.
func excludedLocalCommitKeys(mask *bls.Mask, localPubs []bls.SerializedPublicKey) []bls.SerializedPublicKey {
	excluded := make([]bls.SerializedPublicKey, 0)
	for _, pub := range localPubs {
		ok, err := mask.KeyEnabled(pub)
		if err != nil {
			continue
		}
		if !ok {
			excluded = append(excluded, pub)
		}
	}
	return excluded
}

// reportAcceptedVote counts a prepare/commit vote accepted on time by the leader.
func (consensus *Consensus) reportAcceptedVote(phase string) {
	consensusSignatureTotalCounterVec.With(prometheus.Labels{
		"role":  "leader",
		"phase": phase,
	}).Inc()
}

// reportLateVoteIfPastFinalized logs and counts a prepare/commit vote whose
// block number is exactly one behind the leader's current block number.
func (consensus *Consensus) reportLateVoteIfPastFinalized(recvMsg *FBFTMessage, myBlockNum uint64) {
	if recvMsg == nil || recvMsg.BlockNum+1 != myBlockNum {
		return
	}
	phase := recvMsg.MessageType.String()
	consensusLateSignatureCounterVec.With(prometheus.Labels{
		"role":  "leader",
		"phase": phase,
	}).Inc()
	consensusSignatureTotalCounterVec.With(prometheus.Labels{
		"role":  "leader",
		"phase": phase,
	}).Inc()

	consensus.getLogger().Info().
		Uint64("msgBlockNum", recvMsg.BlockNum).
		Uint64("myBlockNum", myBlockNum).
		Uint64("msgViewID", recvMsg.ViewID).
		Str("phase", phase).
		Str("recvMsg", recvMsg.String()).
		Msg("[Consensus] late vote received after block finalized")
}
