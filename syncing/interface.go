package syncing

import (
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
)

// StateSyncInterface is the interface to do state-sync.
type StateSyncInterface interface {
	// Syncing blockchain from other peers.
	// The returned channel is the signal of syncing finish.
	ProcessStateSyncFromPeers(peers []p2p.Peer, bc *blockchain.Blockchain) (chan struct{}, error)
}
