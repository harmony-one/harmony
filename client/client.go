package client

import (
	"github.com/harmony-one/harmony/core/types"
	"github.com/simple-rules/harmony-benchmark/blockchain"
	"sync"

	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// Client represents a node (e.g. a wallet) which  sends transactions and receives responses from the harmony network
type Client struct {
	Leaders      *map[uint32]p2p.Peer // Map of shard Id and corresponding leader
	UpdateBlocks func([]*types.Block) // Closure function used to sync new block with the leader. Once the leader finishes the consensus on a new block, it will send it to the clients. Clients use this method to update their blockchain

	ShardUtxoMap      map[uint32]blockchain.UtxoMap
	ShardUtxoMapMutex sync.Mutex // Mutex for the UTXO maps
	log               log.Logger // Log utility

	// The p2p host used to send/receive p2p messages
	host host.Host
}

// NewClient creates a new Client
func NewClient(host host.Host, leaders *map[uint32]p2p.Peer) *Client {
	client := Client{}
	client.Leaders = leaders
	client.host = host
	// Logger
	client.log = log.New()
	return &client
}

// BuildOutputShardTransactionMap builds output shard transaction map.
func BuildOutputShardTransactionMap(txs []*blockchain.Transaction) map[uint32][]*blockchain.Transaction {
	txsShardMap := make(map[uint32][]*blockchain.Transaction)

	// Put txs into corresponding output shards
	for _, crossTx := range txs {
		for curShardID := range GetOutputShardIDsOfCrossShardTx(crossTx) {
			txsShardMap[curShardID] = append(txsShardMap[curShardID], crossTx)
		}
	}
	return txsShardMap
}

// GetInputShardIDsOfCrossShardTx gets input shardID.
func GetInputShardIDsOfCrossShardTx(crossTx *blockchain.Transaction) map[uint32]bool {
	shardIDs := map[uint32]bool{}
	for _, txInput := range crossTx.TxInput {
		shardIDs[txInput.ShardID] = true
	}
	return shardIDs
}

// GetOutputShardIDsOfCrossShardTx gets output shard ids.
func GetOutputShardIDsOfCrossShardTx(crossTx *blockchain.Transaction) map[uint32]bool {
	shardIDs := map[uint32]bool{}
	for _, txOutput := range crossTx.TxOutput {
		shardIDs[txOutput.ShardID] = true
	}
	return shardIDs
}

// GetLeaders returns leader peers.
func (client *Client) GetLeaders() []p2p.Peer {
	leaders := []p2p.Peer{}
	for _, leader := range *client.Leaders {
		leaders = append(leaders, leader)
	}
	return leaders
}
