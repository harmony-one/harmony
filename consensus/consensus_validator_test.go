package consensus

import (
	"testing"
	"harmony-benchmark/p2p"
)

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port:"2"}
	validator := p2p.Peer{Ip: "3", Port:"5"}
	consensus := NewConsensus("1", "2", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = getBlockHash(make([]byte, 10))
	msg := consensus.constructCommitMessage()

	if len(msg) != 1 + 1 + 1 + 4 + 32 + 2 + 33 + 64 {
		test.Errorf("Commit message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructResponseMessage(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port:"2"}
	validator := p2p.Peer{Ip: "3", Port:"5"}
	consensus := NewConsensus("1", "2", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = getBlockHash(make([]byte, 10))
	msg := consensus.constructResponseMessage()

	if len(msg) != 1 + 1 + 1 + 4 + 32 + 2 + 32 + 64 {
		test.Errorf("Response message is not constructed in the correct size: %d", len(msg))
	}
}