package node

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestAddNewBlock(t *testing.T) {
	pubKey := bls.RandPrivateKey().GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, 0, []p2p.Peer{leader, validator}, leader, nil)
	node := New(host, consensus, nil)

	selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
	node.Worker.CommitTransactions(selectedTxs)
	block, _ := node.Worker.Commit()

	node.AddNewBlock(block)

	if node.blockchain.CurrentBlock().NumberU64() != 1 {
		t.Error("New block is not added successfully")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	pubKey := bls.RandPrivateKey().GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, 0, []p2p.Peer{leader, validator}, leader, nil)
	node := New(host, consensus, nil)

	selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
	node.Worker.CommitTransactions(selectedTxs)
	block, _ := node.Worker.Commit()

	if !node.VerifyNewBlock(block) {
		t.Error("New block is not verified successfully")
	}
}
