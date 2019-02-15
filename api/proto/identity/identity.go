package identity

import (
	"bytes"
	"errors"

	"github.com/harmony-one/harmony/api/proto"
)

// IdentityMessageTypeBytes is the number of bytes consensus message type occupies
const IdentityMessageTypeBytes = 1

// IDMessageType is the identity message type.
type IDMessageType byte

// Constants of IdentityMessageType.
const (
	Identity IDMessageType = iota
	// TODO: add more types
)

// MessageType ...
type MessageType int

// Constants of MessageType.
const (
	Register MessageType = iota
	Acknowledge
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"Register",
		"Acknowledge",
	}

	if msgType < Register || msgType > Acknowledge {
		return "Unknown"
	}
	return names[msgType]
}

// GetIdentityMessageType Get the identity message type from the identity message
func GetIdentityMessageType(message []byte) (MessageType, error) {
	if len(message) < 1 {
		return 0, errors.New("failed to get identity message type: no data available")
	}
	return MessageType(message[2]), nil
}

// GetIdentityMessagePayload message payload from the identity message
func GetIdentityMessagePayload(message []byte) ([]byte, error) {
	if len(message) < 2 {
		return []byte{}, errors.New("failed to get identity message payload: no data available")
	}
	return message[IdentityMessageTypeBytes:], nil
}

// ConstructIdentityMessage concatenates msgType as one byte with payload, and return the whole byte array
func ConstructIdentityMessage(identityMessageType MessageType, payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Identity)})
	byteBuffer.WriteByte(byte(Identity))
	byteBuffer.WriteByte(byte(identityMessageType))
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
