package newnode

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"time"

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
	log         log.Logger
}

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*NewNode
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
	node.log = log.New()

	return &node
}

//ConnectIdentityChain connects to identity chain
func (node *NewNode) ConnectBeaconChain(BCPeer p2p.Peer) {
	msg := node.SerializeNode()
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	p2p.SendMessage(BCPeer, msgToSend)
}

//ProcessShardInfo
func (node *NewNode) processShardInfo(msgPayload []byte) {
	leadersInfo := DeserializeRandomInfo(msgPayload)
	leaders := leadersInfo.Leaders
	shardNum, isLeader := utils.AllocateShard(leadersInfo.NumberOfNodesAdded, leadersInfo.NumberOfShards)
	leaderNode := leaders[shardNum-1] //0 indexing.
	if isLeader {
		node.leader = leaderNode.Self
	}
	fmt.Println(node.leader)
	//todo set the channel to tell to exit server
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
func DeserializeRandomInfo(d []byte) *registerResponseRandomNumber {
	var wn registerResponseRandomNumber
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize node 1")
	}
	return &wn
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

// StartServer starts a server and process the request by a handler.
func (node *NewNode) StartServer() {
	fmt.Println("going to start server")
	//node.log.Debug("Starting server", "node", node, "port", port)

	node.listenOnPort()
}

func (node *NewNode) listenOnPort() {
	port := node.Self.Port
	addr := net.JoinHostPort("", port)
	listen, err := net.Listen("tcp4", addr)
	if err != nil {
		fmt.Println("Socket listen port failed", "addr", addr, "err", err)
		return
	}
	if listen == nil {
		fmt.Println("Listen returned nil", "addr", addr)
		return
	}
	defer listen.Close()
	backoff := p2p.NewExpBackoff(250*time.Millisecond, 15*time.Second, 2.0)
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("Error listening on port.", "port", port,
				"err", err)
			backoff.Sleep()
			continue
		}
		go node.NodeHandler(conn)
	}
}

//ConnectIdentityChain connects to identity chain
func (node *NewNode) ConnectBC(BCPeer p2p.Peer) {
	//todo add error handling and backoff for not connecting.
	time.Sleep(5 * time.Second)
	node.ConnectBeaconChain(BCPeer)
	time.Sleep(5 * time.Second)
}

// func (node *NewNode) StartClientMode() error {
// 	fmt.Printf("idc ip/port: %v/%v\n", idcIP, idcPort)

// 	// ...
// 	// TODO: connect to idc, and wait unless acknowledge
// 	return nil
// }

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
