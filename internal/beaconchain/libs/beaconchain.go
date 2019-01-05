package beaconchain

import (
	"math/rand"
	"strconv"
	"sync"

	"github.com/dedis/kyber"

	"github.com/harmony-one/harmony/api/proto/bcconn"
	proto_identity "github.com/harmony-one/harmony/api/proto/identity"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/beaconchain/rpc"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

type BCState int

var mutex sync.Mutex
var identityPerBlock = 100000

// BeaconchainServicePortDiff is the positive port diff from beacon chain's self port
const BeaconchainServicePortDiff = 4444

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
	host               host.Host
	state              BCState
	rpcServer          *beaconchain.Server
}

// Followings are the set of states of that beaconchain can be in.
const (
	NodeInfoReceived BCState = iota
	RandomInfoSent
)

// SupportRPC initializes and starts the rpc service
func (bc *BeaconChain) SupportRPC() {
	bc.InitRPCServer()
	bc.StartRPCServer()
}

// InitRPCServer initializes Rpc server.
func (bc *BeaconChain) InitRPCServer() {
	bc.rpcServer = beaconchain.NewServer(bc.GetShardLeaderMap)
}

// StartRPCServer starts Rpc server.
func (bc *BeaconChain) StartRPCServer() {
	port, err := strconv.Atoi(bc.Port)
	if err != nil {
		port = 0
	}
	bc.log.Info("support_client: StartRpcServer on port:", "port", strconv.Itoa(port+BeaconchainServicePortDiff))
	bc.rpcServer.Start(bc.IP, strconv.Itoa(port+BeaconchainServicePortDiff))
}

// GetShardLeaderMap returns the map from shard id to leader.
func (bc *BeaconChain) GetShardLeaderMap() map[int]*bcconn.NodeInfo {
	result := make(map[int]*bcconn.NodeInfo)
	for i, leader := range bc.Leaders {
		result[i] = leader
	}
	return result
}

//New beaconchain initialization
func New(numShards int, ip, port string) *BeaconChain {
	bc := BeaconChain{}
	bc.log = log.New()
	bc.NumberOfShards = numShards
	bc.PubKey = generateBCKey()
	bc.NumberOfNodesAdded = 0
	bc.ShardLeaderMap = make(map[int]*bcconn.NodeInfo)
	bc.Port = port
	bc.IP = ip
	bc.host = p2pimpl.NewHost(p2p.Peer{IP: ip, Port: port})
	return &bc
}

func generateBCKey() kyber.Point {
	r := rand.Intn(1000)
	priKey := pki.GetPrivateKeyFromInt(r)
	pubkey := pki.GetPublicKeyFromPrivateKey(priKey)
	return pubkey
}

//AcceptNodeInfo deserializes node information received via beaconchain handler
func (bc *BeaconChain) AcceptNodeInfo(b []byte) *bcconn.NodeInfo {
	Node := bcconn.DeserializeNodeInfo(b)
	bc.log.Info("New Node Connection", "IP", Node.Self.IP, "Port", Node.Self.Port)
	bc.NumberOfNodesAdded = bc.NumberOfNodesAdded + 1
	_, isLeader := utils.AllocateShard(bc.NumberOfNodesAdded, bc.NumberOfShards)
	if isLeader {
		bc.Leaders = append(bc.Leaders, Node)
	}
	bc.state = NodeInfoReceived
	return Node
}

//RespondRandomness sends a randomness beacon to the node inorder for it process what shard it will be in
func (bc *BeaconChain) RespondRandomness(Node *bcconn.NodeInfo) {
	response := bcconn.ResponseRandomNumber{NumberOfShards: bc.NumberOfShards, NumberOfNodesAdded: bc.NumberOfNodesAdded, Leaders: bc.Leaders}
	msg := bcconn.SerializeRandomInfo(response)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Acknowledge, msg)
	bc.log.Info("Sent Out Msg", "# Nodes", response.NumberOfNodesAdded)
	host.SendMessage(bc.host, Node.Self, msgToSend, nil)
	bc.state = RandomInfoSent
}

//AcceptConnections welcomes new connections
func (bc *BeaconChain) AcceptConnections(b []byte) {
	node := bc.AcceptNodeInfo(b)
	bc.RespondRandomness(node)
}

//StartServer a server and process the request by a handler.
func (bc *BeaconChain) StartServer() {
	bc.host.BindHandlerAndServe(bc.BeaconChainHandler)
}
