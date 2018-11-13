package identitychain

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
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
	PubKey               kyber.Point
	NumberOfShards       int
	NumberOfLeadersAdded int
}

func (IDC *IdentityChain) CreateShardAssignment() {
}

func (IDC *IdentityChain) generateRandomPermutations(num int) {
}

//Init
func New(filename string) {
	idc := IdentityChain{}
	idc.NumberOfShards = readConfigFile(filename)
	idc.PubKey = generateIDCKeys()
	idc.StartServer()
}

func readConfigFile(filename string) int {
	return 2
}

func generateIDCKeys() kyber.Point {
	priKey := pki.GetPrivateKeyFromInt(10)
	pubkey := pki.GetPublicKeyFromPrivateKey(priKey)
	return pubkey
}

//AcceptConnections welcomes new connections
func (IDC *IdentityChain) AcceptConnections() {
	registerNode()
}

func registerNode() {
	return
}

//StartServer a server and process the request by a handler.
func (IDC *IdentityChain) StartServer() {
	fmt.Println("Starting server...")
	IDC.log.Info("Starting IDC server...") //log.Info does nothing for me! (ak)
	IDC.listenOnPort()
}

func (IDC *IdentityChain) listenOnPort() {
	addr := net.JoinHostPort("", "8081")
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
			IDC.log.Crit("Error listening on port. Exiting", "8081")
			continue
		} else {
			fmt.Println("I am accepting connections now")
		}
		go IDC.IdentityChainHandler(conn)
	}
}
