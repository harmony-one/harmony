package node

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
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
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKey(blsKey), decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	nodeconfig.SetNetworkType(nodeconfig.Devnet)
	node := New(host, consensus, testDBFactory, nil, false)

	txs := make(map[common.Address]types.Transactions)
	stks := staking.StakingTransactions{}
	node.Worker.CommitTransactions(
		txs, stks, common.Address{},
	)
	block, _ := node.Worker.FinalizeNewBlock(
		[]byte{}, []byte{}, 0, common.Address{}, nil, nil,
	)

	_, err = node.Blockchain().InsertChain([]*types.Block{block}, true)
	if err != nil {
		t.Errorf("error when adding new block %v", err)
	}

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
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKey(blsKey), decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, nil, false)

	txs := make(map[common.Address]types.Transactions)
	stks := staking.StakingTransactions{}
	node.Worker.CommitTransactions(
		txs, stks, common.Address{},
	)
	block, _ := node.Worker.FinalizeNewBlock(
		[]byte{}, []byte{}, 0, common.Address{}, nil, nil,
	)

	if err := node.VerifyNewBlock(block); err != nil {
		t.Error("New block is not verified successfully:", err)
	}
}
