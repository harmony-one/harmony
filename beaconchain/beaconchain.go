package beaconchain

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/newnode"
	"github.com/harmony-one/harmony/node"
)

var mutex sync.Mutex
var identityPerBlock = 100000

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*newnode.NewNode
}

// BeaconChain (Blockchain) keeps Identities per epoch, currently centralized!
type BeaconChain struct {
	//Identities            []*IdentityBlock //No need to have the identity block as of now
	Leaders            []*newnode.NewNode
	log                log.Logger
	PeerToShardMap     map[*node.Node]int
	ShardLeaderMap     map[int]*node.Node
	PubKey             kyber.Point
	NumberOfShards     int
	NumberOfNodesAdded int
}

//Init
func New(filename string) *BeaconChain {
	idc := BeaconChain{}
	//idc.NumberOfShards = readConfigFile(filename)
	idc.log = log.New()
	idc.NumberOfShards = 2 //hardcode.
	idc.PubKey = generateIDCKeys()
	idc.NumberOfNodesAdded = 0
	return &idc
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
func (IDC *BeaconChain) AcceptConnections(b []byte) {
	NewNode := newnode.DeserializeNode(b)
	IDC.registerNode(NewNode)
}

func (IDC *BeaconChain) registerNode(Node *newnode.NewNode) {
	IDC.NumberOfNodesAdded = IDC.NumberOfNodesAdded + 1
	if IDC.NumberOfNodesAdded <= IDC.NumberOfShards {
		IDC.Leaders = append(IDC.Leaders, Node)
	}
	response := registerResponseRandomNumber{NumberOfShards: IDC.NumberOfShards, NumberOfNodesAdded: IDC.NumberOfNodesAdded, Leaders: IDC.Leaders}
	fmt.Println(response)
	// msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Acknowledge, response)
	// p2p.SendMessage(Node.Self, msgToSend)
	// return
}

//StartServer a server and process the request by a handler.
func (IDC *BeaconChain) StartServer() {
	IDC.log.Info("Starting IDC server...") //log.Info does nothing for me! (ak)
	IDC.listenOnPort()
}

func (IDC *BeaconChain) listenOnPort() {
	addr := net.JoinHostPort("127.0.0.1", "8081")
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		IDC.log.Crit("Socket listen port failed")
		os.Exit(1)
	} else {
		IDC.log.Info("Identity chain is now listening ..")
	}
	defer listen.Close()
	for {
		IDC.log.Info("I am accepting connections now")
		conn, err := listen.Accept()
		fmt.Println(conn)
		if err != nil {
			IDC.log.Crit("Error listening on port. Exiting", "8081")
			continue
		} else {
			IDC.log.Info("I am accepting connections now")
		}
		go IDC.BeaconChainHandler(conn)
	}

}
