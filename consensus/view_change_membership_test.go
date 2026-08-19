package consensus

import (
	"encoding/binary"
	"testing"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/require"
)

// TestOutsiderSignatureBreaksM3Aggregate shows why a view change signature is
// only usable from a committee member. The new view message carries an aggregate
// of the collected signatures together with a bitmap of who signed, and the
// bitmap can only name committee members. A signature from outside the committee
// therefore ends up in the aggregate without a matching bit, and the aggregate no
// longer verifies against the key the bitmap describes.
func TestOutsiderSignatureBreaksM3Aggregate(t *testing.T) {
	viewID := uint64(42)
	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, viewID)

	// A committee of two.
	members := multibls.PublicKeys{}
	priKeys := []*bls_core.SecretKey{}
	for i := 0; i < 2; i++ {
		pri := bls.RandPrivateKey()
		pub := bls.PublicKeyWrapper{Object: pri.GetPublicKey()}
		require.NoError(t, pub.Bytes.FromLibBLSPublicKey(pub.Object))
		members = append(members, pub)
		priKeys = append(priKeys, pri)
	}

	mask := bls_cosi.NewMask(members)
	sigs := []*bls_core.Sign{}
	for i, pri := range priKeys {
		sigs = append(sigs, pri.SignHash(viewIDBytes))
		require.NoError(t, mask.SetKey(members[i].Bytes, true))
	}

	// Members only: the aggregate matches the bitmap.
	require.True(t,
		bls_cosi.AggregateSig(sigs).VerifyHash(mask.AggregatePublic, viewIDBytes),
		"aggregate of committee signatures should verify against the bitmap",
	)

	// An outsider signs the same view id. Its bit cannot be set, since it is not
	// in the committee the bitmap is built from.
	outsider := bls.RandPrivateKey()
	outsiderPub := bls.PublicKeyWrapper{Object: outsider.GetPublicKey()}
	require.NoError(t, outsiderPub.Bytes.FromLibBLSPublicKey(outsiderPub.Object))
	require.Error(t, mask.SetKey(outsiderPub.Bytes, true))

	withOutsider := append(sigs, outsider.SignHash(viewIDBytes))
	require.False(t,
		bls_cosi.AggregateSig(withOutsider).VerifyHash(mask.AggregatePublic, viewIDBytes),
		"an outsider signature in the aggregate should stop it verifying",
	)
}

// TestViewChangeSanityCheckRejectsNonCommitteeSender exercises the actual
// admission check rather than only the aggregate-signature failure it prevents.
func TestViewChangeSanityCheckRejectsNonCommitteeSender(t *testing.T) {
	memberKey := bls.RandPrivateKey()
	member := bls.PublicKeyWrapper{Object: memberKey.GetPublicKey()}
	require.NoError(t, member.Bytes.FromLibBLSPublicKey(member.Object))
	outsiderKey := bls.RandPrivateKey()
	outsider := bls.PublicKeyWrapper{Object: outsiderKey.GetPublicKey()}
	require.NoError(t, outsider.Bytes.FromLibBLSPublicKey(outsider.Object))

	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	decider.UpdateParticipants([]bls.PublicKeyWrapper{member}, nil)
	reg := registry.New()
	reg.SetQuorum(decider)
	consensus := &Consensus{
		ShardID:  shard.BeaconChainShardID,
		current:  NewState(Normal, shard.BeaconChainShardID),
		registry: reg,
	}

	viewID := uint64(1)
	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, viewID)
	message := func(key bls.PublicKeyWrapper, signature *bls_core.Sign) *FBFTMessage {
		return &FBFTMessage{
			ViewID:        viewID,
			SenderPubkeys: []*bls.PublicKeyWrapper{&key},
			ViewidSig:     signature,
		}
	}

	require.True(t, consensus.onViewChangeSanityCheck(message(member, memberKey.SignHash(viewIDBytes))))
	require.False(t, consensus.onViewChangeSanityCheck(message(outsider, outsiderKey.SignHash(viewIDBytes))))
}
