package node

import (
	"harmony-benchmark/blockchain"
	"harmony-benchmark/consensus"
	"harmony-benchmark/log"
	"net"
	"os"
	"strconv"
	"sync"
)

var pendingTxMutex = &sync.Mutex{}

// Node represents a program (machine) participating in the network
// TODO(minhdoan, rj): consider using BlockChannel *chan blockchain.Block for efficiency.
type Node struct {
	Consensus              *consensus.Consensus      // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan blockchain.Block     // The channel to receive new blocks from Node
	pendingTransactions    []*blockchain.Transaction // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*blockchain.Transaction // The transactions selected into the new block and under Consensus process
	blockchain             *blockchain.Blockchain    // The blockchain for the shard where this node belongs
	UtxoPool               *blockchain.UTXOPool      // The corresponding UTXO pool of the current blockchain
	log                    log.Logger                // Log utility
}

// Add new transactions to the pending transaction list
func (node *Node) addPendingTransactions(newTxs []*blockchain.Transaction) {

	pendingTxMutex.Lock()
	node.pendingTransactions = append(node.pendingTransactions, newTxs...)
	pendingTxMutex.Unlock()
	node.log.Debug("Got more transactions", "num", len(newTxs))
	node.log.Debug("Total pending transactions", "num", len(node.pendingTransactions))
}

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlock(maxNumTxs int) []*blockchain.Transaction {
	pendingTxMutex.Lock()
	selected, unselected := node.UtxoPool.SelectTransactionsForNewBlock(node.pendingTransactions, maxNumTxs)
	node.pendingTransactions = unselected
	pendingTxMutex.Unlock()
	return selected
}

// Start a server and process the request by a handler.
func (node *Node) StartServer(port string) {
	node.log.Debug("Starting server", "node", node)
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
// Create in genesis block 1000 transactions which assign 1000 token to each address in [1 - 1000]
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
