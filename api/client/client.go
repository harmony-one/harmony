package client

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p"
)

// Client represents a node (e.g. a wallet) which  sends transactions and receives responses from the harmony network
type Client struct {
	ShardID      uint32               // ShardID
	UpdateBlocks func([]*types.Block) // Closure function used to sync new block with the leader. Once the leader finishes the consensus on a new block, it will send it to the clients. Clients use this method to update their blockchain

	log log.Logger // Log utility

	// The p2p host used to send/receive p2p messages
	host p2p.Host
}

// NewClient creates a new Client
func NewClient(host p2p.Host, shardID uint32) *Client {
	client := Client{}
	client.host = host
	client.ShardID = shardID
	// Logger
	client.log = log.New()
	return &client
}
