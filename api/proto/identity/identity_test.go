package identity

import (
	"strings"
	"testing"

	"github.com/harmony-one/harmony/api/proto"
)

func TestRegisterIdentityMessage(t *testing.T) {
	registerIdentityMessage := ConstructIdentityMessage(Register, []byte("registerIdentityMessage"))
	msgPayload, err := proto.GetMessagePayload(registerIdentityMessage)
	messageType, err := GetIdentityMessageType(msgPayload)
	if err != nil {
		t.Errorf("Error thrown in geting message type")
	}
	if messageType != Register {
		t.Error("Message type expected ", Register, " actual ", messageType)
	}
}

func TestAcknowledgeIdentityMessage(t *testing.T) {
	registerAcknowledgeMessage := ConstructIdentityMessage(Acknowledge, []byte("acknowledgeIdentityMsgPayload"))
	msgPayload, err := proto.GetMessagePayload(registerAcknowledgeMessage)
	messageType, err := GetIdentityMessageType(msgPayload)
	if err != nil {
		t.Errorf("Error thrown in geting message type")
	}
	if messageType != Acknowledge {
		t.Error("Message type expected ", Acknowledge, " actual ", messageType)
	}
}

func TestInvalidIdentityMessage(t *testing.T) {
	registerInvalidMessage := ConstructIdentityMessage(3, []byte("acknowledgeIdentityMsgPayload"))
	registerInvalidMessagePayload, err := GetIdentityMessagePayload(registerInvalidMessage)
	if err != nil {
		t.Errorf("Error thrown in geting message type from invalid message")
	}
	_ = registerInvalidMessagePayload
	messageType, err := GetIdentityMessageType(registerInvalidMessage)
	if err != nil {
		t.Errorf("Error thrown in geting message type from invalid message")
	}
	if strings.Compare(messageType.String(), "Unknown") != 0 {
		t.Error("Unknown message type expected for Invalid identity message")
	}
}

func TestEmptyMessage(t *testing.T) {
	messageType, err := GetIdentityMessageType([]byte(""))
	if err == nil {
		t.Error("Error expected in getting message type from empty message")
	}
	if messageType != 0 {
		t.Error("Message type expected", 0, " actual ", messageType)
	}
}
