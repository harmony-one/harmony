package newnode

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	"github.com/harmony-one/harmony/utils"
)

type NewNode struct {
	IP          string
	Port        string
	Role        string
	ShardID     string
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	self        p2p.Peer
	peers       []p2p.Peer
	pubK        kyber.Scalar
	priK        kyber.Point
}

func (config NewNode) String() string {
	return fmt.Sprintf("idc: %v:%v", config.IP, config.Port)
}

func New(ip string, port string) *NewNode {
	pubKey, priKey := utils.GenKey(ip, port)
	var config NewNode
	config.pubK = pubKey
	config.priK = priKey

	config.peers = make([]p2p.Peer, 0)

	return &config
}

//ConnectIdentityChain connects to identity chain
func (node *NewNode) ConnectBeaconChain(BCPeer p2p.Peer) {
	msg := node.SerializeNode()
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	p2p.SendMessage(BCPeer, msgToSend)
}

//SerializeNode
func (node *NewNode) SerializeNode() []byte {
	//Needs to escape the serialization of unexported fields
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(node)
	if err != nil {
		fmt.Println("Could not serialize node")
		fmt.Println("ERROR", err)
		//node.log.Error("Could not serialize node")
	}

	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeNode(d []byte) *NewNode {
	var wn NewNode
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize node 1")
	}
	return &wn
}
func (config *NewNode) StartClientMode(idcIP, idcPort string) error {
	config.IP = "myip"
	config.Port = "myport"

	fmt.Printf("idc ip/port: %v/%v\n", idcIP, idcPort)

	// ...
	// TODO: connect to idc, and wait unless acknowledge
	return nil
}

func (config *NewNode) GetShardID() string {
	return config.ShardID
}

func (config *NewNode) GetPeers() []p2p.Peer {
	return config.peers
}

func (config *NewNode) GetLeader() p2p.Peer {
	return config.leader
}

func (config *NewNode) GetSelfPeer() p2p.Peer {
	return config.self
}
