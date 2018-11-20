/*
Package proto/node implements the communication protocol among nodes.

pingpong.go adds support of ping/pong messages.

ping: from node to peers, sending IP/Port/PubKey info
TODO: add protocol version support

pong: peer responds to ping messages, sending all pubkeys known by peer
TODO:
* add the version of the protocol

*/

package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
)

type PingMessageType struct {
	IP     string
	Port   string
	PubKey string
}

type PongMessageType struct {
	PubKeys []string
}

func (p PingMessageType) String() string {
	return fmt.Sprintf("%v:%v/%v", p.IP, p.Port, p.PubKey)
}

func (p PongMessageType) String() string {
	return fmt.Sprintf("# Keys: %v", len(p.PubKeys))
}

// Deserialize Ping Message
func GetPingMessage(payload []byte) (*PingMessageType, error) {
	ping := new(PingMessageType)

	r := bytes.NewBuffer(payload)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(ping)

	if err != nil {
		log.Panic("Can't deserialize Ping message")
	}

	return ping, nil
}

// Deserialize Pong Message
func GetPongMessage(payload []byte) (*PongMessageType, error) {
	pong := new(PongMessageType)

	r := bytes.NewBuffer(payload)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(pong)

	if err != nil {
		log.Panic("Can't deserialize Pong message")
	}

	return pong, nil
}

// ConstructPingMessage contructs ping message from node to leader
func (ping PingMessageType) ConstructPingMessage() []byte {
	var byteBuffer bytes.Buffer
	encoder := gob.NewEncoder(&byteBuffer)
	err := encoder.Encode(ping)
	if err != nil {
		log.Panic("Can't serialize Ping message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}

// ConstructPongMessage contructs pong message from leader to node
func (pong PongMessageType) ConstructPongMessage() []byte {
	var byteBuffer bytes.Buffer
	encoder := gob.NewEncoder(&byteBuffer)
	err := encoder.Encode(pong)
	if err != nil {
		log.Panic("Can't serialize Pong message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}
