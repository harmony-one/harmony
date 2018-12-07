package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNew(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	host := p2pimpl.NewHost(leader)
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	if consensus.consensusID != 0 {
		test.Errorf("Consensus Id is initialized to the wrong value: %d", consensus.consensusID)
	}

	if !consensus.IsLeader {
		test.Error("Consensus should belong to a leader")
	}

	if consensus.ReadySignal == nil {
		test.Error("Consensus ReadySignal should be initialized")
	}

	if consensus.leader != leader {
		test.Error("Consensus Leader is set to wrong Peer")
	}
}
