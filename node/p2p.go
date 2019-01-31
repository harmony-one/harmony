package node

import (
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// SendMessage sends data to ip, port
func (node *Node) SendMessage(p p2p.Peer, data []byte) {
	host.SendMessage(node.host, p, data, nil)
}

// BroadcastMessage broadcasts message to peers
func (node *Node) BroadcastMessage(peers []p2p.Peer, data []byte) {
	host.BroadcastMessage(node.host, peers, data, nil)
}

// GetHost returns the p2p host
func (node *Node) GetHost() p2p.Host {
	return node.host
}
