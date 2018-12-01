package newnode

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/bcconn"
	"github.com/harmony-one/harmony/crypto"
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

type NewNode struct {
	Role        string
	ShardID     int
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	isLeader    bool
	Self        p2p.Peer
	peers       []p2p.Peer
	PubK        kyber.Point
	priK        kyber.Scalar
	log         log.Logger
	SetInfo     bool
	Service     *Service
}

//NewNode
func New(ip string, port string) *NewNode {
	priKey, pubKey := utils.GenKey(ip, port)
	var node NewNode
	node.PubK = pubKey
	node.priK = priKey
	node.Self = p2p.Peer{IP: ip, Port: port, PubKey: pubKey, ValidatorID: -1}
	node.log = log.New()
	node.SetInfo = false
	return &node
}

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*bcconn.NodeInfo
}

// NewService
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

// Serve Accept connections and spawn a goroutine to serve each one.  Stop listening
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
		listener.SetDeadline(time.Now().Add(1e9)) // This deadline is for 1 second to accept new connections.
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
	return fmt.Sprintf("idc: %v:%v and node infi %v", node.Self.IP, node.Self.Port, node.SetInfo)
}

//ConnectBeaconChain connects to beacon chain
func (node *NewNode) ConnectBeaconChain(BCPeer p2p.Peer) {
	node.log.Info("connecting to beacon chain now ...")
	pubk, err := node.PubK.MarshalBinary()
	if err != nil {
		node.log.Error("Could not Marshall public key into binary")
	}
	p := p2p.Peer{IP: node.Self.IP, Port: node.Self.Port}
	nodeInfo := &bcconn.NodeInfo{Self: p, PubK: pubk}
	msg := bcconn.SerializeNodeInfo(nodeInfo)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	gotShardInfo := false
	timeout := time.After(300 * time.Second)
	tick := time.Tick(1 * time.Second)
checkLoop:
	for {
		select {
		case <-timeout:
			gotShardInfo = false
			break checkLoop
		case <-tick:
			if node.SetInfo {
				gotShardInfo = true
				break checkLoop
			} else {
				p2p.SendMessage(BCPeer, msgToSend)
			}
		}
	}
	if !gotShardInfo {
		node.log.Crit("Could not get sharding info after 5 minutes")
		os.Exit(1)
	}
}

//ProcessShardInfo
func (node *NewNode) processShardInfo(msgPayload []byte) bool {
	leadersInfo := bcconn.DeserializeRandomInfo(msgPayload)
	leaders := leadersInfo.Leaders
	shardNum, isLeader := utils.AllocateShard(leadersInfo.NumberOfNodesAdded, leadersInfo.NumberOfShards)
	leaderNode := leaders[shardNum-1] //0 indexing.
	//node.leader = leaderNode.Self     //Does not have public key.

	leaderPeer := p2p.Peer{IP: leaderNode.Self.IP, Port: leaderNode.Self.Port}
	leaderPeer.PubKey = crypto.Ed25519Curve.Point()
	err := leaderPeer.PubKey.UnmarshalBinary(leaderNode.PubK[:])
	if err != nil {
		node.log.Info("Could not unmarshall leaders public key from binary to kyber.point")
	}
	node.leader = leaderPeer
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

func (node *NewNode) GetClientPeer() *p2p.Peer {
	return nil
}

//GetSelfPeer
func (node *NewNode) GetSelfPeer() p2p.Peer {
	return node.Self
}
