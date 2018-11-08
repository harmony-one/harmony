package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/client"
	"github.com/simple-rules/harmony-benchmark/consensus"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/db"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto/identity"
	"github.com/simple-rules/harmony-benchmark/syncing"
)

// Node represents a program (machine) participating in the network
// TODO(minhdoan, rj): consider using BlockChannel *chan blockchain.Block for efficiency.
type Node struct {
	Consensus              *consensus.Consensus               // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan blockchain.Block              // The channel to receive new blocks from Node
	pendingTransactions    []*blockchain.Transaction          // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*blockchain.Transaction          // The transactions selected into the new block and under Consensus process
	blockchain             *blockchain.Blockchain             // The blockchain for the shard where this node belongs
	db                     *db.LDBDatabase                    // LevelDB to store blockchain.
	UtxoPool               *blockchain.UTXOPool               // The corresponding UTXO pool of the current blockchain
	CrossTxsInConsensus    []*blockchain.CrossShardTxAndProof // The cross shard txs that is under consensus, the proof is not filled yet.
	CrossTxsToReturn       []*blockchain.CrossShardTxAndProof // The cross shard txs and proof that needs to be sent back to the user client.
	log                    log.Logger                         // Log utility
	pendingTxMutex         sync.Mutex
	crossTxToReturnMutex   sync.Mutex
	ClientPeer             *p2p.Peer      // The peer for the benchmark tx generator client, used for leaders to return proof-of-accept
	Client                 *client.Client // The presence of a client object means this node will also act as a client
	IsWaiting              bool
	SelfPeer               p2p.Peer // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	Self                   p2p.Peer
	IDCPeer                p2p.Peer
	SyncNode               bool // TODO(minhdoan): Remove it later.
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
	selected, unselected, invalid, crossShardTxs := node.UtxoPool.SelectTransactionsForNewBlock(node.pendingTransactions, maxNumTxs)
	_ = invalid // invalid txs are discard

	node.log.Debug("Invalid transactions discarded", "number", len(invalid))
	node.pendingTransactions = unselected
	node.pendingTxMutex.Unlock()
	return selected, crossShardTxs
}

// StartServer starts a server and process the request by a handler.
func (node *Node) StartServer(port string) {
	if node.SyncNode {
		node.blockchain = syncing.StartBlockSyncing(node.Consensus.GetValidatorPeers())
	}
	fmt.Println("Hello in server now")
	node.log.Debug("Starting server", "node", node, "port", port)

	node.listenOnPort(port)
}

func (node *Node) listenOnPort(port string) {
	addr := net.JoinHostPort("", port)
	listen, err := net.Listen("tcp4", addr)
	if err != nil {
		node.log.Error("Socket listen port failed", "addr", addr, "err", err)
		return
	}
	if listen == nil {
		node.log.Error("Listen returned nil", "addr", addr)
		return
	}
	defer listen.Close()
	backoff := p2p.NewExpBackoff(250*time.Millisecond, 15*time.Second, 2.0)
	for {
		conn, err := listen.Accept()
		if err != nil {
			node.log.Error("Error listening on port.", "port", port,
				"err", err)
			backoff.Sleep()
			continue
		}
		go node.NodeHandler(conn)
	}
}

func (node *Node) String() string {
	return node.Consensus.String()
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

//ConnectIdentityChain connects to identity chain
func (node *Node) ConnectIdentityChain() {
	IDCPeer := node.IDCPeer
	p2p.SendMessage(IDCPeer, identity.ConstructIdentityMessage(identity.Announce, node.SerializeWaitNode()))
}

//NewWaitNode is a way to initiate a waiting no
func NewWaitNode(peer, IDCPeer p2p.Peer) *Node {
	node := Node{}
	node.Self = peer
	node.IDCPeer = IDCPeer
	node.log = log.New()
	return &node
}

// NewNodefromIDC is the new node for identity chain.
func NewNodefromIDC(node *Node, consensus *consensus.Consensus, db *db.LDBDatabase) *Node {

	if consensus != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensus
		node.BlockChannel = make(chan blockchain.Block)

		// Genesis Block
		// TODO(minh): Use or implement new function in blockchain package for this.
		genesisBlock := &blockchain.Blockchain{}
		genesisBlock.Blocks = make([]*blockchain.Block, 0)
		// TODO(RJ): use miner's address as coinbase address
		coinbaseTx := blockchain.NewCoinbaseTX(pki.GetAddressFromInt(1), "0", node.Consensus.ShardID)
		genesisBlock.Blocks = append(genesisBlock.Blocks, blockchain.NewGenesisBlock(coinbaseTx, node.Consensus.ShardID))
		node.blockchain = genesisBlock

		// UTXO pool from Genesis block
		//node.UtxoPool = blockchain.CreateUTXOPoolFromGenesisBlockChain(node.blockchain)

		// Initialize level db.
		node.db = db

	}
	// Logger
	node.log = log.New()

	return node
}

// TODO(alok): Fix it or revert the cl on 11/8.
// func (node *Node) processPOWMessage(message []byte) {
// 	payload, err := identity.GetIdentityMessagePayload(message)
// 	if err != nil {
// 		fmt.Println("Could not read payload")
// 	}
// 	IDCPeer := node.IDCPeer
// 	// 4 byte challengeNonce id
// 	req := string(payload)
// 	proof, _ := pow.Fulfil(req, []byte("")) //"This could be blockhasdata"
// 	buffer := bytes.NewBuffer([]byte{})
// 	proofBytes := make([]byte, 32) //proof seems to be 32 byte here
// 	copy(proofBytes[:], proof)
// 	buffer.Write(proofBytes)
// 	buffer.Write(node.SerializeWaitNode())
// 	msgPayload := buffer.Bytes()
// 	p2p.SendMessage(IDCPeer, identity.ConstructIdentityMessage(identity.Register, msgPayload))
// }

// SerializeWaitNode serializes the node
// https://stackoverflow.com/questions/12854125/how-do-i-dump-the-struct-into-the-byte-array-without-reflection/12854659#12854659
func (node *Node) SerializeWaitNode() []byte {
	//Needs to escape the serialization of unexported fields
	result := new(bytes.Buffer)
	encoder := gob.NewEncoder(result)
	err := encoder.Encode(node.Self)
	if err != nil {
		fmt.Println("Could not serialize node")
		fmt.Println("ERROR", err)
		//node.log.Error("Could not serialize node")
	}
	err = encoder.Encode(node.IDCPeer)
	return result.Bytes()
}

// DeserializeWaitNode deserializes the node
func DeserializeWaitNode(d []byte) *Node {
	var wn Node
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn.Self)
	if err != nil {
		log.Error("Could not de-serialize node")
	}
	err = decoder.Decode(&wn.IDCPeer)
	if err != nil {
		log.Error("Could not de-serialize node")
	}
	return &wn
}

// New creates a new node.
func New(consensus *consensus.Consensus, db *db.LDBDatabase) *Node {
	node := Node{}

	if consensus != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensus
		node.BlockChannel = make(chan blockchain.Block)

		// Genesis Block
		// TODO(minh): Use or implement new function in blockchain package for this.
		genesisBlock := &blockchain.Blockchain{}
		genesisBlock.Blocks = make([]*blockchain.Block, 0)
		// TODO(RJ): use miner's address as coinbase address
		coinbaseTx := blockchain.NewCoinbaseTX(pki.GetAddressFromInt(1), "0", node.Consensus.ShardID)
		genesisBlock.Blocks = append(genesisBlock.Blocks, blockchain.NewGenesisBlock(coinbaseTx, node.Consensus.ShardID))
		node.blockchain = genesisBlock

		// UTXO pool from Genesis block
		node.UtxoPool = blockchain.CreateUTXOPoolFromGenesisBlock(node.blockchain.Blocks[0])

		// Initialize level db.
		node.db = db
	}
	// Logger
	node.log = log.New()

	return &node
}
