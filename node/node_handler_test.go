package node

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestAddNewBlock(t *testing.T) {
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, false)

	selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
	node.Worker.CommitTransactions(selectedTxs)
	block, _ := node.Worker.Commit([]byte{}, []byte{}, 0, common.Address{})

	node.AddNewBlock(block)

	if node.Blockchain().CurrentBlock().NumberU64() != 1 {
		t.Error("New block is not added successfully")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, false)

	selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
	node.Worker.CommitTransactions(selectedTxs)
	block, _ := node.Worker.Commit([]byte{}, []byte{}, 0, common.Address{})

	if err := node.VerifyNewBlock(block); err != nil {
		t.Error("New block is not verified successfully:", err)
	}
}
