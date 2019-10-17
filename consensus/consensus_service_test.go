package consensus

import (
	"bytes"
	"testing"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/values"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestPopulateMessageFields(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	blsPriKey := bls.RandPrivateKey()
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	consensus, err := New(
		host, values.BeaconChainShardID, leader, blsPriKey, decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.viewID = 2
	blockHash := [32]byte{}
	consensus.blockHash = blockHash

	msg := &msg_pb.Message{
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}
	consensusMsg := msg.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	if consensusMsg.ViewId != 2 {
		t.Errorf("Consensus ID is not populated correctly")
	}
	if !bytes.Equal(consensusMsg.BlockHash[:], blockHash[:]) {
		t.Errorf("Block hash is not populated correctly")
	}
	if bytes.Compare(consensusMsg.SenderPubkey, blsPriKey.GetPublicKey().Serialize()) != 0 {
		t.Errorf("Sender ID is not populated correctly")
	}
}

func TestSignAndMarshalConsensusMessage(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	consensus, err := New(
		host, values.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.viewID = 2
	consensus.blockHash = [32]byte{}

	msg := &msg_pb.Message{}
	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(msg)

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
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	consensus, err := New(
		host, values.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}

	height := uint64(1000)
	consensus.SetViewID(height)
	if consensus.viewID != height {
		t.Errorf("Cannot set consensus ID. Got: %v, Expected: %v", consensus.viewID, height)
	}
}
