package node

import (
	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"testing"
)

func TestNodeStreamHandler(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", PubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)
	node := New(host, consensus, nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := p2p.NewMockStream(ctrl)

	m.EXPECT().Read(gomock.Any()).AnyTimes()
	m.EXPECT().SetReadDeadline(gomock.Any())
	m.EXPECT().Close()

	node.StreamHandler(m)
}

func TestAddNewBlock(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", PubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9885"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)
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
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", PubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	host, _ := p2pimpl.NewHost(&leader)
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)
	node := New(host, consensus, nil)

	selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
	node.Worker.CommitTransactions(selectedTxs)
	block, _ := node.Worker.Commit()

	if !node.VerifyNewBlock(block) {
		t.Error("New block is not verified successfully")
	}
}
