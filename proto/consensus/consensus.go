package consensus

import (
	"bytes"
	"errors"

	"github.com/simple-rules/harmony-benchmark/proto"
)

/*
Consensus message is the payload of p2p message.
Consensus message data structure:


Announce:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - Announce
                    0x01 - Commit
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - leader id
(n bytes)         - consensus payload (the data to run consensus with, e.g. block header data)
4 byte            - payload size
64 byte           - signature
----  message end  -----

Commit:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - Announce
                    0x01 - Commit
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - validator id
32 byte           - commit message (Note it's different than Zilliqa's ECPoint which takes 33 bytes: https://crypto.stackexchange.com/questions/51703/how-to-convert-from-curve25519-33-byte-to-32-byte-representation)
64 byte           - signature
----  message end  -----

CHALLENGE:
---- message start -----
1 byte            - consensus.MessageType
                    0x00 - Announce
                    0x01 - Commit
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
                    0x00 - Announce
                    0x01 - Commit
                    ...
4 byte            - consensus id
32 byte           - block hash
2 byte            - validator id
32 byte           - response
64 byte           - signature
----  message end  -----
*/

// the number of bytes consensus message type occupies
const ConsensusMessageTypeBytes = 1

// The specific types of message under Consensus category
type ConsensusMessageType byte

const (
	Consensus ConsensusMessageType = iota
	// TODO: add more types
)

// Consensus communication message type.
// Leader and validator dispatch messages based on incoming message type
type MessageType int

const (
	Announce MessageType = iota
	Commit
	CHALLENGE
	RESPONSE
	COLLECTIVE_SIG
	FinalCommit
	FINAL_CHALLENGE
	FinalResponse
	StartConsensus
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"Announce",
		"Commit",
		"CHALLENGE",
		"RESPONSE",
		"COLLECTIVE_SIG",
		"FinalCommit",
		"FINAL_CHALLENGE",
		"FinalResponse",
		"StartConsensus",
	}

	if msgType < Announce || msgType > StartConsensus {
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
	return message[ConsensusMessageTypeBytes:], nil
}

// Concatenate msgType as one byte with payload, and return the whole byte array
func ConstructConsensusMessage(consensusMsgType MessageType, payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Consensus)})
	byteBuffer.WriteByte(byte(Consensus))
	byteBuffer.WriteByte(byte(consensusMsgType))
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
