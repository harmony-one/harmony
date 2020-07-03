package p2ptests

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	harmony_bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/p2p"
	libp2p_crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
)

type host struct {
	IP   string
	Port string
}

var (
	hosts     []host
	topics    []string
	bootnodes []string
)

func init() {
	hosts = []host{
		{IP: "127.0.0.1", Port: "5000"},
		{IP: "8.8.8.8", Port: "5000"},
	}

	topics = []string{
		"hmy/testnet/0.0.1/client/beacon",
		"hmy/testnet/0.0.1/node/beacon",
		"hmy/testnet/0.0.1/node/shard/1",
		"hmy/testnet/0.0.1/node/shard/2",
		"hmy/testnet/0.0.1/node/shard/3",
	}

	bootnodes = []string{
		"/ip4/54.86.126.90/tcp/9850/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
		"/ip4/52.40.84.2/tcp/9850/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
	}
}

func createNode(address string, port string) (p2p.Host, *bls.PublicKey, error) {
	nodePrivateKey, _, err := generatePrivateKey()
	if err != nil {
		return nil, nil, errors.New("failed to generate private key for node")
	}

	peer, err := generatePeer(address, port)
	if err != nil {
		return nil, nil, err
	}

	host, err := p2p.NewHost(&peer, nodePrivateKey)
	if err != nil {
		return nil, nil, err
	}

	return host, peer.ConsensusPubKey, nil
}

func generatePeer(address string, port string) (p2p.Peer, error) {
	peerPrivateKey := harmony_bls.RandPrivateKey()
	peerPublicKey := peerPrivateKey.GetPublicKey()
	if peerPrivateKey == nil || peerPublicKey == nil {
		return p2p.Peer{}, errors.New("failed to generate bls key for peer")
	}

	peer := p2p.Peer{IP: address, Port: port, ConsensusPubKey: peerPublicKey}

	return peer, nil
}

func generatePrivateKey() (privateKey libp2p_crypto.PrivKey, publicKey libp2p_crypto.PubKey, err error) {
	privateKey, publicKey, err = libp2p_crypto.GenerateKeyPair(libp2p_crypto.RSA, 2048)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, publicKey, nil
}
