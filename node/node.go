package node

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/client"
	bft "github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/pki"
	hdb "github.com/harmony-one/harmony/db"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	proto_node "github.com/harmony-one/harmony/proto/node"
	"github.com/harmony-one/harmony/services/syncing"
	"github.com/harmony-one/harmony/services/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/services/syncing/downloader/proto"
)

// State is a state of a node.
type State byte

// All constants except the NodeLeader below are for validators only.
const (
	NodeInit              State = iota // Node just started, before contacting BeaconChain
	NodeWaitToJoin                     // Node contacted BeaconChain, wait to join Shard
	NodeJoinedShard                    // Node joined Shard, ready for consensus
	NodeOffline                        // Node is offline
	NodeReadyForConsensus              // Node is ready for doing consensus
	NodeDoingConsensus                 // Node is already doing consensus
	NodeLeader                         // Node is the leader of some shard.
)

// Constants related to doing syncing.
const (
	NotDoingSyncing uint32 = iota
	DoingSyncing
)

const (
	syncingPortDifference = 3000
	waitBeforeJoinShard   = time.Second * 3
	timeOutToJoinShard    = time.Minute * 10
)

// NetworkNode ...
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
	SelfPeer               p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	IDCPeer                p2p.Peer

	chain     *core.BlockChain // Account Model
	Neighbors sync.Map         // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	State     State            // State of the Node

	// Account Model
	pendingTransactionsAccount types.Transactions // TODO: replace with txPool
	pendingTxMutexAccount      sync.Mutex
	Chain                      *core.BlockChain
	TxPool                     *core.TxPool
	BlockChannelAccount        chan *types.Block // The channel to receive new blocks from Node
	Worker                     *worker.Worker

	// Syncing component.
	downloaderServer *downloader.Server
	stateSync        *syncing.StateSync
	syncingState     uint32

	// Test only
	TestBankKeys []*ecdsa.PrivateKey
	ContractKeys []*ecdsa.PrivateKey
	// The p2p host used to send/receive p2p messages
	host host.Host

	// Channel to stop sending ping message
	StopPing chan struct{}
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

// Add new transactions to the pending transaction list
func (node *Node) addPendingTransactionsAccount(newTxs types.Transactions) {
	node.pendingTxMutexAccount.Lock()
	node.pendingTransactionsAccount = append(node.pendingTransactionsAccount, newTxs...)
	node.pendingTxMutexAccount.Unlock()
	node.log.Debug("Got more transactions (account model)", "num", len(newTxs), "totalPending", len(node.pendingTransactionsAccount), "node", node)
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

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlockAccount(maxNumTxs int) (types.Transactions, []*blockchain.CrossShardTxAndProof) {
	node.pendingTxMutexAccount.Lock()
	selected, unselected, invalid := node.Worker.SelectTransactionsForNewBlock(node.pendingTransactionsAccount, maxNumTxs)
	_ = invalid // invalid txs are discard

	node.log.Debug("Invalid transactions discarded", "number", len(invalid))
	node.pendingTransactionsAccount = unselected
	node.log.Debug("Remaining pending transactions", "number", len(node.pendingTransactionsAccount))
	node.pendingTxMutexAccount.Unlock()
	return selected, nil //TODO: replace cross-shard proofs for account model
}

// StartServer starts a server and process the request by a handler.
func (node *Node) StartServer() {
	node.host.BindHandlerAndServe(node.StreamHandler)
}

// SetLog sets log for Node.
func (node *Node) SetLog() *Node {
	node.log = log.New()
	return node
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

// Count the total number of transactions in the blockchain
// Currently used for stats reporting purpose
func (node *Node) countNumTransactionsInBlockchainAccount() int {
	count := 0
	for curBlock := node.Chain.CurrentBlock(); curBlock != nil; {
		count += len(curBlock.Transactions())
		curBlock = node.Chain.GetBlockByHash(curBlock.ParentHash())
	}
	return count
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

//AddSmartContractsToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddSmartContractsToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractKeys[0]
	contractData := "0x608060405234801561001057600080fd5b506040516020806104c08339810180604052602081101561003057600080fd5b505160008054600160a060020a0319163317808255600160a060020a031681526001602081905260409091205560ff811661006c600282610073565b50506100bd565b8154818355818111156100975760008381526020902061009791810190830161009c565b505050565b6100ba91905b808211156100b657600081556001016100a2565b5090565b90565b6103f4806100cc6000396000f3fe60806040526004361061005b577c010000000000000000000000000000000000000000000000000000000060003504635c19a95c8114610060578063609ff1bd146100955780639e7b8d61146100c0578063b3f98adc146100f3575b600080fd5b34801561006c57600080fd5b506100936004803603602081101561008357600080fd5b5035600160a060020a0316610120565b005b3480156100a157600080fd5b506100aa610280565b6040805160ff9092168252519081900360200190f35b3480156100cc57600080fd5b50610093600480360360208110156100e357600080fd5b5035600160a060020a03166102eb565b3480156100ff57600080fd5b506100936004803603602081101561011657600080fd5b503560ff16610348565b3360009081526001602081905260409091209081015460ff1615610144575061027d565b5b600160a060020a0382811660009081526001602081905260409091200154620100009004161580159061019c5750600160a060020a0382811660009081526001602081905260409091200154620100009004163314155b156101ce57600160a060020a039182166000908152600160208190526040909120015462010000900490911690610145565b600160a060020a0382163314156101e5575061027d565b6001818101805460ff1916821775ffffffffffffffffffffffffffffffffffffffff0000191662010000600160a060020a0386169081029190911790915560009081526020829052604090209081015460ff16156102725781546001820154600280549091610100900460ff1690811061025b57fe5b60009182526020909120018054909101905561027a565b815481540181555b50505b50565b600080805b60025460ff821610156102e6578160028260ff168154811015156102a557fe5b906000526020600020016000015411156102de576002805460ff83169081106102ca57fe5b906000526020600020016000015491508092505b600101610285565b505090565b600054600160a060020a0316331415806103215750600160a060020a0381166000908152600160208190526040909120015460ff165b1561032b5761027d565b600160a060020a0316600090815260016020819052604090912055565b3360009081526001602081905260409091209081015460ff1680610371575060025460ff831610155b1561037c575061027d565b6001818101805460ff191690911761ff00191661010060ff8516908102919091179091558154600280549192909181106103b257fe5b600091825260209091200180549091019055505056fea165627a7a72305820164189ef302b4648e01e22456b0a725191604cb63ee472f230ef6a2d17d702f900290000000000000000000000000000000000000000000000000000000000000002"
	dataEnc := common.FromHex(contractData)
	// Unsigned transaction to avoid the case of transaction address.
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), 0, big.NewInt(1000000), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, priKey)
	node.pendingTransactionsAccount = append(node.pendingTransactionsAccount, mycontracttx)
}

// New creates a new node.
func New(host host.Host, consensus *bft.Consensus, db *hdb.LDBDatabase) *Node {
	node := Node{}

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	if host != nil && consensus != nil {
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
		rand.Seed(0)
		len := 1000000
		bytes := make([]byte, len)
		for i := 0; i < len; i++ {
			bytes[i] = byte(rand.Intn(100))
		}
		reader := strings.NewReader(string(bytes))
		genesisAloc := make(core.GenesisAlloc)
		for i := 0; i < 100; i++ {
			testBankKey, _ := ecdsa.GenerateKey(crypto.S256(), reader)
			testBankAddress := crypto.PubkeyToAddress(testBankKey.PublicKey)
			testBankFunds := big.NewInt(8000000000000000000)
			genesisAloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
			node.TestBankKeys = append(node.TestBankKeys, testBankKey)
		}

		contractKey, _ := ecdsa.GenerateKey(crypto.S256(), reader)
		contractAddress := crypto.PubkeyToAddress(contractKey.PublicKey)
		contractFunds := big.NewInt(8000000000000000000)
		genesisAloc[contractAddress] = core.GenesisAccount{Balance: contractFunds}
		node.ContractKeys = append(node.ContractKeys, contractKey)

		database := hdb.NewMemDatabase()
		chainConfig := params.TestChainConfig
		chainConfig.ChainID = big.NewInt(int64(node.Consensus.ShardID)) // Use ChainID as piggybacked ShardID
		gspec := core.Genesis{
			Config:  chainConfig,
			Alloc:   genesisAloc,
			ShardID: uint32(node.Consensus.ShardID),
		}

		_ = gspec.MustCommit(database)
		chain, _ := core.NewBlockChain(database, nil, gspec.Config, node.Consensus, vm.Config{}, nil)
		node.Chain = chain
		//This one is not used --- RJ.
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.BlockChannelAccount = make(chan *types.Block)
		node.Worker = worker.New(params.TestChainConfig, chain, node.Consensus, pki.GetAddressFromPublicKey(node.SelfPeer.PubKey))
		//Initialize the pending transactions with smart contract transactions
		node.AddSmartContractsToPendingTransactions()
	}

	// Logger
	node.log = log.New()
	if consensus.IsLeader {
		node.State = NodeLeader
	} else {
		node.State = NodeInit
	}

	// Setup initial state of syncing.
	node.syncingState = NotDoingSyncing
	node.StopPing = make(chan struct{})

	return &node
}

// DoSyncing starts syncing.
func (node *Node) DoSyncing() {
	// If this node is currently doing sync, another call for syncing will be returned immediately.
	if !atomic.CompareAndSwapUint32(&node.syncingState, NotDoingSyncing, DoingSyncing) {
		return
	}
	defer atomic.StoreUint32(&node.syncingState, NotDoingSyncing)
	if node.stateSync == nil {
		node.stateSync = syncing.GetStateSync()
	}
	if node.stateSync.StartStateSync(node.GetSyncingPeers(), node.blockchain) {
		node.log.Debug("DoSyncing: successfully sync")
		if node.State == NodeJoinedShard {
			node.State = NodeReadyForConsensus
		}
	} else {
		node.log.Debug("DoSyncing: failed to sync")
	}
}

// AddPeers adds neighbors nodes
func (node *Node) AddPeers(peers []*p2p.Peer) int {
	count := 0
	for _, p := range peers {
		key := fmt.Sprintf("%v", p.PubKey)
		_, ok := node.Neighbors.Load(key)
		if !ok {
			node.Neighbors.Store(key, *p)
			count++
			continue
		}
		if node.SelfPeer.ValidatorID == -1 && p.IP == node.SelfPeer.IP && p.Port == node.SelfPeer.Port {
			node.SelfPeer.ValidatorID = p.ValidatorID
		}
	}

	if count > 0 {
		node.Consensus.AddPeers(peers)
	}
	return count
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-syncingPortDifference)
	}
	os.Exit(1)
	return ""
}

// GetSyncingPeers returns list of peers.
// Right now, the list length is only 1 for testing.
// TODO(mihdoan): fix it later.
func (node *Node) GetSyncingPeers() []p2p.Peer {
	res := []p2p.Peer{}
	node.Neighbors.Range(func(k, v interface{}) bool {
		node.log.Debug("GetSyncingPeers-Range: ", "k", k, "v", v)
		if len(res) == 0 {
			res = append(res, v.(p2p.Peer))
		}
		return true
	})

	for i := range res {
		res[i].Port = GetSyncingPort(res[i].Port)
	}
	node.log.Debug("GetSyncingPeers: ", "res", res)
	return res
}

// JoinShard helps a new node to join a shard.
func (node *Node) JoinShard(leader p2p.Peer) {
	// try to join the shard, send ping message every 1 second, with a 10 minutes time-out
	tick := time.NewTicker(1 * time.Second)
	timeout := time.NewTicker(10 * time.Minute)
	defer tick.Stop()
	defer timeout.Stop()

	for {
		select {
		case <-tick.C:
			ping := proto_node.NewPingMessage(node.SelfPeer)
			if node.Client != nil { // assume this is the client node
				ping.Node.Role = proto_node.ClientRole
			}
			buffer := ping.ConstructPingMessage()

			// Talk to leader.
			node.SendMessage(leader, buffer)
		case <-timeout.C:
			node.log.Info("JoinShard timeout")
			return
		case <-node.StopPing:
			node.log.Info("Stopping JoinShard")
			return
		}
	}
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer() {
	node.downloaderServer = downloader.NewServer(node)
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer() {
	port := GetSyncingPort(node.SelfPeer.Port)
	node.log.Info("support_sycning: StartSyncingServer on port:", "port", port)
	node.downloaderServer.Start(node.SelfPeer.IP, GetSyncingPort(node.SelfPeer.Port))
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(request *downloader_pb.DownloaderRequest) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	if request.Type == downloader_pb.DownloaderRequest_HEADER {
		for _, block := range node.blockchain.Blocks {
			response.Payload = append(response.Payload, block.Hash[:])
		}
	} else {
		for i := range request.Hashes {
			block := node.blockchain.FindBlock(request.Hashes[i])
			response.Payload = append(response.Payload, block.Serialize())
		}
	}
	return response, nil
}
