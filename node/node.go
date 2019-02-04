package node

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	service_manager "github.com/harmony-one/harmony/api/service"
	blockproposal "github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	consensus_service "github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/discovery"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/api/service/networkinfo"
	"github.com/harmony-one/harmony/api/service/staking"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	bft "github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// State is a state of a node.
type State byte

// All constants except the NodeLeader below are for validators only.
const (
	NodeInit              State = iota // Node just started, before contacting BeaconChain
	NodeWaitToJoin                     // Node contacted BeaconChain, wait to join Shard
	NodeNotInSync                      // Node out of sync, might be just joined Shard or offline for a period of time
	NodeOffline                        // Node is offline
	NodeReadyForConsensus              // Node is ready for doing consensus
	NodeDoingConsensus                 // Node is already doing consensus
	NodeLeader                         // Node is the leader of some shard.
)

// Role defines a role of a node.
type Role byte

// All constants for different node roles.
const (
	Unknown Role = iota
	ShardLeader
	ShardValidator
	BeaconLeader
	BeaconValidator
	NewNode
)

func (state State) String() string {
	switch state {
	case NodeInit:
		return "NodeInit"
	case NodeWaitToJoin:
		return "NodeWaitToJoin"
	case NodeNotInSync:
		return "NodeNotInSync"
	case NodeOffline:
		return "NodeOffline"
	case NodeReadyForConsensus:
		return "NodeReadyForConsensus"
	case NodeDoingConsensus:
		return "NodeDoingConsensus"
	case NodeLeader:
		return "NodeLeader"
	}
	return "Unknown"
}

// Constants related to doing syncing.
const (
	lastMileThreshold = 4
	inSyncThreshold   = 1
)

const (
	waitBeforeJoinShard = time.Second * 3
	timeOutToJoinShard  = time.Minute * 10
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff       = 5555
	maxBroadcastNodes           = 10                  // broadcast at most maxBroadcastNodes peers that need in sync
	broadcastTimeout      int64 = 3 * 60 * 1000000000 // 3 mins
)

// use to push new block to outofsync node
type syncConfig struct {
	timestamp int64
	client    *downloader.Client
}

// NetworkNode ...
type NetworkNode struct {
	SelfPeer p2p.Peer
	IDCPeer  p2p.Peer
}

// Node represents a protocol-participating node in the network
type Node struct {
	Consensus              *bft.Consensus       // Consensus object containing all Consensus related data (e.g. committee members, signatures, commits)
	BlockChannel           chan *types.Block    // The channel to receive new blocks from Node
	pendingTransactions    types.Transactions   // All the transactions received but not yet processed for Consensus
	transactionInConsensus []*types.Transaction // The transactions selected into the new block and under Consensus process
	pendingTxMutex         sync.Mutex

	blockchain *core.BlockChain   // The blockchain for the shard where this node belongs
	db         *ethdb.LDBDatabase // LevelDB to store blockchain.

	ClientPeer *p2p.Peer      // The peer for the harmony tx generator client, used for leaders to return proof-of-accept
	Client     *client.Client // The presence of a client object means this node will also act as a client
	SelfPeer   p2p.Peer       // TODO(minhdoan): it could be duplicated with Self below whose is Alok work.
	IDCPeer    p2p.Peer

	Neighbors  sync.Map   // All the neighbor nodes, key is the sha256 of Peer IP/Port, value is the p2p.Peer
	State      State      // State of the Node
	stateMutex sync.Mutex // mutex for change node state

	TxPool *core.TxPool
	Worker *worker.Worker

	// Client server (for wallet requests)
	clientServer *clientService.Server

	// Syncing component.
	downloaderServer       *downloader.Server
	stateSync              *syncing.StateSync
	peerRegistrationRecord map[uint32]*syncConfig // record registration time (unixtime) of peers begin in syncing

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// The pubsub structure to communicate with peers
	Pubsub *pubsub.PubSub

	// Subscriptions of the node
	Subscriptions []*pubsub.Subscription

	// Channel to stop sending ping message
	StopPing chan struct{}

	// Signal channel for lost validators
	OfflinePeers chan p2p.Peer

	// Node Role.
	Role Role

	// Service manager.
	serviceManager *service_manager.Manager

	// For test only
	TestBankKeys      []*ecdsa.PrivateKey
	ContractKeys      []*ecdsa.PrivateKey
	ContractAddresses []common.Address
}

// Blockchain returns the blockchain from node
func (node *Node) Blockchain() *core.BlockChain {
	return node.blockchain
}

// Add new transactions to the pending transaction list
func (node *Node) addPendingTransactions(newTxs types.Transactions) {
	node.pendingTxMutex.Lock()
	node.pendingTransactions = append(node.pendingTransactions, newTxs...)
	node.pendingTxMutex.Unlock()
	utils.GetLogInstance().Debug("Got more transactions", "num", len(newTxs), "totalPending", len(node.pendingTransactions))
}

// Take out a subset of valid transactions from the pending transaction list
// Note the pending transaction list will then contain the rest of the txs
func (node *Node) getTransactionsForNewBlock(maxNumTxs int) types.Transactions {
	node.pendingTxMutex.Lock()
	selected, unselected, invalid := node.Worker.SelectTransactionsForNewBlock(node.pendingTransactions, maxNumTxs)
	_ = invalid // invalid txs are discard

	utils.GetLogInstance().Debug("Invalid transactions discarded", "number", len(invalid))
	node.pendingTransactions = unselected
	utils.GetLogInstance().Debug("Remaining pending transactions", "number", len(node.pendingTransactions))
	node.pendingTxMutex.Unlock()
	return selected
}

// StartServer starts a server and process the requests by a handler.
func (node *Node) StartServer() {
	node.host.BindHandlerAndServe(node.StreamHandler)
}

// Count the total number of transactions in the blockchain
// Currently used for stats reporting purpose
func (node *Node) countNumTransactionsInBlockchain() int {
	count := 0
	for block := node.blockchain.CurrentBlock(); block != nil; block = node.blockchain.GetBlockByHash(block.Header().ParentHash) {
		count += len(block.Transactions())
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
		//utils.GetLogInstance().Error("Could not serialize node")
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
func New(host p2p.Host, consensus *bft.Consensus, db ethdb.Database) *Node {
	node := Node{}

	if host != nil {
		node.host = host
		node.SelfPeer = host.GetSelfPeer()
	}

	if host != nil && consensus != nil {
		// Consensus and associated channel to communicate blocks
		node.Consensus = consensus

		// Initialize genesis block and blockchain
		genesisAlloc := node.CreateGenesisAllocWithTestingAddresses(100)
		contractKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"))
		contractAddress := crypto.PubkeyToAddress(contractKey.PublicKey)
		contractFunds := big.NewInt(9000000)
		contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
		genesisAlloc[contractAddress] = core.GenesisAccount{Balance: contractFunds}
		node.ContractKeys = append(node.ContractKeys, contractKey)

		database := db
		if database == nil {
			database = ethdb.NewMemDatabase()
		}

		chainConfig := params.TestChainConfig
		chainConfig.ChainID = big.NewInt(int64(node.Consensus.ShardID)) // Use ChainID as piggybacked ShardID
		gspec := core.Genesis{
			Config:  chainConfig,
			Alloc:   genesisAlloc,
			ShardID: uint32(node.Consensus.ShardID),
		}

		_ = gspec.MustCommit(database)
		chain, _ := core.NewBlockChain(database, nil, gspec.Config, node.Consensus, vm.Config{}, nil)
		node.blockchain = chain
		node.BlockChannel = make(chan *types.Block)
		node.TxPool = core.NewTxPool(core.DefaultTxPoolConfig, params.TestChainConfig, chain)
		node.Worker = worker.New(params.TestChainConfig, chain, node.Consensus, pki.GetAddressFromPublicKey(node.SelfPeer.PubKey), node.Consensus.ShardID)
		node.AddFaucetContractToPendingTransactions()
		if node.Role == BeaconLeader {
			node.AddDepositContractToPendingTransactions()
		}
		node.Consensus.ConsensusBlock = make(chan *bft.BFTBlockInfo)
		node.Consensus.VerifiedNewBlock = make(chan *types.Block)

		var err error
		node.Pubsub, err = pubsub.NewFloodSub(context.Background(), node.host.GetP2PHost())
		if err != nil {
			utils.GetLogInstance().Error("[NODE]", "NewPubSub Error", err)
		}
		subch, err := node.Pubsub.Subscribe("beacon")
		node.Subscriptions = append(node.Subscriptions, subch)
	}

	if consensus != nil && consensus.IsLeader {
		node.State = NodeLeader
	} else {
		node.State = NodeInit
	}

	// Setup initial state of syncing.
	node.StopPing = make(chan struct{})
	node.peerRegistrationRecord = make(map[uint32]*syncConfig)

	node.OfflinePeers = make(chan p2p.Peer)
	go node.RemovePeersHandler()

	return &node
}

// IsOutOfSync checks whether the node is out of sync by comparing latest block with consensus block
func (node *Node) IsOutOfSync(consensusBlockInfo *bft.BFTBlockInfo) bool {
	consensusBlock := consensusBlockInfo.Block
	consensusID := consensusBlockInfo.ConsensusID

	myHeight := node.blockchain.CurrentBlock().NumberU64()
	newHeight := consensusBlock.NumberU64()
	utils.GetLogInstance().Debug("[SYNC]", "myHeight", myHeight, "newHeight", newHeight)
	if newHeight-myHeight <= inSyncThreshold {
		node.stateSync.AddLastMileBlock(consensusBlock)
		node.Consensus.UpdateConsensusID(consensusID + 1)
		return false
	}
	// cache latest blocks for last mile catch up
	if newHeight-myHeight <= lastMileThreshold && node.stateSync != nil {
		node.stateSync.AddLastMileBlock(consensusBlock)
	}
	return true
}

// DoSyncing wait for check status and starts syncing if out of sync
func (node *Node) DoSyncing() {
	for {
		select {
		// in current implementation logic, timeout means in sync
		case <-time.After(5 * time.Second):
			//myHeight := node.blockchain.CurrentBlock().NumberU64()
			//utils.GetLogInstance().Debug("[SYNC]", "currentHeight", myHeight)
			node.stateMutex.Lock()
			node.State = NodeReadyForConsensus
			node.stateMutex.Unlock()
			continue
		case consensusBlockInfo := <-node.Consensus.ConsensusBlock:
			if !node.IsOutOfSync(consensusBlockInfo) {
				if node.State == NodeNotInSync {
					utils.GetLogInstance().Info("[SYNC] Node is now IN SYNC!")
				}
				node.stateMutex.Lock()
				node.State = NodeReadyForConsensus
				node.stateMutex.Unlock()
				// wait for last mile block finish; think a better way
				time.Sleep(200 * time.Millisecond)
				node.stateSync.CloseConnections()
				node.stateSync = nil
				continue
			} else {
				utils.GetLogInstance().Debug("[SYNC] node is out of sync")
				node.stateMutex.Lock()
				node.State = NodeNotInSync
				node.stateMutex.Unlock()
			}

			if node.stateSync == nil {
				node.stateSync = syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port)
				node.stateSync.CreateSyncConfig(node.GetSyncingPeers())
				node.stateSync.MakeConnectionToPeers()
			}
			startHash := node.blockchain.CurrentBlock().Hash()
			node.stateSync.StartStateSync(startHash[:], node.blockchain, node.Worker)
		}
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
			node.host.AddPeer(p)
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
		return fmt.Sprintf("%d", port-syncing.SyncingPortDifference)
	}
	os.Exit(1)
	return ""
}

// GetSyncingPeers returns list of peers.
func (node *Node) GetSyncingPeers() []p2p.Peer {
	res := []p2p.Peer{}
	node.Neighbors.Range(func(k, v interface{}) bool {
		res = append(res, v.(p2p.Peer))
		return true
	})

	removeID := -1
	for i := range res {
		if res[i].Port == node.SelfPeer.Port {
			removeID = i
		}
		res[i].Port = GetSyncingPort(res[i].Port)
	}
	if removeID != -1 {
		res = append(res[:removeID], res[removeID+1:]...)
	}
	utils.GetLogInstance().Debug("GetSyncingPeers: ", "res", res, "self", node.SelfPeer)
	return res
}

//AddFaucetContractToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddFaucetContractToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractKeys[0]
	contractData := "0x6080604052678ac7230489e8000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058203e799228fee2fa7c5d15e71c04267a0cc2687c5eff3b48b98f21f355e1064ab30029"
	dataEnc := common.FromHex(contractData)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(8000000)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, priKey)
	node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0)))

	node.addPendingTransactions(types.Transactions{mycontracttx})
}

//AddDepositContractToPendingTransactions adds the deposit smart contract the genesis block.
func (node *Node) AddDepositContractToPendingTransactions() {
	// Add a contract deployment transaction
	//Generate contract key and associate funds with the smart contract
	priKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Deposit Smart Contract Key"))
	contractAddress := crypto.PubkeyToAddress(priKey.PublicKey)
	//Initially the smart contract should have minimal funds.
	contractFunds := big.NewInt(0)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	contractData := "0x608060405234801561001057600080fd5b50610b51806100206000396000f3fe608060405260043610610072576000357c01000000000000000000000000000000000000000000000000000000009004806325ca4c9c146100775780632e1a7d4d146100e05780634c1b64cb1461012f578063a98e4e7714610194578063d0e30db0146101bf578063e27fd057146101dd575b600080fd5b34801561008357600080fd5b506100c66004803603602081101561009a57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610249565b604051808215151515815260200191505060405180910390f35b3480156100ec57600080fd5b506101196004803603602081101561010357600080fd5b8101908080359060200190929190505050610310565b6040518082815260200191505060405180910390f35b34801561013b57600080fd5b5061017e6004803603602081101561015257600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104cb565b6040518082815260200191505060405180910390f35b3480156101a057600080fd5b506101a96106f3565b6040518082815260200191505060405180910390f35b6101c7610700565b6040518082815260200191505060405180910390f35b3480156101e957600080fd5b506101f2610a46565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561023557808201518184015260208101905061021a565b505050509050019250505060405180910390f35b6000806002805490501415610261576000905061030b565b8173ffffffffffffffffffffffffffffffffffffffff166002600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548154811015156102c657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161490505b919050565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151561048457816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055503373ffffffffffffffffffffffffffffffffffffffff166108fc839081150290604051600060405180830381858888f193505050501580156103eb573d6000803e3d6000fd5b5060008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054141561043e5761043c336104cb565b505b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506104c6565b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b919050565b600080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600260016002805490500381548110151561052957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690508060028381548110151561056657fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555081600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555060028054809190600190036106079190610ad4565b508373ffffffffffffffffffffffffffffffffffffffff167e1fab73a76dc2de66330e055b1c1e3319c77b736bb4478cc706497f318a4ad7836040518082815260200191505060405180910390a28073ffffffffffffffffffffffffffffffffffffffff167f6095abd20e12b7e743432b409b7879ac77a0b927f89ae330f59c15b32dce0b69836000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054604051808381526020018281526020019250505060405180910390a28192505050919050565b6000600280549050905090565b600061070b33610249565b1561087657346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055503373ffffffffffffffffffffffffffffffffffffffff167f6095abd20e12b7e743432b409b7879ac77a0b927f89ae330f59c15b32dce0b69600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054604051808381526020018281526020019250505060405180910390a2600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050610a43565b346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600160023390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555003600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055503373ffffffffffffffffffffffffffffffffffffffff167fd2ad617bb539c9a6219058035b15d87478e478eb0f74164eae890a0c70fa3f40600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054604051808381526020018281526020019250505060405180910390a260016002805490500390505b90565b60606002805480602002602001604051908101604052809291908181526020018280548015610aca57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610a80575b5050505050905090565b815481835581811115610afb57818360005260206000209182019101610afa9190610b00565b5b505050565b610b2291905b80821115610b1e576000816000905550600101610b06565b5090565b9056fea165627a7a7230582032eb9f748231d6ef2fd0ac6bd603cc3e381b6a6596e87beb1f6ce117e5237f4b0029"
	dataEnc := common.FromHex(contractData)
	// Unsigned transaction to avoid the case of transaction address.
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, priKey)
	node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(contractAddress, uint64(0)))
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

//CallFaucetContract invokes the faucet contract to give the walletAddress initial money
func (node *Node) CallFaucetContract(walletAddress common.Address) common.Hash {
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(node.ContractKeys[0].PublicKey))
	callingFunction := "0x27c78c42000000000000000000000000"
	contractData := callingFunction + hex.EncodeToString(walletAddress.Bytes())
	dataEnc := common.FromHex(contractData)
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.ContractKeys[0])

	node.addPendingTransactions(types.Transactions{tx})
	return tx.Hash()
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
			ping := proto_discovery.NewPingMessage(node.SelfPeer)
			if node.Client != nil { // assume this is the client node
				ping.Node.Role = proto_node.ClientRole
			}
			buffer := ping.ConstructPingMessage()

			// Talk to leader.
			node.SendMessage(leader, buffer)
		case <-timeout.C:
			utils.GetLogInstance().Info("JoinShard timeout")
			return
		case <-node.StopPing:
			utils.GetLogInstance().Info("Stopping JoinShard")
			return
		}
	}
}

// SupportClient initializes and starts the client service
func (node *Node) SupportClient() {
	node.InitClientServer()
	node.StartClientServer()
}

// InitClientServer initializes client server.
func (node *Node) InitClientServer() {
	node.clientServer = clientService.NewServer(node.blockchain.State, node.CallFaucetContract)
}

// StartClientServer starts client server.
func (node *Node) StartClientServer() {
	port, _ := strconv.Atoi(node.SelfPeer.Port)
	utils.GetLogInstance().Info("support_client: StartClientServer on port:", "port", port+ClientServicePortDiff)
	node.clientServer.Start(node.SelfPeer.IP, strconv.Itoa(port+ClientServicePortDiff))
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()

	go node.DoSyncing()
	go node.SendNewBlockToUnsync()
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer() {
	node.downloaderServer = downloader.NewServer(node)
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer() {
	utils.GetLogInstance().Info("support_sycning: StartSyncingServer")
	node.downloaderServer.Start(node.SelfPeer.IP, GetSyncingPort(node.SelfPeer.Port))
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(request *downloader_pb.DownloaderRequest) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	switch request.Type {
	case downloader_pb.DownloaderRequest_HEADER:
		utils.GetLogInstance().Debug("[SYNC] CalculateResponse DownloaderRequest_HEADER", "request.BlockHash", request.BlockHash)
		var startHeaderHash []byte
		if request.BlockHash == nil {
			tmp := node.blockchain.Genesis().Hash()
			startHeaderHash = tmp[:]
		} else {
			startHeaderHash = request.BlockHash
		}
		for block := node.blockchain.CurrentBlock(); block != nil; block = node.blockchain.GetBlockByHash(block.Header().ParentHash) {
			blockHash := block.Hash()
			if bytes.Compare(blockHash[:], startHeaderHash) == 0 {
				break
			}
			response.Payload = append(response.Payload, blockHash[:])
		}
	case downloader_pb.DownloaderRequest_BLOCK:
		for _, bytes := range request.Hashes {
			var hash common.Hash
			hash.SetBytes(bytes)
			block := node.blockchain.GetBlockByHash(hash)
			encodedBlock, err := rlp.EncodeToBytes(block)
			if err == nil {
				response.Payload = append(response.Payload, encodedBlock)
			}
		}
	case downloader_pb.DownloaderRequest_NEWBLOCK:
		if node.State != NodeNotInSync {
			utils.GetLogInstance().Debug("[SYNC] new block received, but state is", "state", node.State.String())
			response.Type = downloader_pb.DownloaderResponse_INSYNC
			return response, nil
		}
		var blockObj types.Block
		err := rlp.DecodeBytes(request.BlockHash, &blockObj)
		if err != nil {
			utils.GetLogInstance().Warn("[SYNC] unable to decode received new block")
			return response, err
		}
		node.stateSync.AddNewBlock(request.PeerHash, &blockObj)

	case downloader_pb.DownloaderRequest_REGISTER:
		peerID := binary.BigEndian.Uint32(request.PeerHash)
		if _, ok := node.peerRegistrationRecord[peerID]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			return response, nil
		} else if len(node.peerRegistrationRecord) >= maxBroadcastNodes {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			return response, nil
		} else {
			peer, ok := node.Consensus.GetPeerFromID(peerID)
			if !ok {
				utils.GetLogInstance().Warn("[SYNC] unable to get peer from peerID", "peerID", peerID)
			}
			client := downloader.ClientSetup(peer.IP, GetSyncingPort(peer.Port))
			if client == nil {
				utils.GetLogInstance().Warn("[SYNC] unable to setup client")
				return response, nil
			}
			utils.GetLogInstance().Debug("[SYNC] client setup correctly", "client", client)
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.stateMutex.Lock()
			node.peerRegistrationRecord[peerID] = config
			node.stateMutex.Unlock()
			utils.GetLogInstance().Debug("[SYNC] register peerID success", "peerID", peerID)
			response.Type = downloader_pb.DownloaderResponse_SUCCESS
		}
	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
		if node.State == NodeNotInSync {
			count := node.stateSync.RegisterNodeInfo()
			utils.GetLogInstance().Debug("[SYNC] extra node registered", "number", count)
		}
	}
	return response, nil
}

// SendNewBlockToUnsync send latest verified block to unsync, registered nodes
func (node *Node) SendNewBlockToUnsync() {
	for {
		block := <-node.Consensus.VerifiedNewBlock
		blockHash, err := rlp.EncodeToBytes(block)
		if err != nil {
			utils.GetLogInstance().Warn("[SYNC] unable to encode block to hashes")
			continue
		}

		// really need to have a unique id independent of ip/port
		selfPeerID := utils.GetUniqueIDFromIPPort(node.SelfPeer.IP, node.SelfPeer.Port)
		utils.GetLogInstance().Debug("[SYNC] peerRegistration Record", "peerID", selfPeerID, "number", len(node.peerRegistrationRecord))

		for peerID, config := range node.peerRegistrationRecord {
			elapseTime := time.Now().UnixNano() - config.timestamp
			if elapseTime > broadcastTimeout {
				utils.GetLogInstance().Warn("[SYNC] SendNewBlockToUnsync to peer timeout", "peerID", peerID)
				// send last time and delete
				config.client.PushNewBlock(selfPeerID, blockHash, true)
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
				continue
			}
			response := config.client.PushNewBlock(selfPeerID, blockHash, false)
			if response != nil && response.Type == downloader_pb.DownloaderResponse_INSYNC {
				node.stateMutex.Lock()
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				node.stateMutex.Unlock()
			}
		}
	}
}

// RemovePeersHandler is a goroutine to wait on the OfflinePeers channel
// and remove the peers from validator list
func (node *Node) RemovePeersHandler() {
	for {
		select {
		case p := <-node.OfflinePeers:
			node.Consensus.OfflinePeerList = append(node.Consensus.OfflinePeerList, p)
		}
	}
}

func (node *Node) setupForShardLeader() {
	// Register explorer service.
	node.serviceManager.RegisterService(service_manager.SupportExplorer, explorer.New(&node.SelfPeer))
	// Register consensus service.
	node.serviceManager.RegisterService(service_manager.Consensus, consensus_service.NewService(node.BlockChannel, node.Consensus))
	// Register new block service.
	node.serviceManager.RegisterService(service_manager.BlockProposal, blockproposal.NewService(node.Consensus.ReadySignal, node.WaitForConsensusReady))
	// Register client support service.
	node.serviceManager.RegisterService(service_manager.ClientSupport, clientsupport.NewService(node.blockchain.State, node.CallFaucetContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// Register peer discovery service.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, fmt.Sprintf("%v", node.Consensus.ShardID)))
}

func (node *Node) setupForShardValidator() {
	// Register peer discovery service.
	node.serviceManager.RegisterService(service_manager.PeerDiscovery, discovery.New(node.host, fmt.Sprintf("%v", node.Consensus.ShardID)))
}

func (node *Node) setupForBeaconLeader() {
	// Register consensus service.
	node.serviceManager.RegisterService(service_manager.Consensus, consensus_service.NewService(node.BlockChannel, node.Consensus))
	// Register new block service.
	node.serviceManager.RegisterService(service_manager.BlockProposal, blockproposal.NewService(node.Consensus.ReadySignal, node.WaitForConsensusReady))
	// Register client support service.
	node.serviceManager.RegisterService(service_manager.ClientSupport, clientsupport.NewService(node.blockchain.State, node.CallFaucetContract, node.SelfPeer.IP, node.SelfPeer.Port))
}

func (node *Node) setupForBeaconValidator() {
}

func (node *Node) setupForNewNode() {
	chanPeer := make(chan *p2p.Peer)
	// Add network info serivce.
	node.serviceManager.RegisterService(service_manager.NetworkInfo, networkinfo.NewService(chanPeer))
	// Add staking service.
	node.serviceManager.RegisterService(service_manager.Staking, staking.NewService(chanPeer))
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() {
	node.serviceManager = &service_manager.Manager{}
	switch node.Role {
	case ShardLeader:
		node.setupForShardLeader()
	case ShardValidator:
		node.setupForShardValidator()
	case BeaconLeader:
		node.setupForBeaconLeader()
	case BeaconValidator:
		node.setupForBeaconValidator()
	case NewNode:
		node.setupForNewNode()
	}
}

// RunServices runs registered services.
func (node *Node) RunServices() {
	if node.serviceManager == nil {
		utils.GetLogInstance().Info("Service manager is not set up yet.")
		return
	}
	node.serviceManager.RunServices()
}
