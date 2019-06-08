package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus, err := New(host, 0, leader, bls.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}

	message := &msg_pb.Message{}
	msgBytes := consensus.constructAnnounceMessage()
	msgPayload, _ := proto.GetConsensusMessagePayload(msgBytes)

	if err := protobuf.Unmarshal(msgPayload, message); err != nil {
		test.Errorf("Error when creating announce message")
	}
	if message.Type != msg_pb.MessageType_ANNOUNCE {
		test.Error("it did not created announce message")
	}
}

func TestConstructPreparedMessage(test *testing.T) {
	leaderPriKey := bls.RandPrivateKey()
	leaderPubKey := leaderPriKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "6000", ConsensusPubKey: leaderPubKey}

	validatorPriKey := bls.RandPrivateKey()
	validatorPubKey := leaderPriKey.GetPublicKey()
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus, err := New(host, 0, leader, bls.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.ResetState()
	consensus.blockHash = [32]byte{}

	message := "test string"
	consensus.prepareSigs[common.Address{}] = leaderPriKey.Sign(message)
	consensus.prepareSigs[common.Address{}] = validatorPriKey.Sign(message)
	// According to RJ these failures are benign.
	if err := consensus.prepareBitmap.SetKey(leaderPubKey, true); err != nil {
		test.Log(ctxerror.New("prepareBitmap.SetKey").WithCause(err))
	}
	if err := consensus.prepareBitmap.SetKey(validatorPubKey, true); err != nil {
		test.Log(ctxerror.New("prepareBitmap.SetKey").WithCause(err))
	}

	msgBytes, _ := consensus.constructPreparedMessage()
	msgPayload, _ := proto.GetConsensusMessagePayload(msgBytes)

	msg := &msg_pb.Message{}
	if err = protobuf.Unmarshal(msgPayload, msg); err != nil {
		test.Errorf("Error when creating prepared message")
	}
	if msg.Type != msg_pb.MessageType_PREPARED {
		test.Error("it did not created prepared message")
	}
}
