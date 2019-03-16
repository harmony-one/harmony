package node

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/drand"

	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNewNode(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader, nil)
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
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}

	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader, nil)

	node := New(host, consensus, nil)
	peer := p2p.Peer{IP: "127.0.0.1", Port: "8000"}
	peer2 := p2p.Peer{IP: "127.0.0.1", Port: "8001"}
	node.Neighbors.Store("minh", peer)
	node.Neighbors.Store("mark", peer2)
	res := node.GetSyncingPeers()
	if len(res) == 0 || !(res[0].IP == peer.IP || res[0].IP == peer2.IP) {
		t.Error("GetSyncingPeers should return list of {peer, peer2}")
	}
	if len(res) == 0 || (res[0].Port != "5000" && res[0].Port != "5001") {
		t.Errorf("Syncing ports should be 5000, got %v", res[0].Port)
	}
}

func TestAddPeers(t *testing.T) {
	pubKey1 := pki.GetBLSPrivateKeyFromInt(333).GetPublicKey()
	pubKey2 := pki.GetBLSPrivateKeyFromInt(444).GetPublicKey()

	peers1 := []*p2p.Peer{
		&p2p.Peer{
			IP:              "127.0.0.1",
			Port:            "8888",
			ConsensusPubKey: pubKey1,
			ValidatorID:     1,
		},
		&p2p.Peer{
			IP:              "127.0.0.1",
			Port:            "9999",
			ConsensusPubKey: pubKey2,
			ValidatorID:     2,
		},
	}
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8982", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8985"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader, nil)
	dRand := drand.New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true, nil)

	node := New(host, consensus, nil)
	node.DRand = dRand
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

func TestAddBeaconPeer(t *testing.T) {
	_, pubKey1 := utils.GenKey("127.0.0.1", "2000")
	_, pubKey2 := utils.GenKey("127.0.0.1", "8000")

	peers1 := []*p2p.Peer{
		&p2p.Peer{
			IP:              "127.0.0.1",
			Port:            "8888",
			ConsensusPubKey: pubKey1,
			ValidatorID:     1,
			PeerID:          "1234",
		},
		&p2p.Peer{
			IP:              "127.0.0.1",
			Port:            "9999",
			ConsensusPubKey: pubKey2,
			ValidatorID:     2,
			PeerID:          "4567",
		},
	}
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8982", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8985"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader, nil)
	dRand := drand.New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true, nil)

	node := New(host, consensus, nil)
	node.DRand = dRand
	for _, p := range peers1 {
		ret := node.AddBeaconPeer(p)
		if ret {
			t.Errorf("AddBeaconPeer Failed, expecting false, got %v, peer %v", ret, p)
		}
	}
	for _, p := range peers1 {
		ret := node.AddBeaconPeer(p)
		if !ret {
			t.Errorf("AddBeaconPeer Failed, expecting true, got %v, peer %v", ret, p)
		}
	}
}

func sendPingMessage(node *Node, leader p2p.Peer) {
	pubKey1 := pki.GetBLSPrivateKeyFromInt(333).GetPublicKey()

	p1 := p2p.Peer{
		IP:              "127.0.0.1",
		Port:            "9999",
		ConsensusPubKey: pubKey1,
	}

	ping1 := proto_discovery.NewPingMessage(p1, true)
	ping2 := proto_discovery.NewPingMessage(p1, false)
	_ = ping1.ConstructPingMessage()
	_ = ping2.ConstructPingMessage()
}

func sendPongMessage(node *Node, leader p2p.Peer) {
	pubKey1 := pki.GetBLSPrivateKeyFromInt(333).GetPublicKey()
	pubKey2 := pki.GetBLSPrivateKeyFromInt(444).GetPublicKey()
	p1 := p2p.Peer{
		IP:              "127.0.0.1",
		Port:            "9998",
		ConsensusPubKey: pubKey1,
	}
	p2 := p2p.Peer{
		IP:              "127.0.0.1",
		Port:            "9999",
		ConsensusPubKey: pubKey2,
	}

	pubKeys := []*bls.PublicKey{pubKey1, pubKey2}
	leaderPubKey := pki.GetBLSPrivateKeyFromInt(888).GetPublicKey()

	pong1 := proto_discovery.NewPongMessage([]p2p.Peer{p1, p2}, pubKeys, leaderPubKey)
	_ = pong1.ConstructPongMessage()
}

func exitServer() {
	fmt.Println("wait 5 seconds to terminate the process ...")
	time.Sleep(5 * time.Second)

	os.Exit(0)
}

func TestPingPongHandler(t *testing.T) {
	_, pubKey := utils.GenKey("127.0.0.1", "8881")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8881", ConsensusPubKey: pubKey}
	//   validator := p2p.Peer{IP: "127.0.0.1", Port: "9991"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader}, leader, nil)
	node := New(host, consensus, nil)
	//go sendPingMessage(leader)
	go sendPongMessage(node, leader)
	go exitServer()
	node.StartServer()
}
