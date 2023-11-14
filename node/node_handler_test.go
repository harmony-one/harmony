package node

import (
	"math/big"
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
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

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
	consensus, err := consensus.New(host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsKey), reg, decider, 3, false)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	nodeconfig.SetNetworkType(nodeconfig.Testnet)
	node := New(host, consensus, nil, nil, nil, nil, reg)

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

	consensusObj, err := consensus.New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsKey), reg, decider, 3, false,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	archiveMode := make(map[uint32]bool)
	archiveMode[0] = true
	archiveMode[1] = false
	node := New(host, consensusObj, nil, nil, nil, nil, reg)

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

	// work around vrf verification as it's tested in another test.
	node.Blockchain().Config().VRFEpoch = big.NewInt(2)
	if err := node.Blockchain().ValidateNewBlock(block, node.Beaconchain()); err != nil {
		t.Error("New block is not verified successfully:", err)
	}
}

func TestVerifyVRF(t *testing.T) {
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
	engine := chain.NewEngine()
	chainconfig := nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType().ChainConfig()
	collection := shardchain.NewCollection(
		nil, testDBFactory, &core.GenesisInitializer{NetworkType: nodeconfig.GetShardConfig(shard.BeaconChainShardID).GetNetworkType()}, engine, &chainconfig,
	)
	blockchain, err := collection.ShardChain(shard.BeaconChainShardID)
	if err != nil {
		t.Fatal("cannot get blockchain")
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
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
	archiveMode := make(map[uint32]bool)
	archiveMode[0] = true
	archiveMode[1] = false
	node := New(host, consensus, nil, nil, nil, nil, reg)

	txs := make(map[common.Address]types.Transactions)
	stks := staking.StakingTransactions{}
	node.Worker.CommitTransactions(
		txs, stks, common.Address{},
	)
	commitSigs := make(chan []byte)
	go func() {
		commitSigs <- []byte{}
	}()

	ecdsaAddr := pubKey.GetAddress()

	shardState := &shard.State{}
	com := shard.Committee{ShardID: uint32(0)}

	spKey := bls.SerializedPublicKey{}
	spKey.FromLibBLSPublicKey(pubKey)
	curNodeID := shard.Slot{
		EcdsaAddress: ecdsaAddr,
		BLSPublicKey: spKey,
	}
	com.Slots = append(com.Slots, curNodeID)
	shardState.Epoch = big.NewInt(1)
	shardState.Shards = append(shardState.Shards, com)

	node.Consensus.LeaderPubKey = &bls.PublicKeyWrapper{Bytes: spKey, Object: pubKey}
	node.Worker.GetCurrentHeader().SetEpoch(big.NewInt(1))
	node.Consensus.GenerateVrfAndProof(node.Worker.GetCurrentHeader())
	block, _ := node.Worker.FinalizeNewBlock(
		commitSigs, func() uint64 { return 0 }, ecdsaAddr, nil, shardState,
	)
	// Write shard state for the new epoch
	node.Blockchain().WriteShardStateBytes(node.Blockchain().ChainDb(), big.NewInt(1), node.Worker.GetCurrentHeader().ShardState())

	node.Blockchain().Config().VRFEpoch = big.NewInt(0)
	if err := node.Blockchain().Engine().VerifyVRF(
		node.Blockchain(), block.Header(),
	); err != nil {
		t.Error("New vrf is not verified successfully:", err)
	}
}
