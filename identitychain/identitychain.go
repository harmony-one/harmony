package identitychain

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

var mutex sync.Mutex
var identityPerBlock = 100000

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities            []*IdentityBlock
	PendingIdentities     []*node.Node
	log                   log.Logger
	Peer                  p2p.Peer
	SelectedIdentitites   []*node.Node
	EpochNum              int
	PeerToShardMap        map[*node.Node]int
	ShardLeaderMap        map[int]*node.Node
	PubKey                string
	CurrentEpochStartTime int64
	NumberOfShards        int
	NumberOfNodesInShard  int
	PowMap                map[p2p.Peer]uint32
}

func seekRandomNumber(EpochNum int, SelectedIdentitites []*node.Node) int {
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
	IDC.CreateShardAssignment()
	IDC.ElectLeaders()
	IDC.BroadCastNewConfiguration()
}

//
func (IDC *IdentityChain) ElectLeaders() {
}

func (IDC *IdentityChain) BroadCastNewConfiguration() {
	// allPeers := make([]p2p.Peer, len(IDC.SelectedIdentitites))
	// msgToSend := proto.
	// 	p2p.BroadCastMessage(allPeers, msgToSend)

}

//CreateShardAssignment
func (IDC *IdentityChain) CreateShardAssignment() {
	num := seekRandomNumber(IDC.EpochNum, IDC.SelectedIdentitites)
	IDC.NumberOfShards = IDC.NumberOfShards + needNewShards()
	IDC.SelectedIdentitites = generateRandomPermutations(num, IDC.SelectedIdentitites)
	IDC.PeerToShardMap = make(map[*node.Node]int)
	numberInOneShard := len(IDC.SelectedIdentitites) / IDC.NumberOfShards
	for peerNum := 1; peerNum <= len(IDC.SelectedIdentitites); peerNum++ {
		IDC.PeerToShardMap[IDC.SelectedIdentitites[peerNum]] = peerNum / numberInOneShard
	}
}

func generateRandomPermutations(num int, SelectedIdentitites []*node.Node) []*node.Node {
	src := rand.NewSource(int64(num))
	rnd := rand.New(src)
	perm := rnd.Perm(len(SelectedIdentitites))
	SelectedIdentititesCopy := make([]*node.Node, len(SelectedIdentitites))
	for j, i := range perm {
		SelectedIdentititesCopy[j] = SelectedIdentitites[i]
	}
	return SelectedIdentititesCopy
}

// SelectIds as
func (IDC *IdentityChain) SelectIds() {
	selectNumber := IDC.NumberOfNodesInShard - len(IDC.Identities)
	IB := IDC.GetLatestBlock()
	currentIDS := IB.GetIdentities()
	selectNumber = int(math.Min(float64(len(IDC.PendingIdentities)), float64(selectNumber)))
	pending := IDC.PendingIdentities[:selectNumber]
	IDC.SelectedIdentitites = append(currentIDS, pending...)
	IDC.PendingIdentities = []*node.Node{}
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
		IDC.PendingIdentities = []*node.Node{}
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
	IDC.PowMap = make(map[p2p.Peer]uint32)
	return &IDC
}
