package drand

import (
	"testing"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "55555"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil)
	dRand.blockHash = [32]byte{}
	msg := dRand.constructCommitMessage([32]byte{}, []byte{})

	if len(msg) != 127 {
		test.Errorf("Commit message is not constructed in the correct size: %d", len(msg))
	}
}
