package beaconchain

import (
	"math/rand"
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

//BeaconChainInfo is to
type BeaconChainInfo struct {
	Leaders            []*bcconn.NodeInfo
	ShardLeaderMap     map[int]*bcconn.NodeInfo
	NumberOfShards     int
	NumberOfNodesAdded int
	IP                 string
	Port               string
}

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
}

//New beaconchain initialization
func New(numShards int, ip, port string) *BeaconChain {
	bc := BeaconChain{}
	bc.log = log.New()
	bc.NumberOfShards = numShards
	bc.PubKey = generateBCKey()
	bc.NumberOfNodesAdded = 0
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
