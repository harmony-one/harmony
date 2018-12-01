package beaconchain

import (
	"math/rand"
	"net"
	"os"
	"sync"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/bcconn"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	"github.com/harmony-one/harmony/utils"
)

var mutex sync.Mutex
var identityPerBlock = 100000

// BeaconChain (Blockchain) keeps Identities per epoch, currently centralized!
type BeaconChain struct {
	Leaders            []*bcconn.NodeInfo
	log                log.Logger
	ShardLeaderMap     map[int]*bcconn.NodeInfo
	PubKey             kyber.Point
	NumberOfShards     int
	NumberOfNodesAdded int
	IP                 string
	Port               string
}

//New beaconchain initialization
func New(numShards int, ip, port string) *BeaconChain {
	bc := BeaconChain{}
	bc.log = log.New()
	bc.NumberOfShards = numShards
	bc.PubKey = generateIDCKeys()
	bc.NumberOfNodesAdded = 0
	bc.Port = port
	bc.IP = ip
	return &bc
}

func generateIDCKeys() kyber.Point {
	r := rand.Intn(1000)
	priKey := pki.GetPrivateKeyFromInt(r)
	pubkey := pki.GetPublicKeyFromPrivateKey(priKey)
	return pubkey
}

//AcceptConnections welcomes new connections
func (bc *BeaconChain) AcceptConnections(b []byte) {
	Node := bcconn.DeserializeNodeInfo(b)
	bc.log.Info("Obtained node information, updating local information")
	bc.NumberOfNodesAdded = bc.NumberOfNodesAdded + 1
	_, isLeader := utils.AllocateShard(bc.NumberOfNodesAdded, bc.NumberOfShards)
	if isLeader {
		bc.Leaders = append(bc.Leaders, Node)
	}
	response := bcconn.ResponseRandomNumber{NumberOfShards: bc.NumberOfShards, NumberOfNodesAdded: bc.NumberOfNodesAdded, Leaders: bc.Leaders}
	msg := bcconn.SerializeRandomInfo(response)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Acknowledge, msg)
	p2p.SendMessage(Node.Self, msgToSend)
}

//StartServer a server and process the request by a handler.
func (bc *BeaconChain) StartServer() {
	bc.log.Info("Starting Beaconchain server ...")
	ip := bc.IP
	port := bc.Port
	addr := net.JoinHostPort(ip, port)
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		bc.log.Crit("Socket listen port failed")
		os.Exit(1)
	}
	defer listen.Close()
	for {
		bc.log.Info("beacon chain is now listening ..")
		conn, err := listen.Accept()
		if err != nil {
			bc.log.Crit("Error listening on port. Exiting", "8081")
			continue
		}
		go bc.BeaconChainHandler(conn)
	}
}
