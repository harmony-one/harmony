package drand

import (
	"testing"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestConstructInitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "55555"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)
	dRand.blockHash = [32]byte{}
	msg := dRand.constructInitMessage()

	if len(msg) != 93 {
		test.Errorf("Init message is not constructed in the correct size: %d", len(msg))
	}
}
