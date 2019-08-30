package node

import (
	"fmt"
	"os"
	"testing"
	"time"

	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/shardchain"

	"github.com/harmony-one/harmony/drand"

	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var testDBFactory = &shardchain.MemDBFactory{}

func TestNewNode(t *testing.T) {
	blsKey := bls2.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, false)
	if node.Consensus == nil {
		t.Error("Consensus is not initialized for the node")
	}

	if node.Blockchain() == nil {
		t.Error("Blockchain is not initialized for the node")
	}

	if node.Blockchain().CurrentBlock() == nil {
		t.Error("Genesis block is not initialized for the node")
	}
}

func TestGetSyncingPeers(t *testing.T) {
	blsKey := bls2.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}

	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, false)
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
		{
			IP:              "127.0.0.1",
			Port:            "8888",
			ConsensusPubKey: pubKey1,
		},
		{
			IP:              "127.0.0.1",
			Port:            "9999",
			ConsensusPubKey: pubKey2,
		},
	}
	blsKey := bls2.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8982", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8985"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	dRand := drand.New(host, 0, []p2p.Peer{leader, validator}, leader, nil, nil)

	node := New(host, consensus, testDBFactory, false)
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
	pubKey1 := bls2.RandPrivateKey().GetPublicKey()
	pubKey2 := bls2.RandPrivateKey().GetPublicKey()

	peers1 := []*p2p.Peer{
		{
			IP:              "127.0.0.1",
			Port:            "8888",
			ConsensusPubKey: pubKey1,
			PeerID:          "1234",
		},
		{
			IP:              "127.0.0.1",
			Port:            "9999",
			ConsensusPubKey: pubKey2,
			PeerID:          "4567",
		},
	}
	blsKey := bls2.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8982", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8985"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	dRand := drand.New(host, 0, []p2p.Peer{leader, validator}, leader, nil, nil)

	node := New(host, consensus, testDBFactory, false)
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

func exitServer() {
	fmt.Println("wait 5 seconds to terminate the process ...")
	time.Sleep(5 * time.Second)

	os.Exit(0)
}
