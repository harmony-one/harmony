package consensus

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/crypto/bls"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNew(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus, err := New(host, 0, leader, bls.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	if consensus.consensusID != 0 {
		test.Errorf("Consensus Id is initialized to the wrong value: %d", consensus.consensusID)
	}

	if !consensus.IsLeader {
		test.Error("Consensus should belong to a leader")
	}

	if consensus.ReadySignal == nil {
		test.Error("Consensus ReadySignal should be initialized")
	}
}

func TestGetPeerFromID(t *testing.T) {
	leaderPriKey := bls.RandPrivateKey()
	leaderPubKey := leaderPriKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902", ConsensusPubKey: leaderPubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := New(host, 0, leader, leaderPriKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	leaderAddress := utils.GetAddressFromBlsPubKey(leader.ConsensusPubKey)
	l := consensus.GetPeerByAddress(leaderAddress.Hex())
	if l.IP != leader.IP || l.Port != leader.Port {
		t.Errorf("leader IP not equal")
	}
}

func TestPopulateMessageFields(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(host, 0, leader, blsPriKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.consensusID = 2
	consensus.blockHash = blockHash

	msg := &msg_pb.Message{
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}
	consensusMsg := msg.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	if consensusMsg.ConsensusId != 2 {
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
	consensus, err := New(host, 0, leader, bls.RandPrivateKey())
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.consensusID = 2
	consensus.blockHash = blockHash
	consensus.SelfAddress = common.Address{}

	msg := &msg_pb.Message{}
	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(msg)

	if err != nil || len(marshaledMessage) == 0 {
		t.Errorf("Failed to sign and marshal the message: %s", err)
	}
	if len(msg.Signature) == 0 {
		t.Error("No signature is signed on the consensus message.")
	}
}
