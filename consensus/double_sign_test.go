package consensus

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	corepkg "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	coretypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	hmybls "github.com/harmony-one/harmony/crypto/bls"
	chain2 "github.com/harmony-one/harmony/internal/chain"
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

func TestDoubleSignEvidenceCannotBeReboundToLaterEpoch(t *testing.T) {
	previousSchedule := shard.Schedule
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
	})
	shard.Schedule = shardingconfig.MainnetSchedule

	oldEpoch := new(big.Int).Add(params.MainnetChainConfig.HIP32Epoch, big.NewInt(2))
	forgedEpoch := new(big.Int).Add(oldEpoch, common.Big1)
	offender := common.HexToAddress("0x0000000000000000000000000000000000000b41")
	newDelegator := common.HexToAddress("0x0000000000000000000000000000000000000d41")
	reporter := common.HexToAddress("0x0000000000000000000000000000000000000e41")
	offenderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())

	oldShardState := epochRebindSlashShardState(
		oldEpoch,
		offender,
		offenderSigner.Pub.Bytes,
	)
	forgedShardState := epochRebindSlashShardState(
		forgedEpoch,
		offender,
		offenderSigner.Pub.Bytes,
	)
	chain, setupState, genesis := epochRebindSlashChain(t, oldShardState, forgedShardState)

	currentSelfStake := epochRebindSlashOnes(100_000)
	newDelegatorStake := epochRebindSlashOnes(500_000)
	currentWrapper := epochRebindSlashValidator(
		offender,
		newDelegator,
		offenderSigner.Pub.Bytes,
		currentSelfStake,
		newDelegatorStake,
		forgedEpoch,
	)

	setupState.SetValidatorFlag(offender)
	require.NoError(t, setupState.UpdateValidatorWrapper(offender, currentWrapper))
	require.NoError(t, rawdb.WriteValidatorList(setupState.Database().DiskDB(), []common.Address{offender}))
	oldSnapshot := epochRebindSlashValidator(
		offender,
		newDelegator,
		offenderSigner.Pub.Bytes,
		epochRebindSlashOnes(20_000),
		common.Big0,
		oldEpoch,
	)
	oldSnapshotWrapper := staketest.CopyValidatorWrapper(*oldSnapshot)
	require.NoError(t, chain.WriteValidatorSnapshot(
		setupState.Database().DiskDB(),
		&staking.ValidatorSnapshot{
			Validator: &oldSnapshotWrapper,
			Epoch:     new(big.Int).Set(oldEpoch),
		},
	))
	reboundSnapshotWrapper := staketest.CopyValidatorWrapper(*currentWrapper)
	require.NoError(t, chain.WriteValidatorSnapshot(
		setupState.Database().DiskDB(),
		&staking.ValidatorSnapshot{
			Validator: &reboundSnapshotWrapper,
			Epoch:     new(big.Int).Set(forgedEpoch),
		},
	))

	setupBlockNum := epochRebindSlashNextBlock(shardingconfig.MainnetSchedule.EpochLastBlock(forgedEpoch.Uint64()-1) + 1)
	setupHeader := epochRebindSlashHeader(forgedEpoch, setupBlockNum, offender, genesis.Hash())
	processor := corepkg.NewStateProcessor(chain, chain)
	_, err := epochRebindSlashProcessBlock(t, chain, processor, setupState, setupHeader)
	require.NoError(t, err)

	headState, err := chain.State()
	require.NoError(t, err)

	height := epochRebindSlashNextBlock(shardingconfig.MainnetSchedule.EpochLastBlock(oldEpoch.Uint64()-1) + 1)
	viewID := uint64(3)
	oldRecord := epochRebindSlashRecord(t, oldEpoch, height, viewID, offender, reporter, offenderSigner)
	reboundRecord := oldRecord
	reboundRecord.Evidence.Epoch = new(big.Int).Set(forgedEpoch)

	for _, vote := range []slash.Vote{oldRecord.Evidence.FirstVote, oldRecord.Evidence.SecondVote} {
		oldPayload := consensus_sig.ConstructCommitPayload(params.MainnetChainConfig, oldEpoch, vote.BlockHeaderHash, height, viewID)
		reboundPayload := consensus_sig.ConstructCommitPayload(params.MainnetChainConfig, forgedEpoch, vote.BlockHeaderHash, height, viewID)
		require.Equal(t, oldPayload, reboundPayload)
	}

	require.NoError(t, slash.Verify(chain, headState, &oldRecord))
	require.ErrorIs(t, slash.Verify(chain, headState, &reboundRecord), slash.ErrSlashEpochHeightMismatch)

	slashWire := proto_node.ConstructSlashMessage(slash.Records{reboundRecord})
	receivedRecords := slash.Records{}
	require.NoError(t, rlp.DecodeBytes(slashWire[3:], &receivedRecords))
	require.Len(t, receivedRecords, 1)
	require.NoError(t, chain.AddPendingSlashingCandidates(receivedRecords))
	require.Empty(t, chain.ReadPendingSlashingCandidates())

	beforeSlash, err := headState.ValidatorWrapper(offender, false, true)
	require.NoError(t, err)
	require.Equal(t, effective.Active, beforeSlash.Status)
	require.Zero(t, beforeSlash.Delegations[0].Amount.Cmp(currentSelfStake))
	require.Zero(t, beforeSlash.Delegations[1].Amount.Cmp(newDelegatorStake))
}

func epochRebindSlashRecord(
	t *testing.T,
	epoch *big.Int,
	height uint64,
	viewID uint64,
	offender common.Address,
	reporter common.Address,
	offenderSigner hmybls.PrivateKeyWrapper,
) slash.Record {
	t.Helper()

	firstBlock := epochRebindSlashSignedBlock(epoch, height, viewID, common.HexToHash("0x01"))
	secondBlock := epochRebindSlashSignedBlock(epoch, height, viewID, common.HexToHash("0x02"))
	require.NotEqual(t, firstBlock.Hash(), secondBlock.Hash())

	return slash.Record{
		Evidence: slash.Evidence{
			ConflictingVotes: slash.ConflictingVotes{
				FirstVote:  epochRebindSlashVote(t, firstBlock, epoch, height, viewID, offenderSigner),
				SecondVote: epochRebindSlashVote(t, secondBlock, epoch, height, viewID, offenderSigner),
			},
			Moment: slash.Moment{
				Epoch:   new(big.Int).Set(epoch),
				ShardID: shard.BeaconChainShardID,
				Height:  height,
				ViewID:  viewID,
			},
			Offender: offender,
		},
		Reporter: reporter,
	}
}

func epochRebindSlashVote(
	t *testing.T,
	block *coretypes.Block,
	epoch *big.Int,
	height uint64,
	viewID uint64,
	signer hmybls.PrivateKeyWrapper,
) slash.Vote {
	t.Helper()

	payload := consensus_sig.ConstructCommitPayload(
		params.MainnetChainConfig,
		epoch,
		block.Hash(),
		height,
		viewID,
	)
	return slash.Vote{
		SignerPubKeys:   []hmybls.SerializedPublicKey{signer.Pub.Bytes},
		BlockHeaderHash: block.Hash(),
		Signature:       signer.Pri.SignHash(payload).Serialize(),
	}
}

func epochRebindSlashSignedBlock(
	epoch *big.Int,
	height uint64,
	viewID uint64,
	root common.Hash,
) *coretypes.Block {
	header := blockfactory.ForMainnet.NewHeader(epoch).With().
		Number(new(big.Int).SetUint64(height)).
		Epoch(epoch).
		ShardID(shard.BeaconChainShardID).
		ViewID(new(big.Int).SetUint64(viewID)).
		Root(root).
		GasLimit(params.TestGenesisGasLimit).
		Header()
	return coretypes.NewBlockWithHeader(header)
}

func epochRebindSlashChain(
	t *testing.T,
	oldShardState shard.State,
	forgedShardState shard.State,
) (*corepkg.BlockChainImpl, *state.DB, *coretypes.Block) {
	t.Helper()

	genesisState := oldShardState
	genesisState.Epoch = common.Big0

	db := rawdb.NewMemoryDatabase()
	gspec := corepkg.Genesis{
		Config:     params.MainnetChainConfig,
		Factory:    blockfactory.ForMainnet,
		ShardID:    shard.BeaconChainShardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: genesisState,
	}
	genesis := gspec.MustCommit(db)

	for _, shardState := range []shard.State{oldShardState, forgedShardState} {
		encodedState, err := shard.EncodeWrapper(shardState, true)
		require.NoError(t, err)
		require.NoError(t, rawdb.WriteShardStateBytes(db, shardState.Epoch, encodedState))
	}

	chain, err := corepkg.NewBlockChain(
		db,
		nil,
		nil,
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

func epochRebindSlashShardState(
	epoch *big.Int,
	offender common.Address,
	offenderBLS hmybls.SerializedPublicKey,
) shard.State {
	stake := numeric.OneDec()
	return shard.State{
		Epoch: new(big.Int).Set(epoch),
		Shards: []shard.Committee{
			{
				ShardID: shard.BeaconChainShardID,
				Slots: shard.SlotList{
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

func epochRebindSlashValidator(
	offender common.Address,
	delegator common.Address,
	offenderBLS hmybls.SerializedPublicKey,
	selfStake *big.Int,
	delegatorStake *big.Int,
	epoch *big.Int,
) *staking.ValidatorWrapper {
	delegations := staking.Delegations{
		staking.NewDelegation(offender, new(big.Int).Set(selfStake)),
	}
	if delegatorStake.Sign() > 0 {
		delegations = append(delegations, staking.NewDelegation(delegator, new(big.Int).Set(delegatorStake)))
	}
	return &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offender,
			SlotPubKeys:          []hmybls.SerializedPublicKey{offenderBLS},
			LastEpochInCommittee: new(big.Int).Add(epoch, common.Big1),
			MinSelfDelegation:    epochRebindSlashOnes(10_000),
			MaxTotalDelegation:   epochRebindSlashOnes(1_000_000),
			Status:               effective.Active,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					Rate:          numeric.ZeroDec(),
					MaxRate:       numeric.OneDec(),
					MaxChangeRate: numeric.OneDec(),
				},
				UpdateHeight: common.Big0,
			},
			Description: staking.Description{
				Name:            "epoch-rebind-offender",
				Identity:        "epoch-rebind-offender",
				Website:         "harmony.one",
				SecurityContact: "security",
				Details:         "epoch rebind slash test",
			},
			CreationHeight: common.Big0,
		},
		Delegations: delegations,
	}
}

func epochRebindSlashHeader(
	epoch *big.Int,
	number uint64,
	coinbase common.Address,
	parentHash common.Hash,
) *block.Header {
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

func epochRebindSlashProcessBlock(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	processor *corepkg.StateProcessor,
	statedb *state.DB,
	header *block.Header,
) (*coretypes.Block, error) {
	t.Helper()

	inputBlock := coretypes.NewBlockWithHeader(header)
	receipts, outcxs, stakeMsgs, _, usedGas, payout, _, err := processor.Process(
		inputBlock,
		statedb,
		vm.Config{},
		false,
	)
	if err != nil {
		return nil, err
	}

	header.SetGasUsed(usedGas)
	header.SetRoot(statedb.IntermediateRoot(params.MainnetChainConfig.IsS3(header.Epoch())))
	finalBlock := coretypes.NewBlock(header, nil, receipts, outcxs, nil, nil)

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

	return finalBlock, nil
}

func epochRebindSlashNextBlock(start uint64) uint64 {
	blockNum := start
	for shard.Schedule.IsLastBlock(blockNum) ||
		blockNum%shardingconfig.RewardFrequency == shardingconfig.RewardFrequency-1 {
		blockNum++
	}
	return blockNum
}

func epochRebindSlashOnes(amount int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(amount), big.NewInt(denominations.One))
}
