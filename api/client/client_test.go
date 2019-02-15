package client

import (
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/p2p"
)

func TestClient(t *testing.T) {
	leaders := map[uint32]p2p.Peer{
		0: p2p.Peer{
			IP:   "127.0.0.1",
			Port: "90",
		},
	}
	client := NewClient(nil, leaders)

	leadersGot := client.GetLeaders()

	if len(leadersGot) != 1 || !reflect.DeepEqual(leaders[0], leadersGot[0]) {
		t.Errorf("expected: %v, got: %v\n", leaders[0], leadersGot[0])
	}
}
