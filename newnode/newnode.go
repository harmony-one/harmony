package newnode

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"strconv"
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

type NodeInfo struct {
	Self p2p.Peer
	PubK []byte
}
type NewNode struct {
	Role        string
	ShardID     int
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	isLeader    bool
	Self        p2p.Peer
	peers       []p2p.Peer
	PubK        kyber.Scalar
	priK        kyber.Point
	log         log.Logger
	SetInfo     bool
	Service     *Service
}

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*NodeInfo
}

// Make a new Service.
func (node *NewNode) NewService(ip, port string) *Service {
	laddr, err := net.ResolveTCPAddr("tcp", ip+":"+port)
	if nil != err {
		node.log.Crit("cannot resolve the tcp address of the new node", err)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if nil != err {
		node.log.Crit("cannot start a listener for new node", err)
	}
	node.log.Debug("listening on", "address", laddr.String())

	node.Service = &Service{
		ch:        make(chan bool),
		waitGroup: &sync.WaitGroup{},
	}
	node.Service.waitGroup.Add(1)
	go node.Serve(listener)
	return node.Service
}

// Accept connections and spawn a goroutine to serve each one.  Stop listening
// if anything is received on the service's channel.
func (node *NewNode) Serve(listener *net.TCPListener) {
	defer node.Service.waitGroup.Done()
	for {
		select {
		case <-node.Service.ch:
			node.log.Debug("stopping listening on", "address", listener.Addr())
			listener.Close()
			node.log.Debug("stopped listening")
			return
		default:
		}
		listener.SetDeadline(time.Now().Add(1e9))
		conn, err := listener.AcceptTCP()
		if nil != err {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			node.log.Error(err.Error())

		}
		node.Service.waitGroup.Add(1)
		go node.NodeHandler(conn)
	}
}

// Stop the service by closing the service's channel.  Block until the service
// is really stopped.
func (s *Service) Stop() {
	close(s.ch)
	s.waitGroup.Wait()
}

func (node NewNode) String() string {
	return fmt.Sprintf("idc: %v:%v and node infi %v", node.Self.Ip, node.Self.Port, node.SetInfo)
}

//NewNode
func New(ip string, port string) *NewNode {
	pubKey, priKey := utils.GenKey(ip, port)
	var node NewNode
	node.PubK = pubKey
	node.priK = priKey
	node.Self = p2p.Peer{Ip: ip, Port: port}
	node.peers = make([]p2p.Peer, 0)
	node.log = log.New()
	node.SetInfo = false
	return &node
}

//ConnectIdentityChain connects to identity chain
func (node *NewNode) ConnectBeaconChain(BCPeer p2p.Peer) {
	node.log.Info("connecting to beacon chain now ...")
	pubk, err := node.PubK.MarshalBinary()
	if err != nil {
		node.log.Error("Could not Marshall public key into binary")
	}
	nodeInfo := &NodeInfo{Self: node.Self, PubK: pubk}
	msg := SerializeNodeInfo(nodeInfo)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	p2p.SendMessage(BCPeer, msgToSend) //TODO: need backoff behavior for connecting to beaconchain over minuted
}

//SerializeNode
func SerializeNodeInfo(nodeinfo *NodeInfo) []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(nodeinfo)
	if err != nil {
		log.Error("Could not serialize node info", err)
	}
	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeNodeInfo(d []byte) *NodeInfo {
	var wn NodeInfo
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize node info", err)
	}
	return &wn
}

// DeserializeNode deserializes the node
func DeserializeRandomInfo(d []byte) *registerResponseRandomNumber {
	var wn registerResponseRandomNumber
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize random info for 1")
	}
	return &wn
}

//ProcessShardInfo
func (node *NewNode) processShardInfo(msgPayload []byte) bool {
	leadersInfo := DeserializeRandomInfo(msgPayload)
	leaders := leadersInfo.Leaders
	shardNum, isLeader := utils.AllocateShard(leadersInfo.NumberOfNodesAdded, leadersInfo.NumberOfShards)
	leaderNode := leaders[shardNum-1] //0 indexing.
	node.leader = leaderNode.Self
	node.isLeader = isLeader
	node.ShardID = shardNum
	node.SetInfo = true
	node.log.Info("Shard information obtained ..")
	return true
}

func (node *NewNode) GetShardID() string {
	return strconv.Itoa(node.ShardID)
}

func (node *NewNode) GetLeader() p2p.Peer {
	return node.leader
}

func (node *NewNode) GetSelfPeer() p2p.Peer {
	return node.Self
}
