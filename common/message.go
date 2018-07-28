package common

import (
	"errors"
)

// TODO: Fix the comments below.
/*

The message structure of any message in Harmony network

----  content start -----
1 byte            - message category
                    0x00: CONSENSUS
                    0x01: NODE...
1 byte            - message type
                    - for CONSENSUS category
                      0x00: consensus
                      0x01: sharding ...
				    - for NODE category
                      0x00: transaction ...
n - 2 bytes       - actual message payload
----   content end  -----

*/

// NODE_TYPE_BYTES is # of bytes  message category
const NODE_TYPE_BYTES = 1

// ACTION_TYPE_BYTES is # of bytes for message type which can be
// - for CONSENSUS category
// 0x00: consensus
// 0x01: sharding ...
// - for NODE category
// 0x00: transaction ...
const ACTION_TYPE_BYTES = 1

// The CATEGORY of messages
type MessageCategory byte

const (
	CONSENSUS MessageCategory = iota
	NODE
	CLIENT
	// TODO: add more types
)

// Get the message category from the p2p message content
func GetMessageCategory(message []byte) (MessageCategory, error) {
	if len(message) < NODE_TYPE_BYTES {
		return 0, errors.New("Failed to get node type: no data available.")
	}
	return MessageCategory(message[NODE_TYPE_BYTES-1]), nil
}

// Get the message type from the p2p message content
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
