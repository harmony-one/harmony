package consensus

import (
	"bytes"
	"errors"
)

/*
Consensus message is the payload of p2p message.
Consensus message data structure:


---- message start -----
1 byte            - consensus.MessageType
		            0x00 - ANNOUNCE
                    0x01 - COMMIT
                    ...
payload (n bytes) - consensus message payload (the data to run consensus with)
----  message end  -----

*/

const MESSAGE_TYPE_BYTES = 1

// Consensus communication message type.
// Leader and validator dispatch messages based on incoming message type
type MessageType int

const (
	ANNOUNCE MessageType = iota
	COMMIT
	CHALLENGE
	RESPONSE
	START_CONSENSUS
	ERROR = -1
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"ANNOUNCE",
		"COMMIT",
		"CHALLENGE",
		"RESPONSE",
		"START_CONSENSUS"}

	if msgType < ANNOUNCE || msgType > START_CONSENSUS {
		return "Unknown"
	}
	return names[msgType]
}

// Get the consensus message type from the p2p message payload
func GetConsensusMessageType(message []byte) (MessageType, error) {
	if len(message) < 1 {
		return ERROR, errors.New("Failed to get consensus message type: no data available.")
	}
	return MessageType(message[0]), nil
}

// Get the consensus message payload from the p2p message payload
func GetConsensusMessagePayload(message []byte) ([]byte, error) {
	if len(message) < 2 {
		return []byte{}, errors.New("Failed to get consensus message payload: no data available.")
	}
	return message[MESSAGE_TYPE_BYTES:], nil
}

// Concatenate msgType as one byte with payload, and return the whole byte array
func ConstructConsensusMessage(msgType MessageType, payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(msgType)})
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
