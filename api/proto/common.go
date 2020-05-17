package proto

import (
	"bytes"
	"errors"
)

/*
The message structure of any message in Harmony network

----  content start -----
1 byte            - message category
                    0x00: Consensus
                    0x01: Node...
1 byte            - message type
                    - for Consensus category
                      0x00: consensus
                      0x01: sharding ...
				    - for Node category
                      0x00: transaction ...
n - 2 bytes       - actual message payload
----   content end  -----
*/

// MessageCategory defines the message category enum
type MessageCategory byte

const (
	// Consensus ..
	Consensus MessageCategory = iota
	// Node ..
	Node
	_ // used to be Client
	_ // used to be DRand
)

const (
	// ProtocolVersion is a constant defined as the version of the Harmony protocol
	ProtocolVersion = 1
	// MessageCategoryBytes is the number of bytes message category takes
	MessageCategoryBytes = 1
	// MessageTypeBytes is the number of bytes message type takes
	MessageTypeBytes = 1
)

// GetMessageCategory gets the message category from the p2p message content
func GetMessageCategory(message []byte) (MessageCategory, error) {
	if len(message) < MessageCategoryBytes {
		return 0, errors.New("failed to get message category: no data available")
	}
	return MessageCategory(message[MessageCategoryBytes-1]), nil
}

// GetMessageType gets the message type from the p2p message content
func GetMessageType(message []byte) (byte, error) {
	if len(message) < MessageCategoryBytes+MessageTypeBytes {
		return 0, errors.New("failed to get message type: no data available")
	}
	return byte(message[MessageCategoryBytes+MessageTypeBytes-1]), nil
}

// GetMessagePayload gets the node message payload from the p2p message content
func GetMessagePayload(message []byte) ([]byte, error) {
	if len(message) < MessageCategoryBytes+MessageTypeBytes {
		return []byte{}, errors.New("failed to get message payload: no data available")
	}
	return message[MessageCategoryBytes+MessageTypeBytes:], nil
}

// GetConsensusMessagePayload gets the consensus message payload from the p2p message content
func GetConsensusMessagePayload(message []byte) ([]byte, error) {
	if len(message) < MessageCategoryBytes {
		return []byte{}, errors.New("failed to get message payload: no data available")
	}
	return message[MessageCategoryBytes:], nil
}

// ConstructConsensusMessage creates a message with the payload and returns as byte array.
func ConstructConsensusMessage(payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(Consensus)})
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
