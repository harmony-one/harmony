package newnode

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"sync"
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
	quit        chan bool
	SetInfo     bool
	waitGroup   *sync.WaitGroup
}

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*NewNode
}

func (node NewNode) String() string {
	return fmt.Sprintf("idc: %v:%v and node infi %v", node.Self.Ip, node.Self.Port, node.SetInfo)
}

func New(ip string, port string) *NewNode {
	pubKey, priKey := utils.GenKey(ip, port)
	var node NewNode
	node.pubK = pubKey
	node.priK = priKey
	node.Self = p2p.Peer{Ip: ip, Port: port}
	node.peers = make([]p2p.Peer, 0)
	node.log = log.New()
	node.SetInfo = false
	node.quit = make(chan bool)

	return &node
}

//ConnectIdentityChain connects to identity chain
func (node *NewNode) ConnectBeaconChain(BCPeer p2p.Peer) {
	time.Sleep(5 * time.Second)
	msg := node.SerializeNode()
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	p2p.SendMessage(BCPeer, msgToSend)
}

//ProcessShardInfo
func (node *NewNode) processShardInfo(msgPayload []byte) bool {

	leadersInfo := DeserializeRandomInfo(msgPayload)
	leaders := leadersInfo.Leaders
	shardNum, isLeader := utils.AllocateShard(leadersInfo.NumberOfNodesAdded, leadersInfo.NumberOfShards)
	leaderNode := leaders[shardNum-1] //0 indexing.
	if isLeader {
		node.leader = leaderNode.Self
	} else {
		node.leader = leaderNode.Self
	}
	node.SetInfo = true
	fmt.Println("processing shard info")
	return true
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
	node.log.Info("Starting Server ..")

	backoff := p2p.NewExpBackoff(250*time.Millisecond, 15*time.Second, 2.0)
	for {
		p := <-node.quit
		fmt.Println(p)
		//fmt.Println("node quit value in start server", <-node.quit)
		//time.Sleep(1 * time.Second)
		//Here I am trying to comment and uncomment above line
		// commenting and uncommenting this line is difference between correct and incorrect execution
		//comment is correct execution

		select {
		case <-node.quit:
			node.log.Info("Going to close listener")
			fmt.Println("closing listener")
			listen.Close()
			return
		default:
			node.log.Info("going to OPEN listener")
		}
		conn, err := listen.Accept()
		if err != nil {

			fmt.Println("Error listening on port.", "port", port,
				"err", err)
			backoff.Sleep()
			continue
		}
		fmt.Println("Handling Connections", err, conn)
		go node.NodeHandler(conn)
	}
}

func (node *NewNode) StopServer() {
	node.log.Info("server stopping")
	close(node.quit)
	node.log.Info("server stopped")
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
