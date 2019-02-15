package discovery

import (
	"testing"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	ip      = "127.0.0.1"
	port    = "7099"
	service *Service
)

func TestDiscoveryService(t *testing.T) {
	selfPeer := p2p.Peer{IP: ip, Port: port}
	priKey, _, err := utils.GenKeyP2P(ip, port)

	host, err := p2pimpl.NewHost(&selfPeer, priKey)
	if err != nil {
		t.Fatalf("unable to new host in harmony: %v", err)
	}

	service = New(host, "rendezvous", nil, nil)

	if service == nil {
		t.Fatalf("unable to create new discovery service")
	}
}
