package helpers

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
	libp2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"
)

// Host - struct for representing a host (IP / Port)
type Host struct {
	IP   string
	Port string
}

var (
	// Hosts - host combinations
	Hosts []Host
	// Topics - p2p topics
	Topics []string
	// Bootnodes - p2p bootnodes
	Bootnodes []string
)

func init() {
	Hosts = []Host{
		{IP: nodeconfig.DefaultLocalListenIP, Port: "9000"},
		{IP: nodeconfig.DefaultLocalListenIP, Port: "9001"},
	}

	Topics = []string{
		"hmy/testnet/0.0.1/client/beacon",
		"hmy/testnet/0.0.1/node/beacon",
		"hmy/testnet/0.0.1/node/shard/1",
		"hmy/testnet/0.0.1/node/shard/2",
		"hmy/testnet/0.0.1/node/shard/3",
	}

	Bootnodes = []string{
		"/ip4/54.86.126.90/tcp/9850/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
		"/ip4/52.40.84.2/tcp/9850/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
	}
}

// GenerateHost - test helper to generate a new host
func GenerateHost(address string, port string) (p2p.Host, *bls.PublicKey, error) {
	nodePrivateKey, _, err := GeneratePrivateKey()
	if err != nil {
		return nil, nil, errors.New("failed to generate private key for node")
	}

	peer, err := GeneratePeer(address, port)
	if err != nil {
		return nil, nil, err
	}
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:          &peer,
		BLSKey:        nodePrivateKey,
		BootNodes:     nil,
		DataStoreFile: nil,
	})
	if err != nil {
		return nil, nil, err
	}

	return host, peer.ConsensusPubKey, nil
}

// GeneratePeer - test helper to generate a new peer
func GeneratePeer(address string, port string) (p2p.Peer, error) {
	peerPrivateKey := harmony_bls.RandPrivateKey()
	peerPublicKey := peerPrivateKey.GetPublicKey()
	if peerPrivateKey == nil || peerPublicKey == nil {
		return p2p.Peer{}, errors.New("failed to generate bls key for peer")
	}

	peer := p2p.Peer{IP: address, Port: port, ConsensusPubKey: peerPublicKey}

	return peer, nil
}

// GeneratePrivateKey - test helper to generate a new private key to be used for p2p
func GeneratePrivateKey() (privateKey libp2p_crypto.PrivKey, publicKey libp2p_crypto.PubKey, err error) {
	privateKey, publicKey, err = libp2p_crypto.GenerateKeyPair(libp2p_crypto.RSA, 2048)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, publicKey, nil
}
