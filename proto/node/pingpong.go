/*
Package proto/node implements the communication protocol among nodes.

pingpong.go adds support of ping/pong messages.

ping: from node to peers, sending IP/Port/PubKey info
pong: peer responds to ping messages, sending all pubkeys known by peer

*/

package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
)

// refer to Peer struct in p2p/peer.go
// this is basically a simplified version of Peer
// for network transportation
type nodeInfo struct {
	IP          string
	Port        string
	PubKey      []byte
	ValidatorID int
}

type PingMessageType struct {
	Version uint16 // version of the protocol
	Node    nodeInfo
}

type PongMessageType struct {
	Version uint16 // version of the protocol
	Peers   []nodeInfo
}

func (p PingMessageType) String() string {
	return fmt.Sprintf("ping:%v=>%v:%v:%v/%v", p.Version, p.Node.IP, p.Node.Port, p.Node.ValidatorID, p.Node.PubKey)
}

func (p PongMessageType) String() string {
	str := fmt.Sprintf("pong:%v=>length:%v\n", p.Version, len(p.Peers))
	for _, p := range p.Peers {
		str = fmt.Sprintf("%v%v:%v:%v/%v\n", str, p.IP, p.Port, p.ValidatorID, p.PubKey)
	}
	return str
}

func NewPingMessage(peer p2p.Peer) *PingMessageType {
	ping := new(PingMessageType)

	var err error
	ping.Version = PROTOCOL_VERSION
	ping.Node.IP = peer.Ip
	ping.Node.Port = peer.Port
	ping.Node.ValidatorID = peer.ValidatorID
	ping.Node.PubKey, err = peer.PubKey.MarshalBinary()

	if err != nil {
		fmt.Printf("Error Marshall PubKey: %v", err)
		return nil
	}

	return ping
}

func NewPongMessage(peers []p2p.Peer) *PongMessageType {
	pong := new(PongMessageType)

	pong.Version = PROTOCOL_VERSION
	pong.Peers = make([]nodeInfo, 0)

	var err error
	for _, p := range peers {
		n := nodeInfo{}
		n.IP = p.Ip
		n.Port = p.Port
		n.ValidatorID = p.ValidatorID
		n.PubKey, err = p.PubKey.MarshalBinary()
		if err != nil {
			fmt.Printf("Error Marshall PubKey: %v", err)
			continue
		}
		pong.Peers = append(pong.Peers, n)
	}

	return pong
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
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(PING))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(ping)
	if err != nil {
		log.Panic("Can't serialize Ping message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}

// ConstructPongMessage contructs pong message from leader to node
func (pong PongMessageType) ConstructPongMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(PONG))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(pong)
	if err != nil {
		log.Panic("Can't serialize Pong message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}
