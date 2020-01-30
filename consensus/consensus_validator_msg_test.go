package consensus

import (
	"testing"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"github.com/harmony-one/harmony/shard"
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
		host, shard.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	_, err = consensus.construct(msg_pb.MessageType_PREPARE)

	if err != nil {
		test.Error("Error when getting consensus message", "error", err)
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
		host, shard.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
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
