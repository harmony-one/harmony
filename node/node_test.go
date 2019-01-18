package node

import (
	"fmt"
	"os"
	"testing"
	"time"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNewNode(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", PubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)
	node := New(host, consensus, nil)
	if node.Consensus == nil {
		t.Error("Consensus is not initialized for the node")
	}

	if node.blockchain == nil {
		t.Error("Blockchain is not initialized for the node")
	}

	if node.blockchain.CurrentBlock() == nil {
		t.Error("Genesis block is not initialized for the node")
	}
}

func TestGetSyncingPeers(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", PubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)

	node := New(host, consensus, nil)
	peer := p2p.Peer{IP: "127.0.0.1", Port: "8000"}
	peer2 := p2p.Peer{IP: "127.0.0.1", Port: "8001"}
	node.Neighbors.Store("minh", peer)
	node.Neighbors.Store("mark", peer2)
	res := node.GetSyncingPeers()
	if len(res) != 1 || !(res[0].IP == peer.IP || res[0].IP == peer2.IP) {
		t.Error("GetSyncingPeers should return list of {peer, peer2}")
	}
	if len(res) != 1 || (res[0].Port != "5000" && res[0].Port != "5001") {
		t.Errorf("Syncing ports should be 5000, got %v", res[0].Port)
	}
}

func TestAddPeers(t *testing.T) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)

	priKey2 := crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 := pki.GetPublicKeyFromScalar(priKey2)

	peers1 := []*p2p.Peer{
		&p2p.Peer{
			IP:          "127.0.0.1",
			Port:        "8888",
			PubKey:      pubKey1,
			Ready:       true,
			ValidatorID: 1,
		},
		&p2p.Peer{
			IP:          "127.0.0.1",
			Port:        "9999",
			PubKey:      pubKey2,
			Ready:       false,
			ValidatorID: 2,
		},
	}
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8982", PubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8985"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)

	node := New(host, consensus, nil)
	r1 := node.AddPeers(peers1)
	e1 := 2
	if r1 != e1 {
		t.Errorf("Add %v peers, expectd %v", r1, e1)
	}
	r2 := node.AddPeers(peers1)
	e2 := 0
	if r2 != e2 {
		t.Errorf("Add %v peers, expectd %v", r2, e2)
	}
}

func sendPingMessage(node *Node, leader p2p.Peer) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)

	p1 := p2p.Peer{
		IP:     "127.0.0.1",
		Port:   "9999",
		PubKey: pubKey1,
	}

	ping1 := proto_node.NewPingMessage(p1)
	buf1 := ping1.ConstructPingMessage()

	fmt.Println("waiting for 5 seconds ...")
	time.Sleep(5 * time.Second)

	node.SendMessage(leader, buf1)
	fmt.Println("sent ping message ...")
}

func sendPongMessage(node *Node, leader p2p.Peer) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)
	p1 := p2p.Peer{
		IP:     "127.0.0.1",
		Port:   "9998",
		PubKey: pubKey1,
	}
	priKey2 := crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 := pki.GetPublicKeyFromScalar(priKey2)
	p2 := p2p.Peer{
		IP:     "127.0.0.1",
		Port:   "9999",
		PubKey: pubKey2,
	}

	pong1 := proto_node.NewPongMessage([]p2p.Peer{p1, p2}, nil)
	buf1 := pong1.ConstructPongMessage()

	fmt.Println("waiting for 10 seconds ...")
	time.Sleep(10 * time.Second)

	node.SendMessage(leader, buf1)
	fmt.Println("sent pong message ...")
}

func exitServer() {
	fmt.Println("wait 5 seconds to terminate the process ...")
	time.Sleep(5 * time.Second)

	os.Exit(0)
}

func TestPingPongHandler(test *testing.T) {
	_, pubKey := utils.GenKey("127.0.0.1", "8881")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8881", PubKey: pubKey}
	//   validator := p2p.Peer{IP: "127.0.0.1", Port: "9991"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader}, leader)
	node := New(host, consensus, nil)
	//go sendPingMessage(leader)
	go sendPongMessage(node, leader)
	go exitServer()
	node.StartServer()
}
