package beaconchain

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
	proto_identity "github.com/simple-rules/harmony-benchmark/proto/identity"
)

var mutex sync.Mutex
var identityPerBlock = 100000

// BeaconChain (Blockchain) keeps Identities per epoch, currently centralized!
type BeaconChain struct {
	//Identities            []*IdentityBlock //No need to have the identity block as of now
	Identities           []*node.Node
	log                  log.Logger
	PeerToShardMap       map[*node.Node]int
	ShardLeaderMap       map[int]*node.Node
	PubKey               kyber.Point
	NumberOfShards       int
	NumberOfLeadersAdded int
}

//Init
func Init(filename string) {
	idc := BeaconChain{}
	//idc.NumberOfShards = readConfigFile(filename)
	idc.NumberOfShards = 2
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
func (IDC *BeaconChain) AcceptConnections(b []byte) {
	Node := node.DeserializeWaitNode(b)
	IDC.registerNode(Node) //This copies lock value of sync.mutex, we need to have a way around it by creating auxiliary data struct.
}

func (IDC *BeaconChain) registerNode(Node *node.Node) {
	IDC.Identities = append(IDC.Identities, Node)
	IDC.CommunicatePublicKeyToNode(Node.Self)
	return
}

func (IDC *BeaconChain) CommunicatePublicKeyToNode(Peer p2p.Peer) {
	pbkey := pki.GetBytesFromPublicKey(IDC.PubKey)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Acknowledge, pbkey[:])
	p2p.SendMessage(Peer, msgToSend)
}

//StartServer a server and process the request by a handler.
func (IDC *BeaconChain) StartServer() {
	fmt.Println("Starting server...")
	fmt.Println(IDC.PubKey)
	IDC.log.Info("Starting IDC server...") //log.Info does nothing for me! (ak)
	IDC.listenOnPort()
}

func (IDC *BeaconChain) listenOnPort() {
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
		go IDC.BeaconChainHandler(conn)
	}
}
