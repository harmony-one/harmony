package proto

import (
	"bytes"
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

// Consensus and other message categories
const (
	Consensus MessageCategory = iota
	Node
	Client // deprecated
	DRand  // not used
)

const (
	// ProtocolVersion is a constant defined as the version of the Harmony protocol
	ProtocolVersion = 1
	// MessageCategoryBytes is the number of bytes message category takes
	MessageCategoryBytes = 1
)

// ConstructConsensusMessage creates a message with the payload and returns as byte array.
func ConstructConsensusMessage(payload []byte) []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(Consensus)})
	byteBuffer.Write(payload)
	return byteBuffer.Bytes()
}
