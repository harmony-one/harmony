package node

import (
	"harmony-benchmark/p2p"
	"testing"
	"harmony-benchmark/consensus"
)

func TestNewNewNode(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port: "2"}
	validator := p2p.Peer{Ip: "3", Port: "5"}
	consensus := consensus.NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)

	node := NewNode(&consensus)
	if node.consensus == nil {
		test.Error("Consensus is not initialized for the node")
	}

	if node.blockchain == nil {
		test.Error("Blockchain is not initialized for the node")
	}

	if len(node.blockchain.Blocks) != 1 {
		test.Error("Genesis block is not initialized for the node")
	}

	if len(node.blockchain.Blocks[0].Transactions) != 1 {
		test.Error("Coinbase TX is not initialized for the node")
	}

	if node.utxoPool == nil {
		test.Error("Utxo pool is not initialized for the node")
	}
}
