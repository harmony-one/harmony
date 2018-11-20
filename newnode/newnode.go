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
	Role        string
	ShardID     string
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	Self        p2p.Peer
	peers       []p2p.Peer
	pubK        kyber.Scalar
	priK        kyber.Point
}

func (node NewNode) String() string {
	return fmt.Sprintf("idc: %v:%v", node.Self.Ip, node.Self.Port)
}

func New(ip string, port string) *NewNode {
	pubKey, priKey := utils.GenKey(ip, port)
	var node NewNode
	node.pubK = pubKey
	node.priK = priKey
	node.Self = p2p.Peer{Ip: ip, Port: port}
	node.peers = make([]p2p.Peer, 0)

	return &node
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
func (node *NewNode) StartClientMode(idcIP, idcPort string) error {
	fmt.Printf("idc ip/port: %v/%v\n", idcIP, idcPort)

	// ...
	// TODO: connect to idc, and wait unless acknowledge
	return nil
}

func (node *NewNode) GetShardID() string {
	return node.ShardID
}

func (node *NewNode) GetPeers() []p2p.Peer {
	return node.peers
}

func (node *NewNode) GetLeader() p2p.Peer {
	return node.leader
}

func (node *NewNode) GetSelfPeer() p2p.Peer {
	return node.Self
}
