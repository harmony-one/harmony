package node

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/servprotocolorg/harmony/consensus"
	"github.com/servprotocolorg/harmony/consensus/quorum"
	"github.com/servprotocolorg/harmony/core"
	"github.com/servprotocolorg/harmony/core/types"
	"github.com/servprotocolorg/harmony/crypto/bls"
	"github.com/servprotocolorg/harmony/internal/chain"
	nodeconfig "github.com/servprotocolorg/harmony/internal/configs/node"
	"github.com/servprotocolorg/harmony/internal/registry"
	"github.com/servprotocolorg/harmony/internal/shardchain"
	"github.com/servprotocolorg/harmony/internal/utils"
	"github.com/servprotocolorg/harmony/multibls"
	"github.com/servprotocolorg/harmony/p2p"
	"github.com/servprotocolorg/harmony/shard"
	staking "github.com/servprotocolorg/harmony/staking/types"
	"github.com/stretchr/testify/require"
)

func TestFinalizeNewBlockAsync(t *testing.T) {
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	var testDBFactory = &shardchain.MemDBFactory{}
	engine := chain.NewEngine()
	chainconfig := nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType().ChainConfig()
	collection := shardchain.NewCollection(
		nil, testDBFactory, &core.GenesisInitializer{NetworkType: nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType()}, engine, &chainconfig,
	)
	blockchain, err := collection.ShardChain(shard.BeaconChainShardID)
	require.NoError(t, err)

	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsKey), nil, decider, 3, false,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}

	node := New(host, consensus, engine, collection, nil, nil, nil, nil, nil, registry.New().SetBlockchain(blockchain))

	node.Worker.UpdateCurrent()

	txs := make(map[common.Address]types.Transactions)
	stks := staking.StakingTransactions{}
	node.Worker.CommitTransactions(
		txs, stks, common.Address{},
	)
	commitSigs := make(chan []byte)
	go func() {
		commitSigs <- []byte{}
	}()

	block, _ := node.Worker.FinalizeNewBlock(
		commitSigs, func() uint64 { return 0 }, common.Address{}, nil, nil,
	)

	if err := VerifyNewBlock(nil, blockchain, nil)(block); err != nil {
		t.Error("New block is not verified successfully:", err)
	}

	node.Blockchain().InsertChain(types.Blocks{block}, false)

	node.Worker.UpdateCurrent()

	_, err = node.Worker.FinalizeNewBlock(
		commitSigs, func() uint64 { return 0 }, common.Address{}, nil, nil,
	)

	if !strings.Contains(err.Error(), "cannot finalize block") {
		t.Error("expect timeout on FinalizeNewBlock")
	}
}
