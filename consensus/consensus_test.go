package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNew(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
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

func TestRemovePeers(t *testing.T) {
	_, pk1 := utils.GenKey("1", "1")
	_, pk2 := utils.GenKey("2", "2")
	_, pk3 := utils.GenKey("3", "3")
	_, pk4 := utils.GenKey("4", "4")
	_, pk5 := utils.GenKey("5", "5")

	p1 := p2p.Peer{IP: "127.0.0.1", Port: "19901", PubKey: pk1}
	p2 := p2p.Peer{IP: "127.0.0.1", Port: "19902", PubKey: pk2}
	p3 := p2p.Peer{IP: "127.0.0.1", Port: "19903", PubKey: pk3}
	p4 := p2p.Peer{IP: "127.0.0.1", Port: "19904", PubKey: pk4}

	peers := []p2p.Peer{p1, p2, p3, p4}

	peerRemove := []p2p.Peer{p1, p2}

	leader := p2p.Peer{IP: "127.0.0.1", Port: "9000", PubKey: pk5}
	host := p2pimpl.NewHost(leader)
	consensus := New(host, "0", peers, leader)

	//	consensus.DebugPrintPublicKeys()
	f := consensus.RemovePeers(peerRemove)
	if f == 0 {
		t.Errorf("consensus.RemovePeers return false")
		consensus.DebugPrintPublicKeys()
	}
}

func TestGetPeerFromID(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	host := p2pimpl.NewHost(leader)
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	leaderID := utils.GetUniqueIDFromIPPort(leader.IP, leader.Port)
	validatorID := utils.GetUniqueIDFromIPPort(validator.IP, validator.Port)
	l, _ := consensus.GetPeerFromID(leaderID)
	v, _ := consensus.GetPeerFromID(validatorID)
	if l.IP != leader.IP || l.Port != leader.Port {
		t.Errorf("leader IP not equal")
	}
	if v.IP != validator.IP || v.Port != validator.Port {
		t.Errorf("validator IP not equal")
	}
}
