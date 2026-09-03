package consensus

import (
	"encoding/binary"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	bls_core "github.com/harmony-one/harmony/crypto/bls/core"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/test/helpers"
	"github.com/stretchr/testify/require"
)

type newViewCommittee struct {
	priKeys []*bls_core.SecretKey
	pubKeys []bls.PublicKeyWrapper
}

func newViewTestCommittee(t *testing.T, n int) newViewCommittee {
	t.Helper()
	c := newViewCommittee{}
	for i := 0; i < n; i++ {
		pri := bls.RandPrivateKey()
		pub := bls.PublicKeyWrapper{Object: pri.GetPublicKey()}
		require.NoError(t, pub.Bytes.FromLibBLSPublicKey(pub.Object))
		c.priKeys = append(c.priKeys, pri)
		c.pubKeys = append(c.pubKeys, pub)
	}
	return c
}

func newViewTestConsensus(t *testing.T, c newViewCommittee) *Consensus {
	t.Helper()
	host, _, err := helpers.GenerateHost(helpers.Hosts[0].IP, helpers.Hosts[0].Port)
	require.NoError(t, err)

	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	decider.UpdateParticipants(c.pubKeys, nil)
	reg := registry.New()
	priWrapper := bls.PrivateKeyWrapper{Pri: c.priKeys[0], Pub: &c.pubKeys[0]}
	consensus, err := New(
		host, shard.BeaconChainShardID,
		multibls.PrivateKeys{priWrapper},
		reg, decider, 3, false,
	)
	require.NoError(t, err)
	consensus.setLeaderPubKey(&c.pubKeys[0])
	consensus.setBlockNum(1)
	consensus.current.SetMode(ViewChanging)
	return consensus
}

func m3Certificate(t *testing.T, c newViewCommittee, viewID uint64) (*bls_core.Sign, *bls_cosi.Mask) {
	t.Helper()
	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, viewID)
	mask := bls_cosi.NewMask(c.pubKeys)
	var sigs []*bls_core.Sign
	for i, pri := range c.priKeys {
		sigs = append(sigs, pri.SignHash(viewIDBytes))
		require.NoError(t, mask.SetKey(c.pubKeys[i].Bytes, true))
	}
	return bls_cosi.AggregateSig(sigs), mask
}

func newViewMessage(
	c newViewCommittee, viewID uint64, payload []byte,
	m3Sig *bls_core.Sign, m3Mask *bls_cosi.Mask,
) *FBFTMessage {
	return &FBFTMessage{
		ViewID:        viewID,
		BlockNum:      1,
		SenderPubkeys: []*bls.PublicKeyWrapper{&c.pubKeys[0]},
		Payload:       payload,
		M3AggSig:      m3Sig,
		M3Bitmap:      m3Mask,
	}
}

// TestOnNewViewRejectsShortM1Payload checks that M1 payload shorter than
// ValidPayloadLength is rejected. The hash is the first 32 bytes.
func TestOnNewViewRejectsShortM1Payload(t *testing.T) {
	c := newViewTestCommittee(t, 3)
	consensus := newViewTestConsensus(t, c)
	viewID := uint64(1)
	m3Sig, m3Mask := m3Certificate(t, c, viewID)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("onNewView panicked on a short M1 payload: %v", r)
		}
	}()
	consensus.onNewView(newViewMessage(c, viewID, []byte{0x01}, m3Sig, m3Mask))
	require.True(t, consensus.isViewChangingMode(),
		"invalid NEWVIEW must not take the group out of view change")
}

// TestOnNewViewRejectsMinorityPrepare checks that an M1 prepare aggregate
// must meet quorum, as in ProcessViewChangeMsg.
func TestOnNewViewRejectsMinorityPrepare(t *testing.T) {
	c := newViewTestCommittee(t, 3)
	consensus := newViewTestConsensus(t, c)
	viewID := uint64(1)
	m3Sig, m3Mask := m3Certificate(t, c, viewID)

	blockHash := common.HexToHash("0x01")
	prepSig := c.priKeys[0].SignHash(blockHash[:])
	prepMask := bls_cosi.NewMask(c.pubKeys)
	require.NoError(t, prepMask.SetKey(c.pubKeys[0].Bytes, true))
	require.False(t, consensus.decider().IsQuorumAchievedByMask(prepMask),
		"a single prepare vote is below quorum for this committee")

	payload := append(blockHash[:], prepSig.Serialize()...)
	payload = append(payload, prepMask.Bitmap...)

	// Payload is long enough to parse as M1 and has no block, so
	// VerifyNewViewMsg returns no prepared block. onNewView still
	// requires prepare quorum on the M1 payload.
	consensus.onNewView(newViewMessage(c, viewID, payload, m3Sig, m3Mask))
	require.True(t, consensus.isViewChangingMode(),
		"NEWVIEW with a minority prepare must not complete the view change")
}

func TestParseCommitSigAndBitmapRoundTrip(t *testing.T) {
	// M1 payload layout is hash || signature || bitmap.
	c := newViewTestCommittee(t, 1)
	hash := common.HexToHash("0xab")
	sig := c.priKeys[0].SignHash(hash[:])
	mask := bls_cosi.NewMask(c.pubKeys)
	require.NoError(t, mask.SetKey(c.pubKeys[0].Bytes, true))
	payload := append(hash[:], sig.Serialize()...)
	payload = append(payload, mask.Bitmap...)

	gotSig, gotMask, err := chain.ReadSignatureBitmapByPublicKeys(payload[32:], c.pubKeys)
	require.NoError(t, err)
	require.True(t, gotSig.VerifyHash(gotMask.AggregatePublic, hash[:]))
}
