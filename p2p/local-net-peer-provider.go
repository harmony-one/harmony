package p2p

import (
	"fmt"
)

type LocalProvider struct {
	basePort  uint16
	selfPort  uint16
	numShards uint32
	shardSize uint32
	onShardID uint32
}

// Peers ..
func (p *LocalProvider) Peers() []Peer {
	firstPort := uint32(p.basePort) + p.onShardID
	endPort := uint32(p.basePort) + p.numShards*p.shardSize
	peers := []Peer{}

	for port := firstPort; port < endPort; port += p.numShards {
		if port == uint32(p.selfPort) {
			continue // do not sync from self
		}
		peers = append(peers, Peer{IP: "127.0.0.1", Port: fmt.Sprint(port)})
	}

	return peers

}

// NewLocal returns a provider that synthesizes syncing
// peers given the network configuration
func NewLocal(
	basePort, selfPort uint16, numShards, shardSize uint32,
) *LocalProvider {
	return &LocalProvider{
		basePort:  basePort,
		selfPort:  selfPort,
		numShards: numShards,
		shardSize: shardSize,
	}
}
