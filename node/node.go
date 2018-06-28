package node

import (
	"harmony-benchmark/blockchain"
	"harmony-benchmark/client"
	"harmony-benchmark/consensus"
	"harmony-benchmark/log"
	"harmony-benchmark/p2p"
	"net"
	"os"
	"strconv"
	"sync"
)

// Node represents a program (machine) participating in the network
// TODO(minhdoan, rj): consider using BlockChannel *chan blockchain.Block for efficiency.
type Node struct {
	Consensus              *consensus.Consensus               // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan blockchain.Block              // The channel to receive new blocks from Node
	pendingTransactions    []*blockchain.Transaction          // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*blockchain.Transaction          // The transactions selected into the new block and under Consensus process
	blockchain             *blockchain.Blockchain             // The blockchain for the shard where this node belongs
	UtxoPool               *blockchain.UTXOPool               // The corresponding UTXO pool of the current blockchain
	CrossTxsInConsensus    []*blockchain.CrossShardTxAndProof // The cross shard txs that is under consensus, the proof is not filled yet.
	CrossTxsToReturn       []*blockchain.CrossShardTxAndProof // The cross shard txs and proof that needs to be sent back to the user client.
	log                    log.Logger                         // Log utility

	pendingTxMutex       sync.Mutex
	crossTxToReturnMutex sync.Mutex

	ClientPeer *p2p.Peer // The peer for the benchmark tx generator client, used for leaders to return proof-of-accept

	Client *client.Client // The presence of a client object means this node will also act as a client
}

// Add new crossTx and proofs to the list of crossTx that needs to be sent back to client
func (node *Node) addCrossTxsToReturn(crossTxs []*blockchain.CrossShardTxAndProof) {
	node.crossTxToReturnMutex.Lock()
	node.CrossTxsToReturn = append(node.CrossTxsToReturn, crossTxs...)
	node.crossTxToReturnMutex.Unlock()
	node.log.Debug("Got more cross transactions to return", "num", len(crossTxs), len(node.pendingTransactions), "node", node)
}

// Add new transactions to the pending transaction list
func (node *Node) addPendingTransactions(newTxs []*blockchain.Transaction) {
	node.pendingTxMutex.Lock()
	node.pendingTransactions = append(node.pendingTransactions, newTxs...)
	node.pendingTxMutex.Unlock()
	node.log.Debug("Got more transactions", "num", len(newTxs), "totalPending", len(node.pendingTransactions), "node", node)
}

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlock(maxNumTxs int) ([]*blockchain.Transaction, []*blockchain.CrossShardTxAndProof) {
	node.pendingTxMutex.Lock()
	selected, unselected, crossShardTxs := node.UtxoPool.SelectTransactionsForNewBlock(node.pendingTransactions, maxNumTxs)
	node.pendingTransactions = unselected
	node.pendingTxMutex.Unlock()
	return selected, crossShardTxs
}

// Start a server and process the request by a handler.
func (node *Node) StartServer(port string) {
	node.log.Debug("Starting server", "node", node, "port", port)
	node.listenOnPort(port)
}

func (node *Node) listenOnPort(port string) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer listen.Close()
	if err != nil {
		node.log.Crit("Socket listen port failed", "port", port, "err", err)
		os.Exit(1)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			node.log.Crit("Error listening on port. Exiting.", "port", port)
			continue
		}
		go node.NodeHandler(conn)
	}
}

func (node *Node) String() string {
	return node.Consensus.String()
}

// [Testing code] Should be deleted for production
// Create in genesis block numTxs transactions which assign 1000 token to each address in [1 - numTxs]
func (node *Node) AddMoreFakeTransactions(numTxs int) {
	txs := make([]*blockchain.Transaction, numTxs)
	for i := range txs {
		txs[i] = blockchain.NewCoinbaseTX(strconv.Itoa(i), "", node.Consensus.ShardID)
	}
	node.blockchain.Blocks[0].Transactions = append(node.blockchain.Blocks[0].Transactions, txs...)
	node.UtxoPool.Update(txs)
}

// Count the total number of transactions in the blockchain
// Currently used for stats reporting purpose
func (node *Node) countNumTransactionsInBlockchain() int {
	count := 0
	for _, block := range node.blockchain.Blocks {
		count += len(block.Transactions)
	}
	return count
}

// Create a new Node
func NewNode(consensus *consensus.Consensus) Node {
	node := Node{}

	// Consensus and associated channel to communicate blocks
	node.Consensus = consensus
	node.BlockChannel = make(chan blockchain.Block)

	// Genesis Block
	// TODO(minh): Use or implement new function in blockchain package for this.
	genesisBlock := &blockchain.Blockchain{}
	genesisBlock.Blocks = make([]*blockchain.Block, 0)
	coinbaseTx := blockchain.NewCoinbaseTX("harmony", "1", node.Consensus.ShardID)
	genesisBlock.Blocks = append(genesisBlock.Blocks, blockchain.NewGenesisBlock(coinbaseTx, node.Consensus.ShardID))
	node.blockchain = genesisBlock

	// UTXO pool from Genesis block
	node.UtxoPool = blockchain.CreateUTXOPoolFromGenesisBlockChain(node.blockchain)

	// Logger
	node.log = node.Consensus.Log
	return node
}
