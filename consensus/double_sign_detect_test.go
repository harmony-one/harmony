package consensus

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_core "github.com/harmony-one/harmony/crypto/bls/core"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/stretchr/testify/require"
)

const (
	detectHeight = uint64(42)
	detectViewID = uint64(7)
)

func detectTestKeys(t *testing.T, n int) []bls.PublicKeyWrapper {
	t.Helper()
	keys := make([]bls.PublicKeyWrapper, n)
	for i := range keys {
		private := bls_core.SecretKey{}
		private.SetByCSPRNG()
		public := private.GetPublicKey()
		keys[i].Object = public
		copy(keys[i].Bytes[:], public.Serialize())
	}
	return keys
}

func detectCommitMessage(
	keys []bls.PublicKeyWrapper, hash common.Hash,
) *FBFTMessage {
	senders := make([]*bls.PublicKeyWrapper, len(keys))
	for i := range keys {
		senders[i] = &keys[i]
	}
	return &FBFTMessage{
		ViewID:        detectViewID,
		BlockNum:      detectHeight,
		BlockHash:     hash,
		SenderPubkeys: senders,
		Payload:       []byte{0x01},
	}
}

// castCommit puts a ballot on record for every key, the way a counted commit message
// does, so a later message from the same signer is seen as a repeat.
func castCommit(
	t *testing.T, consensus *Consensus, keys []bls.PublicKeyWrapper, hash common.Hash,
) {
	t.Helper()
	senders := make([]*bls.PublicKeyWrapper, len(keys))
	for i := range keys {
		senders[i] = &keys[i]
	}
	private := bls_core.SecretKey{}
	private.SetByCSPRNG()
	_, err := consensus.decider().AddNewVote(
		quorum.Commit, senders, private.SignHash([]byte("payload")),
		hash, detectHeight, detectViewID,
	)
	require.NoError(t, err)
}

func newDetectConsensus(t *testing.T, keys []bls.PublicKeyWrapper) *Consensus {
	t.Helper()
	_, _, consensus, _, err := GenerateConsensusForTesting()
	require.NoError(t, err)
	consensus.decider().UpdateParticipants(keys, []bls.PublicKeyWrapper{})
	return consensus
}

func TestPriorCommitBallotsFirstVoteIsNotSeen(t *testing.T) {
	keys := detectTestKeys(t, 2)
	consensus := newDetectConsensus(t, keys)

	seen, conflicting := consensus.priorCommitBallots(
		detectCommitMessage(keys[:1], common.HexToHash("0xaa")),
	)
	require.False(t, seen, "a signer with no ballot on record is counted normally")
	require.Nil(t, conflicting)
}

func TestPriorCommitBallotsRepeatOfSameBlockIsNotEvidence(t *testing.T) {
	keys := detectTestKeys(t, 2)
	consensus := newDetectConsensus(t, keys)
	hash := common.HexToHash("0xaa")
	castCommit(t, consensus, keys[:1], hash)

	seen, conflicting := consensus.priorCommitBallots(
		detectCommitMessage(keys[:1], hash),
	)
	require.True(t, seen, "the repeat is not counted")
	require.Nil(t, conflicting, "one signer voting twice for one block is not a double sign")
}

func TestPriorCommitBallotsSecondBlockIsEvidence(t *testing.T) {
	keys := detectTestKeys(t, 2)
	consensus := newDetectConsensus(t, keys)
	first := common.HexToHash("0xaa")
	castCommit(t, consensus, keys[:1], first)

	seen, conflicting := consensus.priorCommitBallots(
		detectCommitMessage(keys[:1], common.HexToHash("0xbb")),
	)
	require.True(t, seen)
	require.Len(t, conflicting, 1)
	require.Equal(t, first, conflicting[0].BlockHeaderHash)
}

// A validator holding several keys casts one message covering all of them and the same
// ballot is filed under each key, so the pair of blocks is collected once, not once per
// key held.
func TestPriorCommitBallotsMultiKeySignerCollectsOneBallot(t *testing.T) {
	keys := detectTestKeys(t, 3)
	consensus := newDetectConsensus(t, keys)
	castCommit(t, consensus, keys[:2], common.HexToHash("0xaa"))

	seen, conflicting := consensus.priorCommitBallots(
		detectCommitMessage(keys[:2], common.HexToHash("0xbb")),
	)
	require.True(t, seen)
	require.Len(t, conflicting, 1)
	require.Len(t, conflicting[0].SignerPubKeys, 2, "the ballot carries both keys")
}

// A message where only some keys have voted before is still a repeat that is not
// counted, and the keys that did vote carry the conflict.
func TestPriorCommitBallotsPartiallySeenSenderIsSeen(t *testing.T) {
	keys := detectTestKeys(t, 3)
	consensus := newDetectConsensus(t, keys)
	castCommit(t, consensus, keys[:1], common.HexToHash("0xaa"))

	seen, conflicting := consensus.priorCommitBallots(
		detectCommitMessage(keys[:2], common.HexToHash("0xbb")),
	)
	require.True(t, seen)
	require.Len(t, conflicting, 1)
}

func TestReportDoubleSignWithNothingToReport(t *testing.T) {
	keys := detectTestKeys(t, 2)
	consensus := newDetectConsensus(t, keys)
	msg := detectCommitMessage(keys[:1], common.HexToHash("0xbb"))

	require.NotPanics(t, func() {
		consensus.reportDoubleSign(msg, nil)
	}, "a repeat with no conflict returns before reading the chain")
}

func TestSharedSignerKeys(t *testing.T) {
	keys := detectTestKeys(t, 4)
	serialized := func(idx ...int) []bls.SerializedPublicKey {
		out := make([]bls.SerializedPublicKey, 0, len(idx))
		for _, i := range idx {
			out = append(out, keys[i].Bytes)
		}
		return out
	}

	require.Len(t, sharedSignerKeys(serialized(0, 1), serialized(0, 1)), 2)
	require.Len(t, sharedSignerKeys(serialized(0, 1), serialized(1, 2)), 1)
	require.Empty(t, sharedSignerKeys(serialized(0), serialized(1)))
	require.Empty(t, sharedSignerKeys(nil, serialized(0)))
	require.Empty(t, sharedSignerKeys(serialized(0), nil))
}

// Sorting the keys makes one offence witnessed twice produce records that hash alike,
// which is what lets the beacon chain drop the duplicate.
func TestSortedVoteOrdersKeysAndLeavesInputAlone(t *testing.T) {
	keys := detectTestKeys(t, 3)
	original := []bls.SerializedPublicKey{keys[0].Bytes, keys[1].Bytes, keys[2].Bytes}
	input := make([]bls.SerializedPublicKey, len(original))
	copy(input, original)

	sorted := sortedVote(slash.Vote{SignerPubKeys: input}).SignerPubKeys
	require.Equal(t, original, input, "the caller's slice is untouched")
	for i := 1; i < len(sorted); i++ {
		require.Negative(t, bytes.Compare(sorted[i-1][:], sorted[i][:]))
	}
}
