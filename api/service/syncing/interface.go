package syncing

import (
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/p2p"
)

// StateSyncInterface is the interface to do state-sync.
type StateSyncInterface interface {
	// Syncing blockchain from other peers.
	// The returned channel is the signal of syncing finish.
	ProcessStateSyncFromPeers(startHash []byte, peers []p2p.Peer, bc *core.BlockChain) (chan struct{}, error)
}
