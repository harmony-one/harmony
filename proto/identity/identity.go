package identity

import (
	"errors"
)

// the number of bytes consensus message type occupies
const IDENTITY_MESSAGE_TYPE_BYTES = 1

type MessageType int

const (
	REGISTER MessageType = iota
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"REGISTER",
	}

	if msgType < REGISTER || msgType > REGISTER {
		return "Unknown"
	}
	return names[msgType]
}

// GetIdentityMessageType Get the consensus message type from the identity message
func GetIdentityMessageType(message []byte) (MessageType, error) {
	if len(message) < 1 {
		return 0, errors.New("Failed to get consensus message type: no data available.")
	}
	return MessageType(message[0]), nil
}

// GetIdentityMessagePayload message payload from the identity message
func GetIdentityMessagePayload(message []byte) ([]byte, error) {
	if len(message) < 2 {
		return []byte{}, errors.New("Failed to get consensus message payload: no data available.")
	}
	return message[IDENTITY_MESSAGE_TYPE_BYTES:], nil
}
