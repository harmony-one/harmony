package consensus

import (
	"testing"
	"harmony-benchmark/p2p"
)

func TestConstructAnnounceMessage(test *testing.T) {
	header := getBlockHeader()
	leader := p2p.Peer{Ip: "1", Port:"2"}
	validator := p2p.Peer{Ip: "3", Port:"5"}
	consensus := NewConsensus("1", "2", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = getBlockHash(make([]byte, 10))
	msg, err := consensus.constructAnnounceMessage()

	if err != nil {
		test.Error("Annouce message is not constructed successfully")
	}
	if len(msg) != 1 + 1 + 1 + 4 + 32 + 2 + 4 + 64 + len(header) {
		test.Errorf("Annouce message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructChallengeMessage(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port:"2"}
	validator := p2p.Peer{Ip: "3", Port:"5"}
	consensus := NewConsensus("1", "2", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = getBlockHash(make([]byte, 10))
	msg := consensus.constructChallengeMessage()

	if len(msg) != 1 + 1 + 1 + 4 + 32 + 2 + 33 + 33 + 32 + 64 {
		test.Errorf("Annouce message is not constructed in the correct size: %d", len(msg))
	}
}