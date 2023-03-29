package discovery

// TODO: test this module

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

func TestNewDHTDiscovery(t *testing.T) {
	host, err := libp2p.New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var idht *dht.IpfsDHT
	idht, err = dht.New(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewDHTDiscovery(ctx, cancel, host, idht, DHTConfig{})
	if err != nil {
		t.Fatal(err)
	}
}
