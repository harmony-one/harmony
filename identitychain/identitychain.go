package identitychain

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

var mutex sync.Mutex
var identityPerBlock = 100000

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities            []*IdentityBlock
	PendingIdentities     []*waitnode.WaitNode
	log                   log.Logger
	Peer                  p2p.Peer
	SelectedIdentitites   []*waitnode.WaitNode
	EpochNum              int
	PeerToShardMap        map[*waitnode.WaitNode]int
	ShardLeaderMap        map[int]*waitnode.WaitNode
	PubKey                string
	CurrentEpochStartTime int64
	NumberOfShards        int
	NumberOfNodesInShard  int
}

func seekRandomNumber(EpochNum int, SelectedIdentitites []*waitnode.WaitNode) int {
	// Broadcast message to all nodes and collect back their numbers, do consensus and get a leader.
	// Use leader to generate a random number.
	//all here mocked
	// interact with "node" and "wait_node"
	return rand.Intn(1000)

}

//GlobalBlockchainConfig stores global level blockchain configurations.
type GlobalBlockchainConfig struct {
	NumberOfShards  int
	EpochTimeSecs   int16
	MaxNodesInShard int
}

//Shard
func (IDC *IdentityChain) Shard() {
	num := seekRandomNumber(IDC.EpochNum, IDC.SelectedIdentitites)
	IDC.CreateShardAssignment(num)
	IDC.ElectLeaders()
}

//
func (IDC *IdentityChain) ElectLeaders() {
}

//CreateShardAssignment
func (IDC *IdentityChain) CreateShardAssignment(num int) {
	IDC.NumberOfShards = IDC.NumberOfShards + needNewShards()
	IDC.SelectedIdentitites = generateRandomPermutations(num, IDC.SelectedIdentitites)
	IDC.PeerToShardMap = make(map[*waitnode.WaitNode]int)
	numberInOneShard := len(IDC.SelectedIdentitites) / IDC.NumberOfShards
	for peerNum := 1; peerNum <= len(IDC.SelectedIdentitites); peerNum++ {
		IDC.PeerToShardMap[IDC.SelectedIdentitites[peerNum]] = peerNum / numberInOneShard
	}
}

func generateRandomPermutations(num int, SelectedIdentitites []*waitnode.WaitNode) []*waitnode.WaitNode {
	src := rand.NewSource(int64(num))
	rnd := rand.New(src)
	perm := rnd.Perm(len(SelectedIdentitites))
	SelectedIdentititesCopy := make([]*waitnode.WaitNode, len(SelectedIdentitites))
	for j, i := range perm {
		SelectedIdentititesCopy[j] = SelectedIdentitites[i]
	}
	return SelectedIdentititesCopy
}

// SelectIds
func (IDC *IdentityChain) SelectIds() {
	selectNumber := IDC.NumberOfNodesInShard - len(IDC.Identities)
	IB := IDC.GetLatestBlock()
	currentIDS := IB.GetIdentities()
	selectNumber = int(math.Min(float64(len(IDC.PendingIdentities)), float64(selectNumber)))
	pending := IDC.PendingIdentities[:selectNumber]
	IDC.SelectedIdentitites = append(currentIDS, pending...)
	IDC.PendingIdentities = []*waitnode.WaitNode{}
}

//Checks how many new shards we need. Currently we say 0.
func needNewShards() int {
	return 0
}

// GetLatestBlock gests the latest block at the end of the chain
func (IDC *IdentityChain) GetLatestBlock() *IdentityBlock {
	if len(IDC.Identities) == 0 {
		return nil
	}
	return IDC.Identities[len(IDC.Identities)-1]
}

//UpdateIdentityChain is to create the Blocks to be added to the chain
func (IDC *IdentityChain) UpdateIdentityChain() {

	//If there are no more Identities registring the blockchain is dead
	if len(IDC.PendingIdentities) == 0 {
		// This is abd, because previous block might not be alive
		return
	}
	if len(IDC.Identities) == 0 {
		block := NewGenesisBlock()
		IDC.Identities = append(IDC.Identities, block)
	} else {
		prevBlock := IDC.GetLatestBlock()
		prevBlockHash := prevBlock.CalculateBlockHash()
		NewIdentities := IDC.PendingIdentities[:identityPerBlock]
		IDC.PendingIdentities = []*waitnode.WaitNode{}
		//All other blocks are dropped, we need to inform them that they are dropped?
		IDBlock := NewBlock(NewIdentities, prevBlockHash)
		IDC.Identities = append(IDC.Identities, IDBlock)
	}

}

//StartServer a server and process the request by a handler.
func (IDC *IdentityChain) StartServer() {
	fmt.Println("Starting server...")
	IDC.log.Info("Starting IDC server...") //log.Info does nothing for me! (ak)
	IDC.listenOnPort()
}

func (IDC *IdentityChain) listenOnPort() {
	listen, err := net.Listen("tcp4", ":"+IDC.Peer.Port)
	if err != nil {
		IDC.log.Crit("Socket listen port failed")
		os.Exit(1)
	} else {
		fmt.Println("Starting server...now listening")
		IDC.log.Info("Identity chain is now listening ..") //log.Info does nothing for me! (ak) remove this
	}
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			IDC.log.Crit("Error listening on port. Exiting", IDC.Peer.Port)
			continue
		} else {
			fmt.Println("I am accepting connections now")
		}
		go IDC.IdentityChainHandler(conn)
	}
}

// New Create a new Node
func New(Peer p2p.Peer) *IdentityChain {
	IDC := IdentityChain{}
	IDC.Peer = Peer
	IDC.log = log.New()
	return &IDC
}
