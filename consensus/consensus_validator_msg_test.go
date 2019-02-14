package consensus

import (
	"testing"

	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestConstructPrepareMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9992"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9995"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	msgBytes := consensus.constructPrepareMessage()

	msg := &msg_pb.Message{}
	if err := protobuf.Unmarshal(msgBytes, msg); err != nil {
		test.Error("Can not parse the message", err)
	} else {
		if msg.GetConsensus() == nil || !consensus.IsValidatorMessage(msg) {
			test.Error("Wrong message")
		}
	}
}

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	msg := consensus.constructCommitMessage([]byte("random string"))

	message := &msg_pb.Message{}
	if err = protobuf.Unmarshal(msg, message); err != nil {
		test.Errorf("Error when unmarshalling a message")
	}
}
