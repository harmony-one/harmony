package common

import (
	"errors"
)

/*

Node will process the content of the p2p message

----  content start -----
1 byte            - message category
                    0x00: COMMITTEE
                    0x01: NODE...
1 byte            - message type
                    - for COMMITTEE category
                      0x00: consensus
                      0x01: sharding ...
				    - for NODE category
                      0x00: transaction ...
n - 2 bytes       - actual message payload
----   content end  -----

*/

const NODE_TYPE_BYTES = 1
const ACTION_TYPE_BYTES = 1

// The CATEGORY of messages
type MessageCategory byte

const (
	COMMITTEE MessageCategory = iota
	NODE
	// TODO: add more types
)


// The specific types of message under COMMITTEE category
type CommitteeMessageType byte

const (
	CONSENSUS CommitteeMessageType = iota
	// TODO: add more types
)

// The specific types of message under NODE category
type NodeMessageType byte

const (
	TRANSACTION NodeMessageType = iota
	CONTROL
	// TODO: add more types
)


// Get the message category from the p2p message content
func GetMessageCategory(message []byte) (MessageCategory, error) {
	if len(message) < NODE_TYPE_BYTES {
		return 0, errors.New("Failed to get node type: no data available.")
	}
	return MessageCategory(message[NODE_TYPE_BYTES-1]), nil
}

// Get the action type from the p2p message content
func GetMessageType(message []byte) (byte, error) {
	if len(message) < NODE_TYPE_BYTES+ACTION_TYPE_BYTES {
		return 0, errors.New("Failed to get action type: no data available.")
	}
	return byte(message[NODE_TYPE_BYTES+ACTION_TYPE_BYTES-1]), nil
}

// Get the node message payload from the p2p message content
func GetMessagePayload(message []byte) ([]byte, error) {
	if len(message) < NODE_TYPE_BYTES+ACTION_TYPE_BYTES {
		return []byte{}, errors.New("Failed to get message payload: no data available.")
	}
	return message[NODE_TYPE_BYTES+ACTION_TYPE_BYTES:], nil
}
