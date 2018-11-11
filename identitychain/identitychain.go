package identitychain

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/node"
)

var mutex sync.Mutex
var identityPerBlock = 100000

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	//Identities            []*IdentityBlock //No need to have the identity block as of now
	Identities           []*node.Node
	log                  log.Logger
	PeerToShardMap       map[*node.Node]int
	ShardLeaderMap       map[int]*node.Node
	PubKey               string
	NumberOfShards       int
	NumberOfLeadersAdded int
}

func (IDC *IdentityChain) CreateShardAssignment() {
	num := seekRandomNumber(IDC.EpochNum, IDC.SelectedIdentitites)
	IDC.NumberOfShards = IDC.NumberOfShards + needNewShards()
	IDC.generateRandomPermutations(num)
	IDC.PeerToShardMap = make(map[*node.Node]int)
	numberInOneShard := len(IDC.SelectedIdentitites) / IDC.NumberOfShards
	fmt.Println(len(IDC.SelectedIdentitites))
	for peerNum := 0; peerNum < len(IDC.SelectedIdentitites); peerNum++ {
		IDC.PeerToShardMap[IDC.SelectedIdentitites[peerNum]] = peerNum / numberInOneShard
	}
}

func (IDC *IdentityChain) generateRandomPermutations(num int) {
	src := rand.NewSource(int64(num))
	rnd := rand.New(src)
	perm := rnd.Perm(len(IDC.SelectedIdentitites))
	SelectedIdentititesCopy := make([]*node.Node, len(IDC.SelectedIdentitites))
	for j, i := range perm {
		SelectedIdentititesCopy[j] = IDC.SelectedIdentitites[i]
	}
	IDC.SelectedIdentitites = SelectedIdentititesCopy
}

// SelectIds as
func (IDC *IdentityChain) SelectIds() {
	// selectNumber := IDC.NumberOfNodesInShard - len(IDC.Identities)
	// Insert the lines below once you have a identity block
	// IB := IDC.GetLatestBlock()
	// currentIDS := IB.GetIdentities()
	// currentIDS := IDC.Identities
	// selectNumber = int(math.Min(float64(len(IDC.PendingIdentities)), float64(selectNumber)))
	// pending := IDC.PendingIdentities[:selectNumber]
	// IDC.SelectedIdentitites = append(currentIDS, pending...)
	// IDC.PendingIdentities = []*node.Node{}

}

//AcceptConnections welcomes new connections
func (IDC *IdentityChain) AcceptConnections() {
	registerNode()
}

func (IDC *IdentityChain) registerNode() {

}

//StartServer a server and process the request by a handler.
func (IDC *IdentityChain) StartServer() {
	fmt.Println("Starting server...")
	IDC.log.Info("Starting IDC server...") //log.Info does nothing for me! (ak)
	IDC.listenOnPort()
}

func (IDC *IdentityChain) listenOnPort() {
	addr := net.JoinHostPort("", IDC.Peer.Port)
	listen, err := net.Listen("tcp4", addr)
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
