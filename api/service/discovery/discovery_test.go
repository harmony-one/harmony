package discovery

import (
	"testing"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	ip       = "127.0.0.1"
	port     = "7099"
	dService *Service
)

func TestDiscoveryService(t *testing.T) {
	nodePriKey, _, err := utils.LoadKeyFromFile("/tmp/127.0.0.1.12345.key")
	if err != nil {
		t.Fatal(err)
	}
	peerPriKey := bls.RandPrivateKey()
	peerPubKey := peerPriKey.GetPublicKey()
	if peerPriKey == nil || peerPubKey == nil {
		t.Fatal("generate key error")
	}
	selfPeer := p2p.Peer{IP: "127.0.0.1", Port: "12345", ConsensusPubKey: peerPubKey}

	host, err := p2pimpl.NewHost(&selfPeer, nodePriKey)
	if err != nil {
		t.Fatalf("unable to new host in harmony: %v", err)
	}

	config := service.NodeConfig{}

	dService = New(host, config, nil, nil)

	if dService == nil {
		t.Fatalf("unable to create new discovery service")
	}

	dService.StartService()

	time.Sleep(3 * time.Second)

	dService.StopService()
}
