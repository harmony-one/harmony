package consensus

import (
	"bytes"
	"errors"
)

/*
Consensus message is the payload of p2p message.
Consensus message data structure:


ANNOUNCE:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - ANNOUNCE
                    0x01 - COMMIT
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - leader id
(n bytes)         - consensus payload (the data to run consensus with, e.g. block header data)
4 byte            - payload size
64 byte           - signature
----  message end  -----

COMMIT:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - ANNOUNCE
                    0x01 - COMMIT
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - validator id
33 byte           - commit message
64 byte           - signature
----  message end  -----

CHALLENGE:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - ANNOUNCE
                    0x01 - COMMIT
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - leader id
33 byte           - aggregated commit
33 byte           - aggregated key
32 byte           - challenge
64 byte           - signature
----  message end  -----

RESPONSE:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - ANNOUNCE
                    0x01 - COMMIT
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - validator id
32 byte           - response
64 byte           - signature
----  message end  -----
*/

const MESSAGE_TYPE_BYTES = 1

// The specific types of message under COMMITTEE category
type CommitteeMessageType byte

const (
	CONSENSUS CommitteeMessageType = iota
	// TODO: add more types
)

// Consensus communication message type.
// Leader and validator dispatch messages based on incoming message type
type MessageType int

const (
	ANNOUNCE MessageType = iota
	COMMIT
	CHALLENGE
	RESPONSE
	START_CONSENSUS
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"ANNOUNCE",
		"COMMIT",
		"CHALLENGE",
		"RESPONSE",
		"START_CONSENSUS",
	}

	if msgType < ANNOUNCE || msgType > START_CONSENSUS {
		return "Unknown"
	}
	return names[msgType]
}

// Get the consensus message type from the consensus message
func GetConsensusMessageType(message []byte) (MessageType, error) {
	if len(message) < 1 {
		return 0, errors.New("Failed to get consensus message type: no data available.")
	}
	return MessageType(message[0]), nil
}

// Get the consensus message payload from the consensus message
func GetConsensusMessagePayload(message []byte) ([]byte, error) {
	if len(message) < 2 {
		return []byte{}, errors.New("Failed to get consensus message payload: no data available.")
	}
	return message[MESSAGE_TYPE_BYTES:], nil
}

// Concatenate msgType as one byte with payload, and return the whole byte array
func (consensus Consensus) ConstructConsensusMessage(consensusMsgType MessageType, payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{consensus.msgCategory})
	byteBuffer.WriteByte(consensus.actionType)
	byteBuffer.WriteByte(byte(consensusMsgType))
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
