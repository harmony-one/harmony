package beaconchain

import (
	"github.com/harmony-one/harmony/beaconchain/rpc"
	"math/rand"
	"strconv"
	"sync"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"github.com/harmony-one/harmony/proto/bcconn"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	"github.com/harmony-one/harmony/utils"
)

var mutex sync.Mutex
var identityPerBlock = 100000

<<<<<<< HEAD
//BeaconChainInfo is to
type BeaconChainInfo struct {
	Leaders            []*bcconn.NodeInfo
	ShardLeaderMap     map[int]*bcconn.NodeInfo
	NumberOfShards     int
	NumberOfNodesAdded int
	IP                 string
	Port               string
}
=======
// BeaconchainServicePortDiff is the positive port diff from beacon chain's self port
const BeaconchainServicePortDiff = 4444
>>>>>>> a8b8ba05e34f0d3274cf897d3f4ef8fec79d2dfb

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

	rpcServer *beaconchain.Server
}

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
	SaveBeaconChainInfo("bc.json", bc)
	msg := bcconn.SerializeRandomInfo(response)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Acknowledge, msg)
	host.SendMessage(bc.host, Node.Self, msgToSend, nil)
}

//StartServer a server and process the request by a handler.
func (bc *BeaconChain) StartServer() {
	bc.host.BindHandlerAndServe(bc.BeaconChainHandler)
}

//SavesBeaconChainInfo to disk
func SaveBeaconChainInfo(path string, bc *BeaconChain) error {
	bci := BCtoBCI(bc)
	err := utils.Save(path, bci)
	return err
}

//LoadBeaconChainInfo from disk
func LoadBeaconChainInfo(path string) (*BeaconChain, error) {
	bci := &BeaconChainInfo{}
	err := utils.Load(path, bci)
	bc := BCItoBC(bci)
	return bc, err
}

//BCtoBCI converts beaconchain into beaconchaininfo
func BCtoBCI(bc *BeaconChain) *BeaconChainInfo {
	bci := &BeaconChainInfo{Leaders: bc.Leaders, ShardLeaderMap: bc.ShardLeaderMap, NumberOfShards: bc.NumberOfShards, NumberOfNodesAdded: bc.NumberOfNodesAdded, IP: bc.IP, Port: bc.Port}
	return bci
}

//BCItoBC
func BCItoBC(bci *BeaconChainInfo) *BeaconChain {
	bc := &BeaconChain{Leaders: bci.Leaders, ShardLeaderMap: bci.ShardLeaderMap, NumberOfShards: bci.NumberOfShards, NumberOfNodesAdded: bci.NumberOfNodesAdded, IP: bci.IP, Port: bci.Port}
	return bc
}
