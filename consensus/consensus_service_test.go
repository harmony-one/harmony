package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
)

func TestSignAndMarshalConsensusMessage(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	blsPriKey := bls.RandPrivateKey()
	reg := registry.New()
	consensus, err := New(host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey), reg, decider, 3, false)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.SetCurBlockViewID(2)
	consensus.blockHash = [32]byte{}

	msg := &msg_pb.Message{}
	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(msg, blsPriKey)

	if err != nil || len(marshaledMessage) == 0 {
		t.Errorf("Failed to sign and marshal the message: %s", err)
	}
	if len(msg.Signature) == 0 {
		t.Error("No signature is signed on the consensus message.")
	}
}

func TestSetViewID(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandPrivateKey()
	reg := registry.New()
	consensus, err := New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey), reg, decider, 3, false,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}

	height := uint64(1000)
	consensus.SetViewIDs(height)
	if consensus.GetCurBlockViewID() != height {
		t.Errorf("Cannot set consensus ID. Got: %v, Expected: %v", consensus.GetCurBlockViewID(), height)
	}
}

func TestErrors(t *testing.T) {
	e1 := errors.New("e1")
	require.True(t, errors.Is(e1, e1))

	t.Run("wrap", func(t *testing.T) {
		e2 := errors.Wrap(e1, "e2")
		require.True(t, errors.Is(e2, e1))
	})

	t.Run("withMessage", func(t *testing.T) {
		e2 := errors.WithMessage(e1, "e2")
		require.True(t, errors.Is(e2, e1))
	})
}
