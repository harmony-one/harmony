package core_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/require"
)

var testDBFactory = &shardchain.MemDBFactory{}

func TestAddNewBlock(t *testing.T) {
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	engine := chain.NewEngine()
	chainconfig := nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType().ChainConfig()
	collection := shardchain.NewCollection(
		nil, testDBFactory, &core.GenesisInitializer{NetworkType: nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType()}, engine, &chainconfig,
	)
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blockchain, err := collection.ShardChain(shard.BeaconChainShardID)
	if err != nil {
		t.Fatal("cannot get blockchain")
	}
	reg := registry.New().
		SetBlockchain(blockchain).
		SetEngine(engine).
		SetShardChainCollection(collection)
	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsKey), reg, decider, 3, false,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	nodeconfig.SetNetworkType(nodeconfig.Testnet)
	var block *types.Block
	node := node.New(host, consensus, nil, nil, nil, nil, reg)
	commitSigs := make(chan []byte, 1)
	commitSigs <- []byte{}
	block, err = node.Worker.FinalizeNewBlock(
		commitSigs, func() uint64 { return uint64(0) }, common.Address{}, nil, nil,
	)
	if err != nil {
		t.Fatal("cannot finalize new block")
	}

	nn := node.Blockchain().CurrentBlock()
	t.Log("[*]", nn.NumberU64(), nn.Hash().Hex(), nn.ParentHash())

	_, err = blockchain.InsertChain([]*types.Block{block}, false)
	require.NoError(t, err, "error when adding new block")

	meta := blockchain.LeaderRotationMeta()
	require.NotEmptyf(t, meta, "error when getting leader rotation meta")

	t.Log("#", block.Header().NumberU64(), node.Blockchain().CurrentBlock().NumberU64(), block.Hash().Hex(), block.ParentHash())

	err = blockchain.Rollback([]common.Hash{block.Hash()})
	require.NoError(t, err, "error when rolling back")
}
