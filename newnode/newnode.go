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

// An uninteresting service.
type Service struct {
	ch        chan bool
	waitGroup *sync.WaitGroup
}

// Make a new Service.
func NewService(ip, port string) *Service {
	// Listen on 127.0.0.1:48879.  That's my favorite port number because in
	// hex 48879 is 0xBEEF.
	laddr, err := net.ResolveTCPAddr("tcp", ip+":"+port)
	if nil != err {
		fmt.Println(err)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if nil != err {
		fmt.Println(err)
	}
	fmt.Println("listening on", listener.Addr())

	s := &Service{
		ch:        make(chan bool),
		waitGroup: &sync.WaitGroup{},
	}
	s.waitGroup.Add(1)
	go s.Serve(listener)
	return s
}

// Accept connections and spawn a goroutine to serve each one.  Stop listening
// if anything is received on the service's channel.
func (s *Service) Serve(listener *net.TCPListener) {
	defer s.waitGroup.Done()
	for {
		select {
		case <-s.ch:
			fmt.Println("stopping listening on", listener.Addr())
			listener.Close()
			return
		default:
		}
		listener.SetDeadline(time.Now().Add(1e9))
		conn, err := listener.AcceptTCP()
		if nil != err {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			fmt.Println(err)

		}
		fmt.Println(conn.RemoteAddr(), "connected")
		s.waitGroup.Add(1)
		go s.serve(conn)
	}
}

// Stop the service by closing the service's channel.  Block until the service
// is really stopped.
func (s *Service) Stop() {
	close(s.ch)
	s.waitGroup.Wait()
}

// Serve a connection by reading and writing what was read.  That's right, this
// is an echo service.  Stop reading and writing if anything is received on the
// service's channel but only after writing what was read.
func (s *Service) serve(conn *net.TCPConn) {
	defer conn.Close()
	defer s.waitGroup.Done()
	for {
		select {
		case <-s.ch:
			fmt.Println("disconnecting", conn.RemoteAddr())
			return
		default:
		}
		conn.SetDeadline(time.Now().Add(1e9))
		buf := make([]byte, 4096)
		if _, err := conn.Read(buf); nil != err {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			fmt.Println(err)
			return
		}
		if _, err := conn.Write(buf); nil != err {
			fmt.Println(err)
			return
		}
	}
}

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
	SetInfo     bool
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

	return &node
}

//ConnectIdentityChain connects to identity chain
func (node *NewNode) ConnectBeaconChain(BCPeer p2p.Peer) {
	time.Sleep(5 * time.Second)
	msg := node.SerializeNode()
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	p2p.SendMessage(BCPeer, msgToSend)
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
