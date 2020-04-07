package consensus

import (
	"testing"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
)

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
