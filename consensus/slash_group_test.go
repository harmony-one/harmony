package consensus

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	coretypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	hmybls "github.com/harmony-one/harmony/crypto/bls"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
	"github.com/stretchr/testify/require"
)

func TestSlashGroupOrderCanRejectSameBeaconBlock(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	lateEpoch := new(big.Int).Add(params.MainnetChainConfig.HIP32Epoch, big.NewInt(3))
	middleEpoch := new(big.Int).Sub(lateEpoch, common.Big1)
	earlyEpoch := new(big.Int).Sub(lateEpoch, common.Big2)
	proposer := common.HexToAddress("0x0000000000000000000000000000000000000a01")
	offender := common.HexToAddress("0x0000000000000000000000000000000000000b02")
	reporter := common.HexToAddress("0x0000000000000000000000000000000000000c03")
	proposerSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	offenderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	committeePubKeys := []hmybls.PublicKeyWrapper{*proposerSigner.Pub, *offenderSigner.Pub}
	committeeSigners := []hmybls.PrivateKeyWrapper{proposerSigner, offenderSigner}
	coinbase := slashOrderCoinbaseAddress(proposerSigner)

	earlyHeight := slashOrderEvidenceHeight(earlyEpoch)
	lateHeight := slashOrderEvidenceHeight(lateEpoch)
	slashRecords := slash.Records{
		slashOrderRecord(t, earlyEpoch, 1, earlyHeight, 7, offender, reporter, offenderSigner),
		slashOrderRecord(t, lateEpoch, shard.BeaconChainShardID, lateHeight, 7, offender, reporter, offenderSigner),
	}

	chain, setupBlock := newSlashOrderChain(
		t,
		params.MainnetChainConfig,
		earlyEpoch,
		middleEpoch,
		lateEpoch,
		proposer,
		offender,
		coinbase,
		proposerSigner.Pub.Bytes,
		offenderSigner.Pub.Bytes,
		committeeSigners,
		committeePubKeys,
	)
	headState, err := chain.State()
	require.NoError(t, err)
	for i := range slashRecords {
		require.NoError(t, slash.Verify(chain, headState, &slashRecords[i]))
	}

	templateBlock := slashOrderBlock(t, setupBlock, slashRecords, common.Hash{}, nil, committeeSigners, committeePubKeys)
	roots := map[common.Hash]slashOrderRun{}
	for attempt := 0; attempt < 200 && len(roots) < 2; attempt++ {
		run := processSlashOrderBlock(t, chain, setupBlock, templateBlock, offender)
		roots[run.root] = run
	}
	require.Len(t, roots, 2, "same slash block should be able to produce both slash orders")
	requireSlashOrderOutcomes(t, roots)

	var proposerRun slashOrderRun
	for _, run := range roots {
		proposerRun = run
		break
	}
	proposedBlock := slashOrderBlock(
		t,
		setupBlock,
		slashRecords,
		proposerRun.root,
		proposerRun.receipts,
		committeeSigners,
		committeePubKeys,
	)

	sawAccepted, sawRejected := false, false
	for attempt := 0; attempt < 200 && (!sawAccepted || !sawRejected); attempt++ {
		verifierChain, verifierSetupBlock := newSlashOrderChain(
			t,
			params.MainnetChainConfig,
			earlyEpoch,
			middleEpoch,
			lateEpoch,
			proposer,
			offender,
			coinbase,
			proposerSigner.Pub.Bytes,
			offenderSigner.Pub.Bytes,
			committeeSigners,
			committeePubKeys,
		)
		require.Equal(t, setupBlock.Hash(), verifierSetupBlock.Hash())

		_, err := verifierChain.InsertChain([]*coretypes.Block{proposedBlock}, true)
		if err == nil {
			sawAccepted = true
			require.Equal(t, proposedBlock.Hash(), verifierChain.CurrentBlock().Hash())
			continue
		}
		require.Contains(t, err.Error(), "invalid merkle root")
		sawRejected = true
	}

	require.True(t, sawAccepted, "at least one honest importer should reproduce the proposer root")
	require.True(t, sawRejected, "at least one honest importer should compute the other root and reject the same block")
}

func TestSlashGroupOrderIsDeterministicAfterFix(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	lateEpoch := new(big.Int).Add(params.MainnetChainConfig.HIP32Epoch, big.NewInt(3))
	middleEpoch := new(big.Int).Sub(lateEpoch, common.Big1)
	earlyEpoch := new(big.Int).Sub(lateEpoch, common.Big2)
	proposer := common.HexToAddress("0x0000000000000000000000000000000000000a01")
	offender := common.HexToAddress("0x0000000000000000000000000000000000000b02")
	reporter := common.HexToAddress("0x0000000000000000000000000000000000000c03")
	proposerSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	offenderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	committeePubKeys := []hmybls.PublicKeyWrapper{*proposerSigner.Pub, *offenderSigner.Pub}
	committeeSigners := []hmybls.PrivateKeyWrapper{proposerSigner, offenderSigner}
	coinbase := slashOrderCoinbaseAddress(proposerSigner)

	earlyHeight := slashOrderEvidenceHeight(earlyEpoch)
	lateHeight := slashOrderEvidenceHeight(lateEpoch)
	slashRecords := slash.Records{
		slashOrderRecord(t, earlyEpoch, 1, earlyHeight, 7, offender, reporter, offenderSigner),
		slashOrderRecord(t, lateEpoch, shard.BeaconChainShardID, lateHeight, 7, offender, reporter, offenderSigner),
	}

	chainConfig := *params.MainnetChainConfig
	chainConfig.SlashGroupOrderFixEpoch = new(big.Int).Set(earlyEpoch)

	chain, setupBlock := newSlashOrderChain(
		t,
		&chainConfig,
		earlyEpoch,
		middleEpoch,
		lateEpoch,
		proposer,
		offender,
		coinbase,
		proposerSigner.Pub.Bytes,
		offenderSigner.Pub.Bytes,
		committeeSigners,
		committeePubKeys,
	)
	templateBlock := slashOrderBlock(t, setupBlock, slashRecords, common.Hash{}, nil, committeeSigners, committeePubKeys)

	var firstRun slashOrderRun
	for attempt := 0; attempt < 200; attempt++ {
		run := processSlashOrderBlock(t, chain, setupBlock, templateBlock, offender)
		if attempt == 0 {
			firstRun = run
			continue
		}
		require.Equal(t, firstRun.root, run.root, "canonical slash order should be deterministic")
		require.Zero(t, run.remainingSelf.Sign())
		require.Zero(t, run.remainingUnbonded.Cmp(slashOrderOnes(8_000)))
		require.Zero(t, run.coinbaseReward.Cmp(slashOrderOnes(12_000)))
	}

	proposedBlock := slashOrderBlock(
		t,
		setupBlock,
		slashRecords,
		firstRun.root,
		firstRun.receipts,
		committeeSigners,
		committeePubKeys,
	)
	for attempt := 0; attempt < 200; attempt++ {
		verifierChain, verifierSetupBlock := newSlashOrderChain(
			t,
			&chainConfig,
			earlyEpoch,
			middleEpoch,
			lateEpoch,
			proposer,
			offender,
			coinbase,
			proposerSigner.Pub.Bytes,
			offenderSigner.Pub.Bytes,
			committeeSigners,
			committeePubKeys,
		)
		require.Equal(t, setupBlock.Hash(), verifierSetupBlock.Hash())
		_, err := verifierChain.InsertChain([]*coretypes.Block{proposedBlock}, true)
		require.NoError(t, err)
		require.Equal(t, proposedBlock.Hash(), verifierChain.CurrentBlock().Hash())
	}
}

type slashOrderRun struct {
	root              common.Hash
	remainingSelf     *big.Int
	remainingUnbonded *big.Int
	coinbaseReward    *big.Int
	receipts          coretypes.Receipts
}

func newSlashOrderChain(
	t *testing.T,
	chainConfig *params.ChainConfig,
	earlyEpoch *big.Int,
	middleEpoch *big.Int,
	lateEpoch *big.Int,
	proposer common.Address,
	offender common.Address,
	coinbase common.Address,
	proposerBLS hmybls.SerializedPublicKey,
	offenderBLS hmybls.SerializedPublicKey,
	signers []hmybls.PrivateKeyWrapper,
	pubKeys []hmybls.PublicKeyWrapper,
) (*core.BlockChainImpl, *coretypes.Block) {
	t.Helper()

	db := rawdb.NewMemoryDatabase()
	genesisState := slashOrderShardState(common.Big0, shard.BeaconChainShardID, proposer, offender, proposerBLS, offenderBLS)
	gspec := core.Genesis{
		Config:     chainConfig,
		Factory:    blockfactory.ForMainnet,
		ShardID:    shard.BeaconChainShardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: genesisState,
	}
	genesis := gspec.MustCommit(db)

	for _, shardState := range []shard.State{
		slashOrderShardState(earlyEpoch, 1, proposer, offender, proposerBLS, offenderBLS),
		slashOrderShardState(lateEpoch, shard.BeaconChainShardID, proposer, offender, proposerBLS, offenderBLS),
	} {
		encodedState, err := shard.EncodeWrapper(shardState, true)
		require.NoError(t, err)
		require.NoError(t, rawdb.WriteShardStateBytes(db, shardState.Epoch, encodedState))
	}

	chain, err := core.NewBlockChain(
		db,
		nil,
		nil,
		&core.CacheConfig{SnapshotLimit: 0},
		gspec.Config,
		chain2.NewEngine(),
		vm.Config{},
	)
	require.NoError(t, err)

	setupState, err := chain.StateAt(genesis.Root())
	require.NoError(t, err)
	current := slashOrderValidator(offender, offenderBLS, slashOrderOnes(16_000), slashOrderOnes(16_000), middleEpoch, lateEpoch)
	setupState.SetValidatorFlag(offender)
	require.NoError(t, setupState.UpdateValidatorWrapper(offender, current))

	earlySnapshot := slashOrderValidator(offender, offenderBLS, slashOrderOnes(32_000), common.Big0, nil, lateEpoch)
	lateSnapshot := slashOrderValidator(offender, offenderBLS, slashOrderOnes(16_000), slashOrderOnes(16_000), middleEpoch, lateEpoch)
	for _, snapshot := range []staking.ValidatorSnapshot{
		{
			Validator: slashOrderCopyValidator(earlySnapshot),
			Epoch:     new(big.Int).Set(earlyEpoch),
		},
		{
			Validator: slashOrderCopyValidator(lateSnapshot),
			Epoch:     new(big.Int).Set(lateEpoch),
		},
	} {
		require.NoError(t, chain.WriteValidatorSnapshot(setupState.Database().DiskDB(), &snapshot))
	}

	setupNumber := slashOrderSetupBlockNumber(lateEpoch)
	setupHeader := slashOrderHeader(lateEpoch, setupNumber, coinbase, genesis.Hash())
	processor := core.NewStateProcessor(chain, chain)
	setupInput := coretypes.NewBlockWithHeader(setupHeader)
	receipts, outcxs, stakeMsgs, _, usedGas, payout, newState, err := processor.Process(setupInput, setupState, vm.Config{}, false)
	require.NoError(t, err)
	setupHeader.SetGasUsed(usedGas)
	setupHeader.SetRoot(newState.IntermediateRoot(chainConfig.IsS3(setupHeader.Epoch())))
	setupBlock := coretypes.NewBlock(setupHeader, nil, receipts, outcxs, nil, nil)
	setupBlock.SetCurrentCommitSig(slashOrderCommitSig(t, setupBlock, signers, pubKeys))

	status, err := chain.WriteBlockWithState(setupBlock, receipts, outcxs, stakeMsgs, payout, newState)
	require.NoError(t, err)
	require.Equal(t, core.CanonStatTy, status)
	return chain, setupBlock
}

func processSlashOrderBlock(
	t *testing.T,
	chain *core.BlockChainImpl,
	parent *coretypes.Block,
	block *coretypes.Block,
	offender common.Address,
) slashOrderRun {
	t.Helper()

	statedb, err := chain.StateAt(parent.Root())
	require.NoError(t, err)
	processor := core.NewStateProcessor(chain, chain)
	receipts, _, _, _, _, _, newState, err := processor.Process(block, statedb, vm.Config{}, false)
	require.NoError(t, err)
	validator, err := newState.ValidatorWrapper(offender, true, false)
	require.NoError(t, err)
	return slashOrderRun{
		root:              newState.IntermediateRoot(chain.Config().IsS3(block.Epoch())),
		remainingSelf:     new(big.Int).Set(validator.Delegations[0].Amount),
		remainingUnbonded: new(big.Int).Set(validator.Delegations[0].Undelegations[0].Amount),
		coinbaseReward:    new(big.Int).Set(newState.GetBalance(block.Header().Coinbase())),
		receipts:          receipts,
	}
}

func requireSlashOrderOutcomes(t *testing.T, roots map[common.Hash]slashOrderRun) {
	t.Helper()

	earlyFirstOutcome := false
	lateFirstOutcome := false
	for _, run := range roots {
		require.Zero(t, run.remainingSelf.Sign())
		switch {
		case run.remainingUnbonded.Cmp(slashOrderOnes(16_000)) == 0:
			earlyFirstOutcome = true
			require.Zero(t, run.coinbaseReward.Cmp(slashOrderOnes(8_000)))
		case run.remainingUnbonded.Cmp(slashOrderOnes(8_000)) == 0:
			lateFirstOutcome = true
			require.Zero(t, run.coinbaseReward.Cmp(slashOrderOnes(12_000)))
		default:
			t.Fatalf("unexpected remaining undelegation after slash ordering: %s", run.remainingUnbonded)
		}
	}
	require.True(t, earlyFirstOutcome, "one root should leave the middle-epoch undelegation untouched")
	require.True(t, lateFirstOutcome, "one root should slash the middle-epoch undelegation after active stake is partly consumed")
}

func slashOrderBlock(
	t *testing.T,
	parent *coretypes.Block,
	slashes slash.Records,
	root common.Hash,
	receipts coretypes.Receipts,
	signers []hmybls.PrivateKeyWrapper,
	pubKeys []hmybls.PublicKeyWrapper,
) *coretypes.Block {
	t.Helper()

	slashBytes, err := rlp.EncodeToBytes(slashes)
	require.NoError(t, err)
	header := slashOrderHeader(parent.Epoch(), parent.NumberU64()+1, parent.Header().Coinbase(), parent.Hash())
	header.SetSlashes(slashBytes)
	header.SetRoot(root)
	block := coretypes.NewBlock(header, nil, receipts, nil, nil, nil)
	if root != (common.Hash{}) {
		slashOrderSetLastCommitSig(t, block, parent)
		block.SetCurrentCommitSig(slashOrderCommitSig(t, block, signers, pubKeys))
	}
	return block
}

func slashOrderSetLastCommitSig(t *testing.T, block *coretypes.Block, parent *coretypes.Block) {
	t.Helper()

	sig, bitmap, err := chain2.ParseCommitSigAndBitmap(parent.GetCurrentCommitSig())
	require.NoError(t, err)
	block.SetLastCommitSig(sig[:], bitmap)
}

func slashOrderRecord(
	t *testing.T,
	epoch *big.Int,
	shardID uint32,
	height uint64,
	viewID uint64,
	offender common.Address,
	reporter common.Address,
	signer hmybls.PrivateKeyWrapper,
) slash.Record {
	t.Helper()

	firstBlock := slashOrderSignedBlock(epoch, shardID, height, viewID, common.HexToHash("0x01"))
	secondBlock := slashOrderSignedBlock(epoch, shardID, height, viewID, common.HexToHash("0x02"))
	require.NotEqual(t, firstBlock.Hash(), secondBlock.Hash())
	return slash.Record{
		Evidence: slash.Evidence{
			ConflictingVotes: slash.ConflictingVotes{
				FirstVote:  slashOrderVote(firstBlock, epoch, height, viewID, signer),
				SecondVote: slashOrderVote(secondBlock, epoch, height, viewID, signer),
			},
			Moment: slash.Moment{
				Epoch:   new(big.Int).Set(epoch),
				ShardID: shardID,
				Height:  height,
				ViewID:  viewID,
			},
			Offender: offender,
		},
		Reporter: reporter,
	}
}

func slashOrderVote(
	block *coretypes.Block,
	epoch *big.Int,
	height uint64,
	viewID uint64,
	signer hmybls.PrivateKeyWrapper,
) slash.Vote {
	payload := consensus_sig.ConstructCommitPayload(params.MainnetChainConfig, epoch, block.Hash(), height, viewID)
	return slash.Vote{
		SignerPubKeys:   []hmybls.SerializedPublicKey{signer.Pub.Bytes},
		BlockHeaderHash: block.Hash(),
		Signature:       signer.Pri.SignHash(payload).Serialize(),
	}
}

func slashOrderSignedBlock(
	epoch *big.Int,
	shardID uint32,
	height uint64,
	viewID uint64,
	root common.Hash,
) *coretypes.Block {
	header := blockfactory.ForMainnet.NewHeader(epoch).With().
		Number(new(big.Int).SetUint64(height)).
		Epoch(epoch).
		ShardID(shardID).
		ViewID(new(big.Int).SetUint64(viewID)).
		Root(root).
		GasLimit(params.TestGenesisGasLimit).
		Header()
	return coretypes.NewBlockWithHeader(header)
}

func slashOrderHeader(epoch *big.Int, number uint64, coinbase common.Address, parentHash common.Hash) *block.Header {
	return blockfactory.ForMainnet.NewHeader(epoch).With().
		Number(new(big.Int).SetUint64(number)).
		Epoch(epoch).
		ShardID(shard.BeaconChainShardID).
		ViewID(common.Big1).
		Coinbase(coinbase).
		ParentHash(parentHash).
		GasLimit(params.TestGenesisGasLimit).
		Time(new(big.Int).SetUint64(number * 2)).
		Header()
}

func slashOrderCommitSig(
	t *testing.T,
	block *coretypes.Block,
	signers []hmybls.PrivateKeyWrapper,
	pubKeys []hmybls.PublicKeyWrapper,
) []byte {
	t.Helper()
	require.Len(t, signers, len(pubKeys))

	payload := consensus_sig.ConstructCommitPayload(
		params.MainnetChainConfig,
		block.Epoch(),
		block.Hash(),
		block.NumberU64(),
		block.Header().ViewID().Uint64(),
	)
	mask := hmybls.NewMask(pubKeys)
	sigs := make([]*bls_core.Sign, 0, len(signers))
	for i := range signers {
		require.NoError(t, mask.SetBit(i, true))
		sigs = append(sigs, signers[i].Pri.SignHash(payload))
	}
	aggSig := hmybls.AggregateSig(sigs)
	return append(aggSig.Serialize(), mask.Mask()...)
}

func slashOrderCoinbaseAddress(signer hmybls.PrivateKeyWrapper) common.Address {
	var coinbase common.Address
	blsAddress := signer.Pub.Object.GetAddress()
	coinbase.SetBytes(blsAddress[:])
	return coinbase
}

func slashOrderShardState(
	epoch *big.Int,
	committeeShardID uint32,
	proposer common.Address,
	offender common.Address,
	proposerBLS hmybls.SerializedPublicKey,
	offenderBLS hmybls.SerializedPublicKey,
) shard.State {
	stake := numeric.OneDec()
	return shard.State{
		Epoch: new(big.Int).Set(epoch),
		Shards: []shard.Committee{
			{
				ShardID: committeeShardID,
				Slots: shard.SlotList{
					{
						EcdsaAddress:   proposer,
						BLSPublicKey:   proposerBLS,
						EffectiveStake: &stake,
					},
					{
						EcdsaAddress:   offender,
						BLSPublicKey:   offenderBLS,
						EffectiveStake: &stake,
					},
				},
			},
		},
	}
}

func slashOrderValidator(
	offender common.Address,
	offenderBLS hmybls.SerializedPublicKey,
	activeSelfStake *big.Int,
	undelegation *big.Int,
	undelegationEpoch *big.Int,
	lastEpoch *big.Int,
) *staking.ValidatorWrapper {
	selfDelegation := staking.Delegation{
		DelegatorAddress: offender,
		Amount:           new(big.Int).Set(activeSelfStake),
		Reward:           common.Big0,
	}
	if undelegation.Sign() > 0 {
		selfDelegation.Undelegations = staking.Undelegations{
			{
				Amount: new(big.Int).Set(undelegation),
				Epoch:  new(big.Int).Set(undelegationEpoch),
			},
		}
	}
	return &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offender,
			SlotPubKeys:          []hmybls.SerializedPublicKey{offenderBLS},
			LastEpochInCommittee: new(big.Int).Set(lastEpoch),
			MinSelfDelegation:    slashOrderOnes(10_000),
			MaxTotalDelegation:   slashOrderOnes(1_000_000),
			Status:               effective.Active,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					Rate:          numeric.MustNewDecFromStr("0.1"),
					MaxRate:       numeric.MustNewDecFromStr("0.9"),
					MaxChangeRate: numeric.MustNewDecFromStr("0.1"),
				},
				UpdateHeight: common.Big0,
			},
			Description: staking.Description{
				Name:            "slash-order-offender",
				Identity:        "slash-order-offender",
				Website:         "slash-order-offender",
				SecurityContact: "slash-order-offender",
				Details:         "slash-order-offender",
			},
			CreationHeight: common.Big0,
		},
		Delegations: staking.Delegations{selfDelegation},
	}
}

func slashOrderCopyValidator(wrapper *staking.ValidatorWrapper) *staking.ValidatorWrapper {
	copied := staketest.CopyValidatorWrapper(*wrapper)
	return &copied
}

func slashOrderSetupBlockNumber(epoch *big.Int) uint64 {
	number := shardingconfig.MainnetSchedule.EpochLastBlock(epoch.Uint64()-1) + 1
	for slashOrderBadBlockNumber(number) || slashOrderBadBlockNumber(number+1) {
		number++
	}
	return number
}

func slashOrderEvidenceHeight(epoch *big.Int) uint64 {
	height := shardingconfig.MainnetSchedule.EpochLastBlock(epoch.Uint64()-1) + 37
	for slashOrderBadBlockNumber(height) {
		height++
	}
	return height
}

func slashOrderBadBlockNumber(number uint64) bool {
	return shard.Schedule.IsLastBlock(number) ||
		number%shardingconfig.RewardFrequency == shardingconfig.RewardFrequency-1
}

func slashOrderOnes(amount int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(amount), big.NewInt(denominations.One))
}
