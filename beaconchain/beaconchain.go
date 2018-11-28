package beaconchain

import (
	"bytes"
	"encoding/gob"
	"net"
	"os"
	"sync"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/newnode"
	"github.com/harmony-one/harmony/p2p"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	"github.com/harmony-one/harmony/utils"
)

var mutex sync.Mutex
var identityPerBlock = 100000

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*newnode.NodeInfo
}

// BeaconChain (Blockchain) keeps Identities per epoch, currently centralized!
type BeaconChain struct {
	Leaders            []*newnode.NodeInfo
	log                log.Logger
	ShardLeaderMap     map[int]*newnode.NodeInfo
	PubKey             kyber.Point
	NumberOfShards     int
	NumberOfNodesAdded int
}

//Init
func New(filename string) *BeaconChain {
	bc := BeaconChain{}
	//bc.NumberOfShards = readConfigFile(filename)
	bc.log = log.New()
	bc.NumberOfShards = 2 //hardcode.
	bc.PubKey = generateIDCKeys()
	bc.NumberOfNodesAdded = 0
	return &bc
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
func (bc *BeaconChain) AcceptConnections(b []byte) {
	NewNodeInfo := newnode.DeserializeNodeInfo(b)
	bc.registerNode(NewNodeInfo)
}

func (bc *BeaconChain) registerNode(Node *newnode.NodeInfo) {
	bc.NumberOfNodesAdded = bc.NumberOfNodesAdded + 1
	_, isLeader := utils.AllocateShard(bc.NumberOfNodesAdded, bc.NumberOfShards)
	if isLeader {
		bc.Leaders = append(bc.Leaders, Node)
	}
	/**

	**IMPORTANT**

	Note that public key is not in kyber.Scalar form it is in byte form.
	Use following conversion to get back the actual key
	//peer.PubKey = crypto.Ed25519Curve.Point()
	//err = peer.PubKey.UnmarshalBinary(p.PubKey[:])

	**/
	response := registerResponseRandomNumber{NumberOfShards: bc.NumberOfShards, NumberOfNodesAdded: bc.NumberOfNodesAdded, Leaders: bc.Leaders}
	msg := bc.SerializeRandomInfo(response)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Acknowledge, msg)
	p2p.SendMessage(Node.Self, msgToSend)
}

//SerializeNode
func (bc *BeaconChain) SerializeRandomInfo(response registerResponseRandomNumber) []byte {
	//Needs to escape the serialization of unexported fields
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(response)
	if err != nil {
		bc.log.Crit("ERROR", err)
		//node.log.Error("Could not serialize node")
	}

	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeRandomInfo(d []byte) *registerResponseRandomNumber {
	var wn registerResponseRandomNumber
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Crit("Could not de-serialize random number information")
	}
	return &wn
}

//StartServer a server and process the request by a handler.
func (bc *BeaconChain) StartServer() {
	bc.log.Info("Starting Beaconchain server ...")
	addr := net.JoinHostPort("127.0.0.1", "8081")
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
