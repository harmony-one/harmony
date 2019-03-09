/*
Package proto/discovery implements the discovery ping/pong protocol among nodes.

pingpong.go adds support of ping/pong messages.

ping: from node to peers, sending IP/Port/PubKey info
pong: peer responds to ping messages, sending all pubkeys known by peer

*/

package discovery

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/p2p"
)

// PingMessageType defines the data structure of the Ping message
type PingMessageType struct {
	Version uint16 // version of the protocol
	Node    node.Info
}

// PongMessageType defines the data structure of the Pong message
type PongMessageType struct {
	Version      uint16 // version of the protocol
	Peers        []node.Info
	PubKeys      [][]byte // list of publickKeys, has to be identical among all validators/leaders
	LeaderPubKey []byte   // public key of shard leader
}

func (p PingMessageType) String() string {
	return fmt.Sprintf("ping:%v/%v=>%v:%v:%v/%v", p.Node.Role, p.Version, p.Node.IP, p.Node.Port, p.Node.ValidatorID, p.Node.PubKey)
}

func (p PongMessageType) String() string {
	str := fmt.Sprintf("pong:%v=>length:%v, keys:%v, leader:%v\n", p.Version, len(p.Peers), len(p.PubKeys), len(p.LeaderPubKey))
	return str
}

// NewPingMessage creates a new Ping message based on the p2p.Peer input
func NewPingMessage(peer p2p.Peer, isClient bool) *PingMessageType {
	ping := new(PingMessageType)

	ping.Version = proto.ProtocolVersion
	ping.Node.IP = peer.IP
	ping.Node.Port = peer.Port
	ping.Node.PeerID = peer.PeerID
	if !isClient {
		ping.Node.ValidatorID = peer.ValidatorID
		ping.Node.PubKey = peer.ConsensusPubKey.Serialize()
		ping.Node.Role = node.ValidatorRole
	} else {
		ping.Node.ValidatorID = -1
		ping.Node.PubKey = nil
		ping.Node.Role = node.ClientRole
	}

	return ping
}

// NewPongMessage creates a new Pong message based on a list of p2p.Peer and a list of publicKeys
func NewPongMessage(peers []p2p.Peer, pubKeys []*bls.PublicKey, leaderKey *bls.PublicKey) *PongMessageType {
	pong := new(PongMessageType)
	pong.PubKeys = make([][]byte, 0)

	pong.Version = proto.ProtocolVersion
	pong.Peers = make([]node.Info, 0)

	var err error
	for _, p := range peers {
		n := node.Info{}
		n.IP = p.IP
		n.Port = p.Port
		n.ValidatorID = p.ValidatorID
		n.PeerID = p.PeerID
		n.PubKey = p.ConsensusPubKey.Serialize()
		if err != nil {
			fmt.Printf("Error Marshal PubKey: %v", err)
			continue
		}
		pong.Peers = append(pong.Peers, n)
	}

	for _, p := range pubKeys {
		key := p.Serialize()

		pong.PubKeys = append(pong.PubKeys, key)
	}

	pong.LeaderPubKey = leaderKey.Serialize()
	//	utils.GetLogInstance().Info("[pong message]", "keys", len(pong.PubKeys), "peers", len(pong.Peers), "leader", len(pong.LeaderPubKey))

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
	pong.Peers = make([]node.Info, 0)
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
	byteBuffer.WriteByte(byte(node.PING))

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
	byteBuffer.WriteByte(byte(node.PONG))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(p)
	if err != nil {
		log.Panic("Can't serialize Pong message", "error:", err)
		return nil
	}
	return byteBuffer.Bytes()
}
