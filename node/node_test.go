package node

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/shardchain"

	"github.com/harmony-one/bls/ffi/go/bls"

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

func TestLegacySyncingPeerProvider(t *testing.T) {
	t.Run("ShardChain", func(t *testing.T) {
		p := makeLegacySyncingPeerProvider()
		expectedPeers := []p2p.Peer{
			{IP: "127.0.0.1", Port: "6001"},
			{IP: "127.0.0.1", Port: "6003"},
		}
		actualPeers, err := p.SyncingPeers(1)
		if assert.NoError(t, err) {
			assert.ElementsMatch(t, actualPeers, expectedPeers)
		}
	})
	t.Run("BeaconChain", func(t *testing.T) {
		p := makeLegacySyncingPeerProvider()
		expectedPeers := []p2p.Peer{
			{IP: "127.0.0.1", Port: "6000"},
			{IP: "127.0.0.1", Port: "6002"},
		}
		actualPeers, err := p.SyncingPeers(0)
		if assert.NoError(t, err) {
			assert.ElementsMatch(t, actualPeers, expectedPeers)
		}
	})
	t.Run("NoMatch", func(t *testing.T) {
		p := makeLegacySyncingPeerProvider()
		_, err := p.SyncingPeers(999)
		assert.Error(t, err)
	})
}

func makeLegacySyncingPeerProvider() *LegacySyncingPeerProvider {
	node := makeSyncOnlyNode()
	p := NewLegacySyncingPeerProvider(node)
	p.shardID = func() uint32 { return 1 }
	return p
}

func makeSyncOnlyNode() *Node {
	node := &Node{
		Neighbors:       sync.Map{},
		BeaconNeighbors: sync.Map{},
	}
	node.Neighbors.Store(
		"127.0.0.1:9001:omg", p2p.Peer{IP: "127.0.0.1", Port: "9001"})
	node.Neighbors.Store(
		"127.0.0.1:9003:wtf", p2p.Peer{IP: "127.0.0.1", Port: "9003"})
	node.BeaconNeighbors.Store(
		"127.0.0.1:9000:bbq", p2p.Peer{IP: "127.0.0.1", Port: "9000"})
	node.BeaconNeighbors.Store(
		"127.0.0.1:9002:cakes", p2p.Peer{IP: "127.0.0.1", Port: "9002"})
	return node
}

func TestDNSSyncingPeerProvider(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		p := NewDNSSyncingPeerProvider("example.com", "1234")
		lookupCount := 0
		lookupName := ""
		p.lookupHost = func(name string) (addrs []string, err error) {
			lookupCount++
			lookupName = name
			return []string{"1.2.3.4", "5.6.7.8"}, nil
		}
		expectedPeers := []p2p.Peer{
			{IP: "1.2.3.4", Port: "1234"},
			{IP: "5.6.7.8", Port: "1234"},
		}
		actualPeers, err := p.SyncingPeers( /*shardID*/ 3)
		if assert.NoError(t, err) {
			assert.Equal(t, actualPeers, expectedPeers)
		}
		assert.Equal(t, lookupCount, 1)
		assert.Equal(t, lookupName, "s3.example.com")
		if err != nil {
			t.Fatalf("SyncingPeers returned non-nil error %#v", err)
		}
	})
	t.Run("LookupError", func(t *testing.T) {
		p := NewDNSSyncingPeerProvider("example.com", "1234")
		p.lookupHost = func(_ string) ([]string, error) {
			return nil, errors.New("omg")
		}
		_, actualErr := p.SyncingPeers( /*shardID*/ 3)
		assert.Error(t, actualErr)
	})
}

func TestLocalSyncingPeerProvider(t *testing.T) {
	t.Run("BeaconChain", func(t *testing.T) {
		p := makeLocalSyncingPeerProvider()
		expectedBeaconPeers := []p2p.Peer{
			{IP: "127.0.0.1", Port: "6000"},
			{IP: "127.0.0.1", Port: "6002"},
			{IP: "127.0.0.1", Port: "6004"},
		}
		if actualPeers, err := p.SyncingPeers(0); assert.NoError(t, err) {
			assert.ElementsMatch(t, actualPeers, expectedBeaconPeers)
		}
	})
	t.Run("Shard1Chain", func(t *testing.T) {
		p := makeLocalSyncingPeerProvider()
		expectedShard1Peers := []p2p.Peer{
			// port 6001 omitted because self
			{IP: "127.0.0.1", Port: "6003"},
			{IP: "127.0.0.1", Port: "6005"},
		}
		if actualPeers, err := p.SyncingPeers(1); assert.NoError(t, err) {
			assert.ElementsMatch(t, actualPeers, expectedShard1Peers)
		}
	})
	t.Run("InvalidShard", func(t *testing.T) {
		p := makeLocalSyncingPeerProvider()
		_, err := p.SyncingPeers(999)
		assert.Error(t, err)
	})
}

func makeLocalSyncingPeerProvider() *LocalSyncingPeerProvider {
	return NewLocalSyncingPeerProvider(6000, 6001, 2, 3)
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

	pong1 := proto_discovery.NewPongMessage([]p2p.Peer{p1, p2}, pubKeys, leaderPubKey, 0)
	_ = pong1.ConstructPongMessage()
}

func exitServer() {
	fmt.Println("wait 5 seconds to terminate the process ...")
	time.Sleep(5 * time.Second)

	os.Exit(0)
}

func TestPingPongHandler(t *testing.T) {
	blsKey := bls2.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8881", ConsensusPubKey: pubKey}
	//   validator := p2p.Peer{IP: "127.0.0.1", Port: "9991"}
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
	//go sendPingMessage(leader)
	go sendPongMessage(node, leader)
	go exitServer()
	node.StartServer()
}
