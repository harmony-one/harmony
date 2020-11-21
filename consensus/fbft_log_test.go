package consensus

import (
	"bytes"
	"encoding/binary"
	"testing"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/crypto/bls"
)

func TestFBFTLog_id(t *testing.T) {
	tests := []FBFTMessage{
		{
			MessageType: msg_pb.MessageType_ANNOUNCE,
			ViewID:      4,
			BlockHash:   [32]byte{01, 02},
			SenderPubkeys: []*bls.PublicKeyWrapper{&bls.PublicKeyWrapper{
				Bytes: bls.SerializedPublicKey{0x01, 0x02},
			}},
			SenderPubkeyBitmap: []byte{05, 07},
		},
		{
			MessageType:        msg_pb.MessageType_COMMIT,
			ViewID:             4,
			BlockHash:          [32]byte{02, 03},
			SenderPubkeyBitmap: []byte{05, 07},
		},
	}
	for _, msg := range tests {
		id := msg.id()

		if uint32(msg.MessageType) != binary.LittleEndian.Uint32(id[:]) {
			t.Errorf("message type not expected")
		}
		if msg.ViewID != binary.LittleEndian.Uint64(id[idTypeBytes:]) {
			t.Errorf("view id not expected")
		}
		if !bytes.Equal(id[idTypeBytes+idViewIDBytes:idTypeBytes+idViewIDBytes+idHashBytes], msg.BlockHash[:]) {
			t.Errorf("block hash not expected")
		}
		if !msg.HasSingleSender() {
			if !bytes.Equal(id[idTypeBytes+idViewIDBytes+idHashBytes:idTypeBytes+idViewIDBytes+idHashBytes+len(msg.SenderPubkeyBitmap)], msg.SenderPubkeyBitmap[:]) {
				t.Errorf("sender key expected to be the bitmap when key list is not size 1")
			}
		} else {
			if !bytes.Equal(id[idTypeBytes+idViewIDBytes+idHashBytes:], msg.SenderPubkeys[0].Bytes[:]) {
				t.Errorf("sender key not expected")
			}
		}
		if idDup := msg.id(); idDup != id {
			t.Errorf("id not replicable")
		}
		msg.MessageType = 100
		if idDiff := msg.id(); idDiff == id {
			t.Errorf("id not unique")
		}
	}
}

func TestGetMessagesByTypeSeqViewHash(t *testing.T) {
	pbftMsg := FBFTMessage{
		MessageType: msg_pb.MessageType_ANNOUNCE,
		BlockNum:    2,
		ViewID:      3,
		BlockHash:   [32]byte{01, 02},
	}
	log := NewFBFTLog()
	log.AddVerifiedMessage(&pbftMsg)

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
	log.AddVerifiedMessage(&pbftMsg)
	found := log.HasMatchingViewAnnounce(2, 3, [32]byte{01, 02})
	if !found {
		t.Error("found should be true")
	}

	notFound := log.HasMatchingViewAnnounce(2, 3, [32]byte{02, 02})
	if notFound {
		t.Error("notFound should be false")
	}
}
