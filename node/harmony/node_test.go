package node

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"testing"

	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	libp2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
)

var testDBFactory = &shardchain.MemDBFactory{}

// randomPeer generates a random Peer
func randomPeer() (libp2p_crypto.PrivKey, libp2p_crypto.PubKey, libp2p_peer.ID) {
	// Generate a random private key
	priv, _, err := libp2p_crypto.GenerateKeyPairWithReader(libp2p_crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		panic(err)
	}
	pub := priv.GetPublic()
	// Get the peer ID from the private key
	pid, err := libp2p_peer.IDFromPrivateKey(priv)
	if err != nil {
		panic(err)
	}
	return priv, pub, pid
}

// createRandomNode creates a random peer
func createRandomNode(port string) (string, *ffi_bls.SecretKey, *p2p.Peer) {
	var blsKey ffi_bls.SecretKey
	blsKey.SetByCSPRNG() // Secure random private key
	pubKey := blsKey.GetPublicKey()

	// Convert BLS public key to bytes
	blsPubKeyBytes := pubKey.Serialize()

	// Hash BLS public key using SHA-256
	hash := sha256.Sum256(blsPubKeyBytes)

	// Encode hash into Multihash format
	mh, err := multihash.EncodeName(hash[:], "sha2-256")
	if err != nil {
		log.Fatalf("Failed to encode multihash: %v", err)
	}

	// Generate a Peer ID using the Multihash
	peerID, err := libp2p_peer.IDFromBytes(mh)
	if err != nil {
		log.Fatalf("Failed to generate Peer ID: %v", err)
	}

	// Create P2P Peer struct
	p2pPeer := &p2p.Peer{
		IP:              "127.0.0.1",
		Port:            port,
		ConsensusPubKey: pubKey,
		PeerID:          peerID,
	}

	// Create a multiaddress
	addr := fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", "127.0.0.1", port, peerID)
	peerAddr, _ := multiaddr.NewMultiaddr(addr)

	return peerAddr.String(), &blsKey, p2pPeer
}

func TestTrustedNodes(t *testing.T) {
	_, _, leader := createRandomNode("8882")
	addr1, _, _ := createRandomNode("8884")
	addr2, _, _ := createRandomNode("8886")
	addr3, _, _ := createRandomNode("8888")
	trustedNodes := []string{addr1, addr2, addr3}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:         leader,
		BLSKey:       priKey,
		TrustedNodes: trustedNodes,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	host.Start()

	connectedPeers := host.GetPeerCount()
	if connectedPeers != len(trustedNodes)+1 {
		t.Fatalf("host adding trusted nodes failed, expected:%d, got:%d", len(trustedNodes), connectedPeers)
	}
}
func TestNewNode(t *testing.T) {
	_, blsKey, leader := createRandomNode("8882")
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	engine := chain.NewEngine()
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	chainconfig := nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType().ChainConfig()
	collection := shardchain.NewCollection(
		nil, testDBFactory, &core.GenesisInitializer{NetworkType: nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType()}, engine, &chainconfig,
	)
	blockchain, err := collection.ShardChain(shard.BeaconChainShardID)
	if err != nil {
		t.Fatal("cannot get blockchain")
	}
	reg := registry.New().
		SetBlockchain(blockchain).
		SetEngine(engine).
		SetShardChainCollection(collection)

	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsKey), reg, decider, 3, false,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}

	node := New(host, consensus, nil, nil, nil, nil, reg)
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

func TestDNSSyncingPeerProvider(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		addrs := make([]multiaddr.Multiaddr, 0)
		p := NewDNSSyncingPeerProvider("example.com", "1234", addrs)
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
		addrs := make([]multiaddr.Multiaddr, 0)
		p := NewDNSSyncingPeerProvider("example.com", "1234", addrs)
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
			{IP: "127.0.0.1", Port: "6004"},
			{IP: "127.0.0.1", Port: "6008"},
		}
		if actualPeers, err := p.SyncingPeers(0); assert.NoError(t, err) {
			assert.ElementsMatch(t, actualPeers[:3], expectedBeaconPeers)
		}
	})
	t.Run("Shard1Chain", func(t *testing.T) {
		p := makeLocalSyncingPeerProvider()
		expectedShard1Peers := []p2p.Peer{
			// port 6001 omitted because self
			{IP: "127.0.0.1", Port: "6002"},
			{IP: "127.0.0.1", Port: "6006"},
		}
		if actualPeers, err := p.SyncingPeers(1); assert.NoError(t, err) {
			assert.ElementsMatch(t, actualPeers[:2], expectedShard1Peers)
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
