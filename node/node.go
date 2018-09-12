package node

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"sync"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/client"
	"github.com/simple-rules/harmony-benchmark/consensus"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/db"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/pow"
	"github.com/simple-rules/harmony-benchmark/proto/identity"
	proto_node "github.com/simple-rules/harmony-benchmark/proto/node"
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
	Self                   p2p.Peer
	IDCPeer                p2p.Peer
	SyncNode               bool // TODO(minhdoan): Remove it later.
}

type SyncPeerConfig struct {
	peer        p2p.Peer
	conn        net.Conn
	block       *blockchain.Block
	w           *bufio.Writer
	receivedMsg []byte
	err         error
}

type SyncConfig struct {
	peers []SyncPeerConfig
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
	node.pendingTransactions = unselected
	node.pendingTxMutex.Unlock()
	return selected, crossShardTxs
}

// Start a server and process the request by a handler.
func (node *Node) StartServer(port string) {
	if node.SyncNode {
		node.startBlockSyncing()
	}
	fmt.Println("Hello in server now")
	node.log.Debug("Starting server", "node", node, "port", port)

	node.listenOnPort(port)
}

func (node *Node) listenOnPort(port string) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer func(listen net.Listener) {
		if listen != nil {
			listen.Close()
		}
	}(listen)
	if err != nil {
		node.log.Error("Socket listen port failed", "port", port, "err", err)
		return
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			node.log.Error("Error listening on port.", "port", port)
			continue
		}
		go node.NodeHandler(conn)
	}
}
func (node *Node) startBlockSyncing() {
	peers := node.Consensus.GetValidatorPeers()
	peer_number := len(peers)
	syncConfig := SyncConfig{
		peers: make([]SyncPeerConfig, peer_number),
	}
	for id := range syncConfig.peers {
		syncConfig.peers[id].peer = peers[id]
	}

	var wg sync.WaitGroup
	wg.Add(peer_number)

	for id := range syncConfig.peers {
		go func(peerConfig *SyncPeerConfig) {
			defer wg.Done()
			peerConfig.conn, peerConfig.err = p2p.DialWithSocketClient(peerConfig.peer.Ip, peerConfig.peer.Port)
		}(&syncConfig.peers[id])
	}
	wg.Wait()

	activePeerNumber := 0
	for _, configPeer := range syncConfig.peers {
		if configPeer.err == nil {
			activePeerNumber++
			configPeer.w = bufio.NewWriter(configPeer.conn)
		}
	}
	var prevHash [32]byte
	for {
		var wg sync.WaitGroup
		wg.Add(activePeerNumber)

		for _, configPeer := range syncConfig.peers {
			if configPeer.err != nil {
				continue
			}
			go func(peerConfig *SyncPeerConfig, prevHash [32]byte) {
				msg := proto_node.ConstructBlockchainSyncMessage(proto_node.GET_BLOCK, prevHash)
				peerConfig.w.Write(msg)
				peerConfig.w.Flush()
			}(&configPeer, prevHash)
		}
		wg.Wait()
		wg.Add(activePeerNumber)

		for _, configPeer := range syncConfig.peers {
			if configPeer.err != nil {
				continue
			}
			go func(peerConfig *SyncPeerConfig, prevHash [32]byte) {
				defer wg.Done()
				peerConfig.receivedMsg, peerConfig.err = p2p.ReadMessageContent(peerConfig.conn)
				if peerConfig.err == nil {
					peerConfig.block, peerConfig.err = blockchain.DeserializeBlock(peerConfig.receivedMsg)
				}
			}(&configPeer, prevHash)
		}
		wg.Wait()
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
	p2p.SendMessage(IDCPeer, identity.ConstructIdentityMessage(identity.ANNOUNCE, node.SerializeWaitNode()))
	return
}

//NewWaitNode is a way to initiate a waiting no
func NewWaitNode(peer, IDCPeer p2p.Peer) *Node {
	node := Node{}
	node.Self = peer
	node.IDCPeer = IDCPeer
	node.log = log.New()
	return &node
}

//NewNodefromIDC
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

func (node *Node) processPOWMessage(message []byte) {
	payload, err := identity.GetIdentityMessagePayload(message)
	if err != nil {
		fmt.Println("Could not read payload")
	}
	IDCPeer := node.IDCPeer
	// 4 byte challengeNonce id
	req := string(payload)
	proof, _ := pow.Fulfil(req, []byte("")) //"This could be blockhasdata"
	buffer := bytes.NewBuffer([]byte{})
	proofBytes := make([]byte, 32) //proof seems to be 32 byte here
	copy(proofBytes[:], proof)
	buffer.Write(proofBytes)
	buffer.Write(node.SerializeWaitNode())
	msgPayload := buffer.Bytes()
	p2p.SendMessage(IDCPeer, identity.ConstructIdentityMessage(identity.REGISTER, msgPayload))
}

//https://stackoverflow.com/questions/12854125/how-do-i-dump-the-struct-into-the-byte-array-without-reflection/12854659#12854659
//SerializeWaitNode serializes the node
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

// Create a new Node
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
