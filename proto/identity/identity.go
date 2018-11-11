package identity

import (
	"bytes"
	"errors"

	"github.com/simple-rules/harmony-benchmark/proto"
)

// IdentityMessageTypeBytes is the number of bytes consensus message type occupies
const IdentityMessageTypeBytes = 1

// IdentityMessageType is the identity message type.
type IdentityMessageType byte

// Constants of IdentityMessageType.
const (
	Identity IdentityMessageType = iota
	// TODO: add more types
)

// MessageType ...
type MessageType int

// Constants of MessageType.
const (
	Register MessageType = iota
	Acknowledge
	Leader
	IDCKey
	Node_Info
	Peers
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"Register",
		"Acknowledge",
		"Leader",
		"IDCKey",
		"Node_Info",
		"Peers",
	}

	if msgType < Register || msgType > Peers {
		return "Unknown"
	}
	return names[msgType]
}

// GetIdentityMessageType Get the identity message type from the identity message
func GetIdentityMessageType(message []byte) (MessageType, error) {
	if len(message) < 1 {
		return 0, errors.New("Failed to get identity message type: no data available.")
	}
	return MessageType(message[0]), nil
}

// GetIdentityMessagePayload message payload from the identity message
func GetIdentityMessagePayload(message []byte) ([]byte, error) {
	if len(message) < 2 {
		return []byte{}, errors.New("Failed to get identity message payload: no data available.")
	}
	return message[IdentityMessageTypeBytes:], nil
}

// Concatenate msgType as one byte with payload, and return the whole byte array
func ConstructIdentityMessage(identityMessageType MessageType, payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Identity)})
	byteBuffer.WriteByte(byte(Identity))
	byteBuffer.WriteByte(byte(identityMessageType))
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
