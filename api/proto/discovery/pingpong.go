/*
Package proto/discovery implements the discovery ping/pong protocol among nodes.

pingpong.go adds support of ping/pong messages.

ping: from node to peers, sending IP/Port/PubKey info
pong: peer responds to ping messages, sending all pubkeys known by peer

*/

package discovery

import (
	"fmt"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// NewPingMessage creates a new Ping message based on the p2p.Peer input
func NewPingMessage(peer p2p.Peer, isClient bool) *node.PingMessageType {
	ping := new(node.PingMessageType)

	ping.Version = proto.ProtocolVersion
	ping.Node.IP = peer.IP
	ping.Node.Port = peer.Port
	ping.Node.PeerID = peer.PeerID.String()
	if !isClient {
		ping.Node.PubKey = peer.ConsensusPubKey.Serialize()
		ping.Node.Role = uint32(node.ValidatorRole)
	} else {
		ping.Node.PubKey = nil
		ping.Node.Role = uint32(node.ClientRole)
	}

	return ping
}

// NewPongMessage creates a new Pong message based on a list of p2p.Peer and a list of publicKeys
func NewPongMessage(peers []p2p.Peer, pubKeys []*bls.PublicKey, leaderKey *bls.PublicKey, shardID uint32) *node.PongMessageType {
	pong := new(node.PongMessageType)
	pong.ShardID = shardID
	pong.PubKeys = make([][]byte, 0)

	pong.Version = proto.ProtocolVersion
	pong.Peers = make([]*node.Info, 0)

	var err error
	for _, p := range peers {
		n := &node.Info{}
		n.IP = p.IP
		n.Port = p.Port
		n.PeerID = p.PeerID.String()
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

	return pong
}

// GetPingMessage deserializes the Ping Message from a list of byte
func GetPingMessage(payload []byte) (*node.PingMessageType, error) {
	ping := new(node.PingMessageType)

	err := ping.XXX_Unmarshal(payload)

	if err != nil {
		utils.GetLogInstance().Error("[GetPingMessage] Decode", "error", err)
		return nil, fmt.Errorf("Decode Ping Error")
	}

	return ping, nil
}

// GetPongMessage deserializes the Pong Message from a list of byte
func GetPongMessage(payload []byte) (*node.PongMessageType, error) {
	pong := new(node.PongMessageType)
	pong.Peers = make([]*node.Info, 0)
	pong.PubKeys = make([][]byte, 0)

	err := pong.XXX_Unmarshal(payload)

	if err != nil {
		utils.GetLogInstance().Error("[GetPongMessage] Decode", "error", err)
		return nil, fmt.Errorf("Decode Pong Error")
	}

	return pong, nil
}
