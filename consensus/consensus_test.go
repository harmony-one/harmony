package consensus

import (
	"testing"
	"harmony-benchmark/p2p"
	"harmony-benchmark/message"
)
func TestNewConsensus(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port:"2"}
	validator := p2p.Peer{Ip: "3", Port:"5"}
	consensus := NewConsensus("1", "2", []p2p.Peer{leader, validator}, leader)
	if consensus.consensusId != 0 {
		test.Errorf("Consensus Id is initialized to the wrong value: %d", consensus.consensusId)
	}

	if consensus.IsLeader != true {
		test.Error("Consensus should belong to a leader")
	}

	if consensus.ReadySignal == nil {
		test.Error("Consensus ReadySignal should be initialized")
	}

	if consensus.actionType != byte(message.CONSENSUS) {
		test.Error("Consensus actionType should be CONSENSUS")
	}

	if consensus.msgCategory != byte(message.COMMITTEE) {
		test.Error("Consensus msgCategory should be COMMITTEE")
	}

	if consensus.leader != leader {
		test.Error("Consensus Leader is set to wrong Peer")
	}
}