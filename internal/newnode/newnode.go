package newnode

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto/bcconn"
	proto_identity "github.com/harmony-one/harmony/api/proto/identity"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"

	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
	multiaddr "github.com/multiformats/go-multiaddr"
)

//NewNode is ther struct for a candidate node
type NewNode struct {
	Role        string
	ShardID     int
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	isLeader    bool
	Self        p2p.Peer
	Leaders     map[uint32]p2p.Peer
	PubK        *bls.PublicKey
	priK        *bls.SecretKey
	log         log.Logger
	SetInfo     chan bool
	host        p2p.Host
}

// New candidatenode initialization
func New(ip string, port string, nodePk p2p_crypto.PrivKey) *NewNode {
	priKey, pubKey := utils.GenKeyBLS(ip, port)
	var node NewNode
	var err error
	node.PubK = pubKey
	node.priK = priKey
	node.Self = p2p.Peer{IP: ip, Port: port, PubKey: pubKey, ValidatorID: -1}
	node.log = utils.GetLogInstance()
	node.SetInfo = make(chan bool)
	node.host, err = p2pimpl.NewHost(&node.Self, nodePk)
	if err != nil {
		node.log.Error("failed to create new host", "msg", err)
		return nil
	}
	node.Leaders = map[uint32]p2p.Peer{}
	return &node
}

type registerResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*proto_node.Info
}

// ContactBeaconChain starts a newservice in the candidate node
func (node *NewNode) ContactBeaconChain(BCPeer p2p.Peer) error {
	go node.host.BindHandlerAndServe(node.NodeHandler)
	return node.requestBeaconChain(BCPeer)
}

func (node NewNode) String() string {
	return fmt.Sprintf("bc: %v:%v => %v", node.Self.IP, node.Self.Port, node.Self.PeerID)
}

// RequestBeaconChain requests beacon chain for identity data
func (node *NewNode) requestBeaconChain(BCPeer p2p.Peer) (err error) {
	node.log.Info("connecting to beacon chain now ...")
	pubk := node.PubK.Serialize()
	if err != nil {
		node.log.Error("Could not Marshall public key into binary")
	}
	fmt.Printf("[New Node]: %v\n", *node)
	nodeInfo := &proto_node.Info{IP: node.Self.IP, Port: node.Self.Port, PubKey: pubk, PeerID: node.Self.PeerID}
	msg := bcconn.SerializeNodeInfo(nodeInfo)
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, msg)
	gotShardInfo := false
	timeout := time.After(2 * time.Minute)
	tick := time.Tick(3 * time.Second)
checkLoop:
	for {
		select {
		case <-timeout:
			gotShardInfo = false
			break checkLoop
		case <-tick:
			select {
			case setinfo := <-node.SetInfo:
				if setinfo {
					gotShardInfo = true
					break checkLoop
				}
			default:
				host.SendMessage(node.host, BCPeer, msgToSend, nil)
			}
		}
	}
	if !gotShardInfo {
		err = errors.New("could not create connection")
		node.log.Crit("Could not get sharding info after 2 minutes")
		os.Exit(10)
	}
	return
}

// ProcessShardInfo
func (node *NewNode) processShardInfo(msgPayload []byte) bool {
	leadersInfo := bcconn.DeserializeRandomInfo(msgPayload)
	leaders := leadersInfo.Leaders
	shardNum, isLeader := utils.AllocateShard(leadersInfo.NumberOfNodesAdded, leadersInfo.NumberOfShards)
	for n, v := range leaders {
		leaderPeer := p2p.Peer{IP: v.IP, Port: v.Port, PeerID: v.PeerID}

		addr := fmt.Sprintf("/ip4/%s/tcp/%s", leaderPeer.IP, leaderPeer.Port)
		targetAddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			log.Error("processShardInfo NewMultiaddr error", "error", err)
			return false
		}
		leaderPeer.Addrs = append(leaderPeer.Addrs, targetAddr)

		leaderPeer.PubKey = &bls.PublicKey{}
		err = leaderPeer.PubKey.Deserialize(v.PubKey[:])
		if err != nil {
			node.log.Error("Could not unmarshall leaders public key from binary to kyber.point")
		}
		node.Leaders[uint32(n)] = leaderPeer
	}

	node.leader = node.Leaders[uint32(shardNum-1)]
	node.isLeader = isLeader
	node.ShardID = shardNum - 1 //0 indexing.
	node.SetInfo <- true
	node.log.Info("Shard information obtained ..")
	return true
}

// GetShardID gives shardid of node
func (node *NewNode) GetShardID() string {
	return strconv.Itoa(node.ShardID)
}

// GetLeader gives the leader of the node
func (node *NewNode) GetLeader() p2p.Peer {
	return node.leader
}

// GetClientPeer gives the client of the node
func (node *NewNode) GetClientPeer() *p2p.Peer {
	return nil
}

// GetSelfPeer gives the peer part of the node's own struct
func (node *NewNode) GetSelfPeer() p2p.Peer {
	return node.Self
}

// AddPeer add new peer for newnode
func (node *NewNode) AddPeer(p *p2p.Peer) error {
	return node.host.AddPeer(p)
}
