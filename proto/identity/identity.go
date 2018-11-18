package identity

import (
	"bytes"
	"errors"

	"github.com/harmony-one/harmony/proto"
)

// TODO: the message structure should be defined in a structure
//       and use reflect to serialize/deseralize it, instead of
//       manual serialization/deserialization functions

/*
Identity message is the payload of message between nodes and IDC
Identity message data structure:

Register:
---- message start -----
1 byte            - identity.MessageType
                    0x00 - Register
                    0x01 - Acknowledge
                    0x02 - Peers
                    ...
4 byte            - protocol version
32 byte           - pubKey
4 byte            - myIP
4 byte            - myPort
(n bytes)         - transaction data for PoS
----  message end  -----

Acknowledge:
---- message start -----
1 byte            - identity.MessageType
                    0x00 - Register
                    0x01 - Acknowledge
                    0x02 - Peers
                    ...
4 byte            - protocol version
----  message end  -----

Peer:
---- message start -----
1 byte            - identity.MessageType
                    0x00 - Register
                    0x01 - Acknowledge
                    0x02 - Peers
                    ...
4 byte            - number of shards
32 byte           - randomness
(n bytes)         - leader info list
* 32 byte         - leader pubKey
* 4 byte          - leader IP
64 byte           - signature
----  message end  -----

*/

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
	NodeInfo
	Peers
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"Register",
		"Acknowledge",
		"Leader",
		"IDCKey",
		"NodeInfo",
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
