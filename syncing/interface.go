package syncing

import (
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
)

// StateSync is the interface to do state-sync.
type StateSyncInterface interface {
	// Syncing Block in blockchain from other peers.
	// The return channel is the signal of syncing finish.
	ProcessStateSync(peers []p2p.Peer, bc *blockchain.Blockchain) (chan struct{}, error)
}
