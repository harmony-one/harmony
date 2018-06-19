package node

import (
	"harmony-benchmark/blockchain"
	"harmony-benchmark/consensus"
	"harmony-benchmark/log"
	"net"
	"os"
	"sync"
)

var pendingTxMutex = &sync.Mutex{}

// A node represents a program (machine) participating in the network
type Node struct {
	consensus              *consensus.Consensus
	BlockChannel           chan blockchain.Block
	pendingTransactions    []*blockchain.Transaction
	transactionInConsensus []*blockchain.Transaction
	blockchain             *blockchain.Blockchain
	utxoPool               *blockchain.UTXOPool
	log                    log.Logger
}

func (node *Node) addPendingTransactions(newTxs []*blockchain.Transaction) {
	pendingTxMutex.Lock()
	node.pendingTransactions = append(node.pendingTransactions, newTxs...)
	pendingTxMutex.Unlock()
}

func (node *Node) getTransactionsForNewBlock() []*blockchain.Transaction {
	pendingTxMutex.Lock()
	selected, unselected := node.utxoPool.SelectTransactionsForNewBlock(node.pendingTransactions)
	node.pendingTransactions = unselected
	pendingTxMutex.Unlock()
	return selected
}

// Start a server and process the request by a handler.
func (node *Node) StartServer(port string) {
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
	return node.consensus.String()
}

// Create a new Node
func NewNode(consensus *consensus.Consensus) Node {
	node := Node{}
	node.consensus = consensus
	node.BlockChannel = make(chan blockchain.Block)

	coinbaseTx := blockchain.NewCoinbaseTX("harmony", "1")
	node.blockchain = &blockchain.Blockchain{}
	node.blockchain.Blocks = make([]*blockchain.Block, 0)
	node.blockchain.Blocks = append(node.blockchain.Blocks, blockchain.NewGenesisBlock(coinbaseTx))
	node.utxoPool = blockchain.CreateUTXOPoolFromGenesisBlockChain(node.blockchain)
	node.log = node.consensus.Log
	node.log.Debug("New node", "node", node)
	return node
}
