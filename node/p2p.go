package node

import (
	"github.com/harmony-one/harmony/p2p"
)

// GetHost returns the p2p host
func (node *Node) GetHost() p2p.Host {
	return node.host
}
