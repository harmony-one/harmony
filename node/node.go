package node

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/node/worker"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/client"
	bft "github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/crypto/pki"
	hdb "github.com/harmony-one/harmony/db"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	proto_node "github.com/harmony-one/harmony/proto/node"

	"github.com/jinzhu/copier"
)

type NodeState byte

const (
	INIT    NodeState = iota // Node just started, before contacting BeaconChain
	WAIT                     // Node contacted BeaconChain, wait to join Shard
	JOIN                     // Node joined Shard, ready for consensus
	OFFLINE                  // Node is offline
)

type NetworkNode struct {
	SelfPeer p2p.Peer
	IDCPeer  p2p.Peer
}

// Node represents a program (machine) participating in the network
// TODO(minhdoan, rj): consider using BlockChannel *chan blockchain.Block for efficiency.
type Node struct {
	Consensus              *bft.Consensus                     // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan blockchain.Block              // The channel to receive new blocks from Node
	pendingTransactions    []*blockchain.Transaction          // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*blockchain.Transaction          // The transactions selected into the new block and under Consensus process
	blockchain             *blockchain.Blockchain             // The blockchain for the shard where this node belongs
	db                     *hdb.LDBDatabase                   // LevelDB to store blockchain.
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
	IDCPeer                p2p.Peer

	SyncNode  bool                 // TODO(minhdoan): Remove it later.
	chain     *core.BlockChain     // Account Model
	Neighbors map[string]*p2p.Peer // All the neighbor nodes, key is the sha256 of Peer IP/Port
	State     NodeState            // State of the Node

	// Account Model
	Chain               *core.BlockChain
	TxPool              *core.TxPool
	BlockChannelAccount chan *types.Block // The channel to receive new blocks from Node
	worker              *worker.Worker

	// Test only
	testBankKey *ecdsa.PrivateKey
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
		// Disable this temporarily.
		// node.blockchain = syncing.StartBlockSyncing(node.Consensus.GetValidatorPeers())
	}
	fmt.Println("going to start server on port:", port)
	//node.log.Debug("Starting server", "node", node, "port", port)
	node.listenOnPort(port)
}

func (node *Node) SetLog() *Node {
	node.log = log.New()
	return node
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
func (node *Node) ConnectBeaconChain() {
	Nnode := &NetworkNode{SelfPeer: node.SelfPeer, IDCPeer: node.IDCPeer}
	msg := node.SerializeNode(Nnode)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	p2p.SendMessage(node.IDCPeer, msgToSend)
}

// SerializeNode serializes the node
// https://stackoverflow.com/questions/12854125/how-do-i-dump-the-struct-into-the-byte-array-without-reflection/12854659#12854659
func (node *Node) SerializeNode(nnode *NetworkNode) []byte {
	//Needs to escape the serialization of unexported fields
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(nnode)
	if err != nil {
		fmt.Println("Could not serialize node")
		fmt.Println("ERROR", err)
		//node.log.Error("Could not serialize node")
	}

	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeNode(d []byte) *NetworkNode {
	var wn NetworkNode
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize node 1")
	}
	return &wn
}

// New creates a new node.
func New(consensus *bft.Consensus, db *hdb.LDBDatabase) *Node {
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

		// (account model)

		node.testBankKey, _ = crypto.GenerateKey()
		testBankAddress := crypto.PubkeyToAddress(node.testBankKey.PublicKey)
		testBankFunds := big.NewInt(1000000000000000000)
		database := hdb.NewMemDatabase()
		gspec := core.Genesis{
			Alloc: core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
		}

		genesis := gspec.MustCommit(database)
		fmt.Println(genesis.Root())
		chain, _ := core.NewBlockChain(database, nil, gspec.Config, bft.NewFaker(), vm.Config{}, nil)

		node.Chain = chain
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.BlockChannelAccount = make(chan *types.Block)
		node.worker = worker.New(params.TestChainConfig, chain, bft.NewFaker())

	}
	// Logger
	node.log = log.New()
	node.Neighbors = make(map[string]*p2p.Peer)
	node.State = INIT

	return &node
}

// Add neighbors nodes
func (node *Node) AddPeers(peers []p2p.Peer) int {
	count := 0
	for _, p := range peers {
		key := fmt.Sprintf("%v", p.PubKey)
		_, ok := node.Neighbors[key]
		if !ok {
			np := new(p2p.Peer)
			copier.Copy(np, &p)
			node.Neighbors[key] = np
			count++
		}
	}

	if count > 0 {
		c := node.Consensus.AddPeers(peers)
		node.log.Info("Added in Consensus", "# of peers", c)
	}
	return count
}

func (node *Node) JoinShard(leader p2p.Peer) {
	// try to join the shard, with 10 minutes time-out
	backoff := p2p.NewExpBackoff(1*time.Second, 10*time.Minute, 2)

	for node.State == WAIT {
		backoff.Sleep()
		ping := proto_node.NewPingMessage(node.SelfPeer)
		buffer := ping.ConstructPingMessage()

		p2p.SendMessage(leader, buffer)
		node.log.Debug("Sent ping message")
	}
}
