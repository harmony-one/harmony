package beaconchain

import (
	"math/rand"
	"os"
	"strconv"
	"sync"

	"github.com/dedis/kyber"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/api/proto/bcconn"
	proto_identity "github.com/harmony-one/harmony/api/proto/identity"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/beaconchain/rpc"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

//BCState keeps track of the state the beaconchain is in
type BCState int

var mutex sync.Mutex
var identityPerBlock = 100000

// BeaconchainServicePortDiff is the positive port diff from beacon chain's self port
const BeaconchainServicePortDiff = 4444

//BCInfo is the information that needs to be stored on the disk in order to allow for a restart.
type BCInfo struct {
	Leaders            []*bcconn.NodeInfo       `json:"leaders"`
	ShardLeaderMap     map[int]*bcconn.NodeInfo `json:"shardLeaderMap"`
	NumberOfShards     int                      `json:"numShards"`
	NumberOfNodesAdded int                      `json:"numNodesAdded"`
	IP                 string                   `json:"ip"`
	Port               string                   `json:"port"`
}

// BeaconChain (Blockchain) keeps Identities per epoch, currently centralized!
type BeaconChain struct {
	BCInfo         BCInfo
	log            log.Logger
	ShardLeaderMap map[int]*bcconn.NodeInfo
	PubKey         kyber.Point
	host           host.Host
	state          BCState
	rpcServer      *beaconchain.Server
}

//SaveFile is to store the file in which beaconchain info will be stored.
var SaveFile string

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
	port, err := strconv.Atoi(bc.BCInfo.Port)
	if err != nil {
		port = 0
	}
	bc.log.Info("support_client: StartRpcServer on port:", "port", strconv.Itoa(port+BeaconchainServicePortDiff))
	bc.rpcServer.Start(bc.BCInfo.IP, strconv.Itoa(port+BeaconchainServicePortDiff))
}

// GetShardLeaderMap returns the map from shard id to leader.
func (bc *BeaconChain) GetShardLeaderMap() map[int]*bcconn.NodeInfo {
	result := make(map[int]*bcconn.NodeInfo)
	for i, leader := range bc.BCInfo.Leaders {
		result[i] = leader
	}
	return result
}

//New beaconchain initialization
func New(numShards int, ip, port string) *BeaconChain {
	bc := BeaconChain{}
	bc.log = log.New()
	bc.PubKey = generateBCKey()
	bc.host = p2pimpl.NewHost(p2p.Peer{IP: ip, Port: port})
	bcinfo := &BCInfo{NumberOfShards: numShards, NumberOfNodesAdded: 0,
		IP:             ip,
		Port:           port,
		ShardLeaderMap: make(map[int]*bcconn.NodeInfo)}
	bc.BCInfo = *bcinfo
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
	bc.BCInfo.NumberOfNodesAdded = bc.BCInfo.NumberOfNodesAdded + 1
	shardNum, isLeader := utils.AllocateShard(bc.BCInfo.NumberOfNodesAdded, bc.BCInfo.NumberOfShards)
	if isLeader {
		bc.BCInfo.Leaders = append(bc.BCInfo.Leaders, Node)
		bc.BCInfo.ShardLeaderMap[shardNum] = Node
	}
	go SaveBeaconChainInfo(SaveFile, bc)
	bc.state = NodeInfoReceived
	return Node
}

//RespondRandomness sends a randomness beacon to the node inorder for it process what shard it will be in
func (bc *BeaconChain) RespondRandomness(Node *bcconn.NodeInfo) {
	bci := bc.BCInfo
	response := bcconn.ResponseRandomNumber{NumberOfShards: bci.NumberOfShards, NumberOfNodesAdded: bci.NumberOfNodesAdded, Leaders: bci.Leaders}
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

//SaveBeaconChainInfo to disk
func SaveBeaconChainInfo(filePath string, bc *BeaconChain) error {
	bci := BCtoBCI(bc)
	err := utils.Save(filePath, bci)
	return err
}

//LoadBeaconChainInfo from disk
func LoadBeaconChainInfo(path string) (*BeaconChain, error) {
	bci := &BCInfo{}
	var err error
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	err = utils.Load(path, bci)
	var bc *BeaconChain
	if err != nil {
		return nil, err
	}
	bc = BCItoBC(bci)
	return bc, err
}

// BCtoBCI converts beaconchain into beaconchaininfo
func BCtoBCI(bc *BeaconChain) *BCInfo {
	bci := &BCInfo{Leaders: bc.BCInfo.Leaders, ShardLeaderMap: bc.BCInfo.ShardLeaderMap, NumberOfShards: bc.BCInfo.NumberOfShards, NumberOfNodesAdded: bc.BCInfo.NumberOfNodesAdded, IP: bc.BCInfo.IP, Port: bc.BCInfo.Port}
	return bci
}

//BCItoBC converts beconchaininfo to beaconchain
func BCItoBC(bci *BCInfo) *BeaconChain {
	bc := &BeaconChain{BCInfo: *bci}
	return bc
}

//SetSaveFile sets the filepath where beaconchain will be saved
func SetSaveFile(path string) {
	SaveFile = path
}
