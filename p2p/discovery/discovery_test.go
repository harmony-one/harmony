package discovery

// TODO: test this module

import (
	"testing"

	"github.com/libp2p/go-libp2p"
)

func TestNewDHTDiscovery(t *testing.T) {
	host, err := libp2p.New()
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewDHTDiscovery(host, DHTConfig{})
	if err != nil {
		t.Fatal(err)
	}
}
