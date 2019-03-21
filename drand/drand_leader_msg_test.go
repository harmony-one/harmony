package drand

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestConstructInitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "55555"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, true, bls.RandPrivateKey())
	dRand.blockHash = [32]byte{}
	msg := dRand.constructInitMessage()

	msgPayload, _ := proto.GetDRandMessagePayload(msg)

	message := &msg_pb.Message{}
	err = protobuf.Unmarshal(msgPayload, message)

	if err != nil {
		test.Error("Error in extracting Init message from payload", err)
	}
}

func TestProcessCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "55555"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, true, bls.RandPrivateKey())
	dRand.blockHash = [32]byte{}
	msg := dRand.constructCommitMessage([32]byte{}, []byte{})

	msgPayload, _ := proto.GetDRandMessagePayload(msg)

	message := &msg_pb.Message{}
	err = protobuf.Unmarshal(msgPayload, message)

	if err != nil {
		test.Error("Error in extracting Commit message from payload", err)
	}
}
