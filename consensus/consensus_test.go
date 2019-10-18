package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/values"
	"github.com/harmony-one/harmony/crypto/bls"
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
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	consensus, err := New(
		host, values.BeaconChainShardID, leader, bls.RandPrivateKey(), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	if consensus.viewID != 0 {
		test.Errorf("Consensus Id is initialized to the wrong value: %d", consensus.viewID)
	}

	if consensus.ReadySignal == nil {
		test.Error("Consensus ReadySignal should be initialized")
	}
}
