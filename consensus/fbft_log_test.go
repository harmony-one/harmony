package consensus

import (
	"testing"

	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"github.com/harmony-one/harmony/shard"
)

func constructAnnounceMessage(t *testing.T) (*NetworkMessage, error) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKey(blsPriKey), decider,
	)
	if err != nil {
		t.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	return consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, blsPriKey.GetPublicKey(), blsPriKey)
}

func getConsensusMessage(payload []byte) (*msg_pb.Message, error) {
	msg := &msg_pb.Message{}
	if err := protobuf.Unmarshal(payload, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func TestGetMessagesByTypeSeqViewHash(t *testing.T) {
	pbftMsg := FBFTMessage{
		MessageType: msg_pb.MessageType_ANNOUNCE,
		BlockNum:    2,
		ViewID:      3,
		BlockHash:   [32]byte{01, 02},
	}
	log := NewFBFTLog()
	log.AddMessage(&pbftMsg)

	found := log.GetMessagesByTypeSeqViewHash(
		msg_pb.MessageType_ANNOUNCE, 2, 3, [32]byte{01, 02},
	)
	if len(found) != 1 {
		t.Error("cannot find existing message")
	}

	notFound := log.GetMessagesByTypeSeqViewHash(
		msg_pb.MessageType_ANNOUNCE, 2, 3, [32]byte{01, 03},
	)
	if len(notFound) > 0 {
		t.Error("find message that not exist")
	}
}

func TestHasMatchingAnnounce(t *testing.T) {
	pbftMsg := FBFTMessage{
		MessageType: msg_pb.MessageType_ANNOUNCE,
		BlockNum:    2,
		ViewID:      3,
		BlockHash:   [32]byte{01, 02},
	}
	log := NewFBFTLog()
	log.AddMessage(&pbftMsg)
	found := log.HasMatchingViewAnnounce(2, 3, [32]byte{01, 02})
	if !found {
		t.Error("found should be true")
	}

	notFound := log.HasMatchingViewAnnounce(2, 3, [32]byte{02, 02})
	if notFound {
		t.Error("notFound should be false")
	}
}
