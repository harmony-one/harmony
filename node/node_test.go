package node

import (
	"errors"
	"sync"
	"testing"

	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/assert"
)

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
