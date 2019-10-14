package consensus

import (
	"testing"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/values"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestConstructPrepareMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9992"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	consensus, err := New(
		host, values.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	msgBytes := consensus.constructPrepareMessage()
	msgBytes, err = proto.GetConsensusMessagePayload(msgBytes)
	if err != nil {
		test.Error("Error when getting consensus message", "error", err)
	}

	msg := &msg_pb.Message{}
	if err := protobuf.Unmarshal(msgBytes, msg); err != nil {
		test.Error("Can not parse the message", err)
	} else {
		if msg.GetConsensus() == nil {
			test.Error("Wrong message")
		}
	}
}

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	consensus, err := New(
		host, values.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	msg := consensus.constructCommitMessage([]byte("random string"))
	msg, err = proto.GetConsensusMessagePayload(msg)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}

	message := &msg_pb.Message{}
	if err = protobuf.Unmarshal(msg, message); err != nil {
		test.Errorf("Error when unmarshalling a message")
	}
}
