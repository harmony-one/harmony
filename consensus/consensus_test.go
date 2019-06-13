package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNew(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus, err := New(host, 0, leader, bls.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	if consensus.viewID != 0 {
		test.Errorf("Consensus Id is initialized to the wrong value: %d", consensus.viewID)
	}

	if !nodeconfig.GetDefaultConfig().IsLeader() {
		test.Error("Consensus should belong to a leader")
	}

	if consensus.ReadySignal == nil {
		test.Error("Consensus ReadySignal should be initialized")
	}
}
