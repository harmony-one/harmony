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

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/p2p"
)

// RoleType defines the role of the node
type RoleType int

// Type of roles of a node
const (
	ValidatorRole RoleType = iota
	ClientRole
)

func (r RoleType) String() string {
	switch r {
	case ValidatorRole:
		return "Validator"
	case ClientRole:
		return "Client"
	}
	return "Unknown"
}

// Info refers to Peer struct in p2p/peer.go
// this is basically a simplified version of Peer
// for network transportation
type Info struct {
	IP          string
	Port        string
	PubKey      []byte
	ValidatorID int
	Role        RoleType
}

// PingMessageType defines the data structure of the Ping message
type PingMessageType struct {
	Version uint16 // version of the protocol
	Node    Info
}

// PongMessageType defines the data structure of the Pong message
type PongMessageType struct {
	Version uint16 // version of the protocol
	Peers   []Info
	PubKeys [][]byte // list of publickKeys, has to be identical among all validators/leaders
}

func (p PingMessageType) String() string {
	return fmt.Sprintf("ping:%v/%v=>%v:%v:%v/%v", p.Node.Role, p.Version, p.Node.IP, p.Node.Port, p.Node.ValidatorID, p.Node.PubKey)
}

func (p PongMessageType) String() string {
	str := fmt.Sprintf("pong:%v=>length:%v, keys:%v\n", p.Version, len(p.Peers), len(p.PubKeys))
	return str
}

// NewPingMessage creates a new Ping message based on the p2p.Peer input
func NewPingMessage(peer p2p.Peer) *PingMessageType {
	ping := new(PingMessageType)

	var err error
	ping.Version = ProtocolVersion
	ping.Node.IP = peer.IP
	ping.Node.Port = peer.Port
	ping.Node.ValidatorID = peer.ValidatorID
	ping.Node.PubKey, err = peer.PubKey.MarshalBinary()
	ping.Node.Role = ValidatorRole

	if err != nil {
		fmt.Printf("Error Marshal PubKey: %v", err)
		return nil
	}

	return ping
}

// NewPongMessage creates a new Pong message based on a list of p2p.Peer and a list of publicKeys
func NewPongMessage(peers []p2p.Peer, pubKeys []kyber.Point) *PongMessageType {
	pong := new(PongMessageType)
	pong.PubKeys = make([][]byte, 0)

	pong.Version = ProtocolVersion
	pong.Peers = make([]Info, 0)

	var err error
	for _, p := range peers {
		n := Info{}
		n.IP = p.IP
		n.Port = p.Port
		n.ValidatorID = p.ValidatorID
		n.PubKey, err = p.PubKey.MarshalBinary()
		if err != nil {
			fmt.Printf("Error Marshal PubKey: %v", err)
			continue
		}
		pong.Peers = append(pong.Peers, n)
	}

	for _, p := range pubKeys {
		key, err := p.MarshalBinary()
		if err != nil {
			fmt.Printf("Error Marshal PublicKeys: %v", err)
			continue
		}

		pong.PubKeys = append(pong.PubKeys, key)
	}

	return pong
}

// GetPingMessage deserializes the Ping Message from a list of byte
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

// GetPongMessage deserializes the Pong Message from a list of byte
func GetPongMessage(payload []byte) (*PongMessageType, error) {
	pong := new(PongMessageType)
	pong.Peers = make([]Info, 0)
	pong.PubKeys = make([][]byte, 0)

	r := bytes.NewBuffer(payload)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(pong)

	if err != nil {
		log.Panic("Can't deserialize Pong message")
	}

	return pong, nil
}

// ConstructPingMessage contructs ping message from node to leader
func (p PingMessageType) ConstructPingMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(PING))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(p)
	if err != nil {
		log.Panic("Can't serialize Ping message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}

// ConstructPongMessage contructs pong message from leader to node
func (p PongMessageType) ConstructPongMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(PONG))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(p)
	if err != nil {
		log.Panic("Can't serialize Pong message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}
