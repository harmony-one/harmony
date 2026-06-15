package consensus

import (
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/consensus/quorum"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	corepkg "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	coretypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	hmybls "github.com/harmony-one/harmony/crypto/bls"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/multibls"
	workerpkg "github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
	"github.com/stretchr/testify/require"
)

var crossLinkCacheInvalidShardEpochOffset uint64

func TestCrossLinkCacheInvalidShardVariantHaltsRewardFinalization(t *testing.T) {
	t.Run("cachePrimingDoesNotAcceptInvalidShard", func(t *testing.T) {
		testCrossLinkCachePrimingDoesNotAcceptInvalidShard(t)
	})
	t.Run("rewardFinalizationSucceedsAfterValidCrossLink", func(t *testing.T) {
		testCrossLinkCacheRewardFinalizationSucceeds(t)
	})
}

func testCrossLinkCachePrimingDoesNotAcceptInvalidShard(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	epochOffset := atomic.AddUint64(&crossLinkCacheInvalidShardEpochOffset, 1)
	attackEpoch := new(big.Int).Add(
		params.MainnetChainConfig.HIP32Epoch,
		new(big.Int).SetUint64(80+epochOffset),
	)
	beaconLeader := common.HexToAddress("0x0000000000000000000000000000000000000b01")
	shardLeader := common.HexToAddress("0x0000000000000000000000000000000000000c01")
	beaconSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	shardSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())

	// The shard state only has beacon shard 0 and shard 1, so shard 2 is not a valid committee.
	attackState := crossLinkCacheShardState(
		attackEpoch,
		beaconLeader,
		shardLeader,
		beaconSigner.Pub.Bytes,
		shardSigner.Pub.Bytes,
	)

	beaconChain, _, _ := crossLinkCacheNewShardChain(t, shard.BeaconChainShardID, nil, corepkg.GenesisAlloc{}, attackState)

	shardChain, shardStateDB, shardGenesis := crossLinkCacheNewShardChain(t, 1, beaconChain, corepkg.GenesisAlloc{}, attackState)
	shardProcessor := corepkg.NewStateProcessor(shardChain, beaconChain)

	// Block 3 carries the commit signature for block 2, so this creates a valid shard-1 cross-link for block 2.
	shardBlock1 := crossLinkCacheCommitBlockWithOptionalLastCommit(
		t,
		shardChain,
		shardProcessor,
		shardStateDB,
		attackEpoch,
		1,
		1,
		shardLeader,
		shardGenesis.Hash(),
		nil,
		&shardSigner,
	)
	shardBlock2 := crossLinkCacheCommitBlockWithOptionalLastCommit(
		t,
		shardChain,
		shardProcessor,
		shardStateDB,
		attackEpoch,
		2,
		1,
		shardLeader,
		shardBlock1.Hash(),
		shardBlock1.GetCurrentCommitSig(),
		&shardSigner,
	)
	shardBlock3 := crossLinkCacheCommitBlockWithOptionalLastCommit(
		t,
		shardChain,
		shardProcessor,
		shardStateDB,
		attackEpoch,
		3,
		1,
		shardLeader,
		shardBlock2.Hash(),
		shardBlock2.GetCurrentCommitSig(),
		&shardSigner,
	)

	validCrossLink := *coretypes.NewCrossLink(shardBlock3.Header(), shardBlock2.Header())
	require.Equal(t, uint32(1), validCrossLink.ShardID())
	require.Equal(t, shardBlock2.NumberU64(), validCrossLink.BlockNum())

	invalidShardCrossLink := validCrossLink
	invalidShardCrossLink.ShardIDF = 2
	// Only ShardID changes; the hash, signature, and bitmap used by the verifier cache stay the same.
	require.Equal(t, validCrossLink.Hash(), invalidShardCrossLink.Hash())
	require.Equal(t, validCrossLink.Signature(), invalidShardCrossLink.Signature())
	require.Equal(t, validCrossLink.Bitmap(), invalidShardCrossLink.Bitmap())

	// With an empty verifier cache, the shard-2 variant fails because there is no shard-2 committee.
	require.NoError(t, chain2.NewEngine().VerifyCrossLink(beaconChain, validCrossLink))
	require.Error(t, chain2.NewEngine().VerifyCrossLink(beaconChain, invalidShardCrossLink))

	// A valid cross-link must not prime the cache for a different shard ID.
	eng := chain2.NewEngine()
	require.NoError(t, eng.VerifyCrossLink(beaconChain, validCrossLink))
	require.Error(t, eng.VerifyCrossLink(beaconChain, invalidShardCrossLink))
}

func testCrossLinkCacheRewardFinalizationSucceeds(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	epochOffset := atomic.AddUint64(&crossLinkCacheInvalidShardEpochOffset, 1)
	attackEpoch := new(big.Int).Add(
		params.MainnetChainConfig.HIP32Epoch,
		new(big.Int).SetUint64(80+epochOffset),
	)
	beaconLeader := common.HexToAddress("0x0000000000000000000000000000000000000b01")
	shardLeader := common.HexToAddress("0x0000000000000000000000000000000000000c01")
	beaconSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	shardSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())

	attackState := crossLinkCacheShardState(
		attackEpoch,
		beaconLeader,
		shardLeader,
		beaconSigner.Pub.Bytes,
		shardSigner.Pub.Bytes,
	)

	beaconChain, beaconState, beaconGenesis := crossLinkCacheNewShardChain(t, shard.BeaconChainShardID, nil, corepkg.GenesisAlloc{}, attackState)
	beaconProcessor := corepkg.NewStateProcessor(beaconChain, beaconChain)
	crossLinkCacheInstallRewardValidators(
		t,
		beaconChain,
		beaconState,
		attackEpoch,
		crossLinkCacheRewardValidator{address: beaconLeader, blsKey: beaconSigner.Pub.Bytes},
		crossLinkCacheRewardValidator{address: shardLeader, blsKey: shardSigner.Pub.Bytes},
	)

	shardChain, shardStateDB, shardGenesis := crossLinkCacheNewShardChain(t, 1, beaconChain, corepkg.GenesisAlloc{}, attackState)
	shardProcessor := corepkg.NewStateProcessor(shardChain, beaconChain)

	shardBlock1 := crossLinkCacheCommitBlockWithOptionalLastCommit(
		t, shardChain, shardProcessor, shardStateDB, attackEpoch, 1, 1, shardLeader, shardGenesis.Hash(), nil, &shardSigner,
	)
	shardBlock2 := crossLinkCacheCommitBlockWithOptionalLastCommit(
		t, shardChain, shardProcessor, shardStateDB, attackEpoch, 2, 1, shardLeader, shardBlock1.Hash(), shardBlock1.GetCurrentCommitSig(), &shardSigner,
	)
	shardBlock3 := crossLinkCacheCommitBlockWithOptionalLastCommit(
		t, shardChain, shardProcessor, shardStateDB, attackEpoch, 3, 1, shardLeader, shardBlock2.Hash(), shardBlock2.GetCurrentCommitSig(), &shardSigner,
	)

	validCrossLink := *coretypes.NewCrossLink(shardBlock3.Header(), shardBlock2.Header())

	windowStart := crossLinkCacheFirstRewardWindowStartInEpoch(t, attackEpoch)
	rewardBoundary := windowStart + shardingconfig.RewardFrequency - 1

	// The bootstrap block starts the reward window; the poisoned header is imported next.
	bootstrapHeader := crossLinkCacheNewHeader(
		attackEpoch,
		windowStart,
		shard.BeaconChainShardID,
		beaconLeader,
		beaconGenesis.Hash(),
	)
	bootstrapBlock, _, _, _, err := crossLinkCacheProcessAndCommitBlock(
		t,
		beaconChain,
		beaconProcessor,
		beaconState,
		bootstrapHeader,
		nil,
		nil,
		&beaconSigner,
	)
	require.NoError(t, err)

	beaconConsensus := crossLinkCacheConsensusHarness(t, beaconChain, beaconChain, beaconSigner)
	lastCommitSig := bootstrapBlock.GetCurrentCommitSig()

	goodBlock := crossLinkCacheProposeAndInsertBeaconBlockWithCrossLinks(
		t,
		beaconConsensus,
		beaconChain,
		beaconSigner,
		lastCommitSig,
		coretypes.CrossLinks{validCrossLink},
		nil,
	)
	require.Equal(t, windowStart+1, goodBlock.NumberU64())

	storedValidCrossLink, err := beaconChain.ReadCrossLink(1, validCrossLink.BlockNum())
	require.NoError(t, err)
	require.Equal(t, validCrossLink.Hash(), storedValidCrossLink.Hash())
	lastCommitSig = goodBlock.GetCurrentCommitSig()

	for nextBlockNum := goodBlock.NumberU64() + 1; nextBlockNum < rewardBoundary; nextBlockNum++ {
		nextBlock := crossLinkCacheProposeAndInsertBeaconBlock(t, beaconConsensus, beaconChain, beaconSigner, lastCommitSig, nil)
		require.Equal(t, nextBlockNum, nextBlock.NumberU64())
		lastCommitSig = nextBlock.GetCurrentCommitSig()
	}

	beaconConsensus.SetViewIDs(beaconChain.CurrentBlock().NumberU64())
	commitSigs := make(chan []byte, 1)
	commitSigs <- lastCommitSig
	parentTime := beaconChain.CurrentHeader().Time().Int64()
	now := time.Unix(parentTime+1, 0)
	rewardBlock, err := beaconConsensus.ProposeNewBlock(now, commitSigs)
	require.NoError(t, err)
	require.NotNil(t, rewardBlock)
	require.Equal(t, rewardBoundary, rewardBlock.NumberU64())
}

func crossLinkCacheConsensusHarness(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	beacon corepkg.BlockChain,
	signer hmybls.PrivateKeyWrapper,
) *Consensus {
	t.Helper()

	txPoolConfig := corepkg.DefaultTxPoolConfig
	txPoolConfig.Journal = ""
	txPool := corepkg.NewTxPool(
		txPoolConfig,
		params.MainnetChainConfig,
		chain,
		coretypes.NewTransactionErrorSink(),
	)
	t.Cleanup(txPool.Stop)

	reg := registry.New().
		SetBlockchain(chain).
		SetBeaconchain(beacon).
		SetTxPool(txPool).
		SetCxPool(corepkg.NewCxPool(corepkg.CxPoolSize)).
		SetWorker(workerpkg.New(chain, beacon)).
		SetAddressToBLSKey(crossLinkCacheAddressToBLSKey{shardID: chain.ShardID()})

	decider := quorum.NewDecider(quorum.SuperMajorityStake, chain.ShardID())
	consensus, err := New(
		nil,
		chain.ShardID(),
		multibls.GetPrivateKeys(signer.Pri),
		reg,
		decider,
		1,
		false,
	)
	require.NoError(t, err)

	consensus.SetLeaderPubKey(signer.Pub)
	consensus.SetViewIDs(chain.CurrentBlock().NumberU64())
	return consensus
}

func crossLinkCacheNewShardChain(
	t *testing.T,
	shardID uint32,
	beaconChain corepkg.BlockChain,
	alloc corepkg.GenesisAlloc,
	attackState shard.State,
) (*corepkg.BlockChainImpl, *state.DB, *coretypes.Block) {
	t.Helper()

	genesisState := attackState
	genesisState.Epoch = common.Big0

	db := rawdb.NewMemoryDatabase()
	gspec := corepkg.Genesis{
		Config:     params.MainnetChainConfig,
		Factory:    blockfactory.ForMainnet,
		Alloc:      alloc,
		ShardID:    shardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: genesisState,
	}
	genesis := gspec.MustCommit(db)

	encodedAttackState, err := shard.EncodeWrapper(attackState, true)
	require.NoError(t, err)
	require.NoError(t, rawdb.WriteShardStateBytes(db, attackState.Epoch, encodedAttackState))

	chain, err := corepkg.NewBlockChain(
		db,
		nil,
		beaconChain,
		&corepkg.CacheConfig{SnapshotLimit: 0},
		gspec.Config,
		chain2.NewEngine(),
		vm.Config{},
	)
	require.NoError(t, err)

	statedb, err := chain.StateAt(genesis.Root())
	require.NoError(t, err)
	return chain, statedb, genesis
}

func crossLinkCacheShardState(
	epoch *big.Int,
	beaconLeader common.Address,
	shardLeader common.Address,
	beaconBLS hmybls.SerializedPublicKey,
	shardBLS hmybls.SerializedPublicKey,
) shard.State {
	stake := numeric.OneDec()
	return shard.State{
		Epoch: new(big.Int).Set(epoch),
		Shards: []shard.Committee{
			{
				ShardID: 0,
				Slots: shard.SlotList{
					{
						EcdsaAddress:   beaconLeader,
						BLSPublicKey:   beaconBLS,
						EffectiveStake: &stake,
					},
				},
			},
			{
				ShardID: 1,
				Slots: shard.SlotList{
					{
						EcdsaAddress:   shardLeader,
						BLSPublicKey:   shardBLS,
						EffectiveStake: &stake,
					},
				},
			},
		},
	}
}

type crossLinkCacheAddressToBLSKey struct {
	shardID uint32
}

func (a crossLinkCacheAddressToBLSKey) GetAddressForBLSKey(
	publicKeys multibls.PublicKeys,
	shardState registry.FindCommitteeByID,
	blskey *bls_core.PublicKey,
	epoch *big.Int,
) common.Address {
	return a.GetAddresses(publicKeys, shardState, epoch)[blskey.SerializeToHexStr()]
}

func (a crossLinkCacheAddressToBLSKey) GetAddresses(
	publicKeys multibls.PublicKeys,
	shardState registry.FindCommitteeByID,
	_ *big.Int,
) map[string]common.Address {
	addresses := map[string]common.Address{}
	committee, err := shardState.FindCommitteeByID(a.shardID)
	if err != nil {
		return addresses
	}
	for _, publicKey := range publicKeys {
		shardKey := hmybls.FromLibBLSPublicKeyUnsafe(publicKey.Object)
		if shardKey == nil {
			continue
		}
		addr, err := committee.AddressForBLSKey(*shardKey)
		if err != nil {
			continue
		}
		addresses[publicKey.Bytes.Hex()] = *addr
	}
	return addresses
}

type crossLinkCacheRewardValidator struct {
	address common.Address
	blsKey  hmybls.SerializedPublicKey
}

func crossLinkCacheInstallRewardValidators(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	statedb *state.DB,
	epoch *big.Int,
	validators ...crossLinkCacheRewardValidator,
) {
	t.Helper()

	addrs := make([]common.Address, 0, len(validators))
	for _, validator := range validators {
		// Reward finalization updates this wrapper when the slot is counted as signed.
		wrapper := staketest.GetDefaultValidatorWrapperWithAddr(
			validator.address,
			[]hmybls.SerializedPublicKey{validator.blsKey},
		)
		require.NoError(t, statedb.UpdateValidatorWrapper(validator.address, &wrapper))
		addrs = append(addrs, validator.address)

		// Reward calculation uses the snapshot to split each payout.
		snapshotWrapper := staketest.CopyValidatorWrapper(wrapper)
		snapshot := &staking.ValidatorSnapshot{
			Validator: &snapshotWrapper,
			Epoch:     new(big.Int).Set(epoch),
		}
		require.NoError(t, chain.WriteValidatorSnapshot(chain.ChainDb(), snapshot))
	}
	require.NoError(t, chain.WriteValidatorList(chain.ChainDb(), addrs))
}

func crossLinkCacheCommitBlockWithOptionalLastCommit(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	processor *corepkg.StateProcessor,
	statedb *state.DB,
	epoch *big.Int,
	blockNum uint64,
	shardID uint32,
	coinbase common.Address,
	parentHash common.Hash,
	lastCommit []byte,
	signer *hmybls.PrivateKeyWrapper,
) *coretypes.Block {
	t.Helper()

	header := crossLinkCacheNewHeader(epoch, blockNum, shardID, coinbase, parentHash)
	if len(lastCommit) > 0 {
		// A child header carries the commit signature for its parent block.
		crossLinkCacheSetLastCommitOnHeader(t, header, lastCommit)
	}

	block, _, _, _, err := crossLinkCacheProcessAndCommitBlock(
		t,
		chain,
		processor,
		statedb,
		header,
		nil,
		nil,
		signer,
	)
	require.NoError(t, err)
	return block
}

func crossLinkCacheNewHeader(
	epoch *big.Int,
	number uint64,
	shardID uint32,
	coinbase common.Address,
	parentHash common.Hash,
) *block.Header {
	return blockfactory.ForMainnet.NewHeader(epoch).With().
		Number(new(big.Int).SetUint64(number)).
		Epoch(epoch).
		ShardID(shardID).
		ViewID(common.Big1).
		Coinbase(coinbase).
		ParentHash(parentHash).
		GasLimit(params.TestGenesisGasLimit).
		Time(new(big.Int).SetUint64(number * 2)).
		Header()
}

func crossLinkCacheProcessAndCommitBlock(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	processor *corepkg.StateProcessor,
	statedb *state.DB,
	header *block.Header,
	txs []*coretypes.Transaction,
	incxs []*coretypes.CXReceiptsProof,
	signer *hmybls.PrivateKeyWrapper,
) (*coretypes.Block, coretypes.Receipts, coretypes.CXReceipts, uint64, error) {
	t.Helper()

	inputBlock := coretypes.NewBlockWithHeader(header).WithBody(txs, nil, nil, incxs)
	receipts, outcxs, stakeMsgs, _, usedGas, payout, _, err := processor.Process(
		inputBlock,
		statedb,
		vm.Config{},
		false,
	)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	header.SetGasUsed(usedGas)
	header.SetRoot(statedb.IntermediateRoot(params.MainnetChainConfig.IsS3(header.Epoch())))
	finalBlock := coretypes.NewBlock(header, txs, receipts, outcxs, incxs, nil)
	if signer != nil {
		finalBlock.SetCurrentCommitSig(crossLinkCacheSignHeaderCommitSig(t, finalBlock, *signer))
	}

	status, err := chain.WriteBlockWithState(
		finalBlock,
		receipts,
		outcxs,
		stakeMsgs,
		payout,
		statedb,
	)
	require.NoError(t, err)
	require.Equal(t, corepkg.CanonStatTy, status)

	return finalBlock, receipts, outcxs, usedGas, nil
}

func crossLinkCacheSignHeaderCommitSig(
	t *testing.T,
	block *coretypes.Block,
	signer hmybls.PrivateKeyWrapper,
) []byte {
	t.Helper()

	mask := hmybls.NewMask([]hmybls.PublicKeyWrapper{*signer.Pub})
	require.NoError(t, mask.SetBit(0, true))

	payload := consensus_sig.ConstructCommitPayload(
		params.MainnetChainConfig,
		block.Epoch(),
		block.Hash(),
		block.NumberU64(),
		block.Header().ViewID().Uint64(),
	)
	sig := signer.Pri.SignHash(payload)

	return append(sig.Serialize(), mask.Mask()...)
}

func crossLinkCacheSetLastCommitOnHeader(t *testing.T, header *block.Header, sigAndBitmap []byte) {
	t.Helper()

	sig, bitmap, err := chain2.ParseCommitSigAndBitmap(sigAndBitmap)
	require.NoError(t, err)
	header.SetLastCommitSignature(sig)
	header.SetLastCommitBitmap(bitmap)
}

func crossLinkCacheFirstRewardWindowStartInEpoch(t *testing.T, epoch *big.Int) uint64 {
	t.Helper()

	// Pick a reward window that stays inside this epoch.
	blockNum := shardingconfig.MainnetSchedule.EpochLastBlock(epoch.Uint64()-1) + 1
	for blockNum%shardingconfig.RewardFrequency != 0 {
		blockNum++
	}

	neededThrough := blockNum + shardingconfig.RewardFrequency - 1
	require.Less(t, neededThrough, shardingconfig.MainnetSchedule.EpochLastBlock(epoch.Uint64()))
	return blockNum
}

func crossLinkCacheProposeAndInsertBeaconBlock(
	t *testing.T,
	consensus *Consensus,
	chain *corepkg.BlockChainImpl,
	signer hmybls.PrivateKeyWrapper,
	lastCommitSig []byte,
	beforeInsert func(*coretypes.Block),
) *coretypes.Block {
	t.Helper()

	consensus.SetViewIDs(chain.CurrentBlock().NumberU64())
	commitSigs := make(chan []byte, 1)
	commitSigs <- lastCommitSig

	parentTime := chain.CurrentHeader().Time().Int64()
	now := time.Unix(parentTime+1, 0)
	block, err := consensus.ProposeNewBlock(now, commitSigs)
	require.NoError(t, err)
	block.SetCurrentCommitSig(crossLinkCacheSignHeaderCommitSig(t, block, signer))

	if beforeInsert != nil {
		beforeInsert(block)
	}
	require.NoError(t, consensus.verifyBlock(block))

	_, err = chain.InsertChain([]*coretypes.Block{block}, true)
	require.NoError(t, err)
	require.Equal(t, block.Hash(), chain.CurrentBlock().Hash())

	storedCommitSig, err := chain.ReadCommitSig(block.NumberU64())
	require.NoError(t, err)
	require.Equal(t, block.GetCurrentCommitSig(), storedCommitSig)
	return block
}

func crossLinkCacheProposeAndInsertBeaconBlockWithCrossLinks(
	t *testing.T,
	consensus *Consensus,
	chain *corepkg.BlockChainImpl,
	signer hmybls.PrivateKeyWrapper,
	lastCommitSig []byte,
	crossLinks coretypes.CrossLinks,
	beforeInsert func(*coretypes.Block),
) *coretypes.Block {
	t.Helper()

	if len(crossLinks) == 0 {
		return crossLinkCacheProposeAndInsertBeaconBlock(t, consensus, chain, signer, lastCommitSig, beforeInsert)
	}

	// The beacon leader supplies the selected cross-links before finalizing the candidate block.
	consensus.SetViewIDs(chain.CurrentBlock().NumberU64())
	worker := consensus.registry.GetWorker()
	env, err := worker.UpdateCurrent(time.Now())
	require.NoError(t, err)

	header := env.CurrentHeader()
	leaderKey := consensus.GetLeaderPubKey()
	coinbase := common.Address{}
	blsPubKeyBytes := leaderKey.Object.GetAddress()
	coinbase.SetBytes(blsPubKeyBytes[:])
	header.SetCoinbase(coinbase)

	if chain.Config().IsVRF(header.Epoch()) {
		require.NoError(t, consensus.GenerateVrfAndProof(header))
	}

	shardState, err := consensus.Blockchain().SuperCommitteeForNextEpoch(consensus.Beaconchain(), header, false)
	require.NoError(t, err)

	commitSigs := make(chan []byte, 1)
	commitSigs <- lastCommitSig
	block, err := worker.FinalizeNewBlock(
		commitSigs,
		func() uint64 { return consensus.GetCurBlockViewID() },
		coinbase,
		crossLinks,
		shardState,
	)
	require.NoError(t, err)
	block.SetCurrentCommitSig(crossLinkCacheSignHeaderCommitSig(t, block, signer))

	if beforeInsert != nil {
		beforeInsert(block)
	}
	require.NoError(t, consensus.verifyBlock(block))

	_, err = chain.InsertChain([]*coretypes.Block{block}, true)
	require.NoError(t, err)
	require.Equal(t, block.Hash(), chain.CurrentBlock().Hash())
	return block
}

func crossLinkCacheDecodeBlockCrossLinks(t *testing.T, block *coretypes.Block) coretypes.CrossLinks {
	t.Helper()

	if len(block.Header().CrossLinks()) == 0 {
		return nil
	}
	crossLinks := coretypes.CrossLinks{}
	require.NoError(t, rlp.DecodeBytes(block.Header().CrossLinks(), &crossLinks))
	return crossLinks
}
