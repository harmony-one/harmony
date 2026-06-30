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
	"github.com/harmony-one/harmony/consensus/reward"
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
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
	"github.com/stretchr/testify/require"
)

func TestBeaconHeaderSlashRejectedWhenVerificationEnabled(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	attackEpoch := new(big.Int).Add(params.MainnetChainConfig.HIP32Epoch, big.NewInt(1))
	chainConfig := *params.MainnetChainConfig
	chainConfig.VerifyBeaconHeaderSlashEpoch = new(big.Int).Set(attackEpoch)

	leaderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	victimSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())

	leader := common.HexToAddress("0x0000000000000000000000000000000000000a01")
	victim := common.HexToAddress("0x0000000000000000000000000000000000000b01")
	delegator := common.HexToAddress("0x0000000000000000000000000000000000000d01")
	reporter := common.HexToAddress("0x0000000000000000000000000000000000000e01")
	leaderCoinbase := headerSlashCoinbaseAddress(leaderSigner)

	chain, setupState, genesis := newHeaderSlashChain(
		t,
		&chainConfig,
		headerSlashShardState(
			attackEpoch,
			leader,
			victim,
			leaderSigner.Pub.Bytes,
			victimSigner.Pub.Bytes,
		),
	)

	selfStake := headerSlashOnes(40_000)
	delegatorStake := headerSlashOnes(60_000)
	victimWrapper := headerSlashValidatorWrapper(
		victim,
		delegator,
		victimSigner.Pub.Bytes,
		selfStake,
		delegatorStake,
	)

	setupState.SetValidatorFlag(victim)
	require.NoError(t, setupState.UpdateValidatorWrapper(victim, victimWrapper))
	require.NoError(t, rawdb.WriteValidatorList(setupState.Database().DiskDB(), []common.Address{victim}))
	snapshotWrapper := staketest.CopyValidatorWrapper(*victimWrapper)
	require.NoError(t, rawdb.WriteValidatorSnapshot(setupState.Database().DiskDB(), &snapshotWrapper, attackEpoch))

	setupBlockNum := headerSlashNextBlock(attackEpoch)
	setupHeader := headerSlashBlockHeader(
		attackEpoch,
		setupBlockNum,
		leaderCoinbase,
		genesis.Hash(),
	)
	processor := corepkg.NewStateProcessor(chain, chain)
	setupBlock, err := headerSlashProcessAndCommitBlock(
		t,
		chain,
		processor,
		setupState,
		setupHeader,
		leaderSigner,
		victimSigner,
	)
	require.NoError(t, err)
	require.Equal(t, setupBlock.Hash(), chain.CurrentBlock().Hash())

	stateBeforeSlash, err := chain.StateAt(setupBlock.Root())
	require.NoError(t, err)
	forgedSlash := headerSlashInvalidRecord(attackEpoch, setupBlockNum, victim, reporter)
	require.ErrorContains(t, slash.Verify(chain, stateBeforeSlash, &forgedSlash), "no matching double sign keys")

	slashBytes, err := rlp.EncodeToBytes(slash.Records{forgedSlash})
	require.NoError(t, err)
	maliciousHeader := headerSlashBlockHeader(
		attackEpoch,
		setupBlockNum+1,
		leaderCoinbase,
		setupBlock.Hash(),
	)
	maliciousHeader.SetSlashes(slashBytes)

	importState, err := chain.StateAt(setupBlock.Root())
	require.NoError(t, err)
	_, _, _, _, _, _, err = headerSlashProcessBlock(processor, importState, maliciousHeader, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid beacon slash payload")
}

func TestBeaconHeaderSlashImportBeforeVerificationEpoch(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	attackEpoch := new(big.Int).Add(params.MainnetChainConfig.HIP32Epoch, big.NewInt(1))
	chainConfig := *params.MainnetChainConfig

	leaderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	victimSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())

	leader := common.HexToAddress("0x0000000000000000000000000000000000000a01")
	victim := common.HexToAddress("0x0000000000000000000000000000000000000b01")
	delegator := common.HexToAddress("0x0000000000000000000000000000000000000d01")
	reporter := common.HexToAddress("0x0000000000000000000000000000000000000e01")
	leaderCoinbase := headerSlashCoinbaseAddress(leaderSigner)

	chain, setupState, genesis := newHeaderSlashChain(
		t,
		&chainConfig,
		headerSlashShardState(
			attackEpoch,
			leader,
			victim,
			leaderSigner.Pub.Bytes,
			victimSigner.Pub.Bytes,
		),
	)

	selfStake := headerSlashOnes(40_000)
	delegatorStake := headerSlashOnes(60_000)
	victimWrapper := headerSlashValidatorWrapper(
		victim,
		delegator,
		victimSigner.Pub.Bytes,
		selfStake,
		delegatorStake,
	)

	setupState.SetValidatorFlag(victim)
	require.NoError(t, setupState.UpdateValidatorWrapper(victim, victimWrapper))
	require.NoError(t, rawdb.WriteValidatorList(setupState.Database().DiskDB(), []common.Address{victim}))
	snapshotWrapper := staketest.CopyValidatorWrapper(*victimWrapper)
	require.NoError(t, rawdb.WriteValidatorSnapshot(setupState.Database().DiskDB(), &snapshotWrapper, attackEpoch))

	setupBlockNum := headerSlashNextBlock(attackEpoch)
	setupHeader := headerSlashBlockHeader(
		attackEpoch,
		setupBlockNum,
		leaderCoinbase,
		genesis.Hash(),
	)
	processor := corepkg.NewStateProcessor(chain, chain)
	setupBlock, err := headerSlashProcessAndCommitBlock(
		t,
		chain,
		processor,
		setupState,
		setupHeader,
		leaderSigner,
		victimSigner,
	)
	require.NoError(t, err)

	forgedSlash := headerSlashInvalidRecord(attackEpoch, setupBlockNum, victim, reporter)
	slashBytes, err := rlp.EncodeToBytes(slash.Records{forgedSlash})
	require.NoError(t, err)
	maliciousHeader := headerSlashBlockHeader(
		attackEpoch,
		setupBlockNum+1,
		leaderCoinbase,
		setupBlock.Hash(),
	)
	maliciousHeader.SetSlashes(slashBytes)

	importState, err := chain.StateAt(setupBlock.Root())
	require.NoError(t, err)
	_, _, _, _, _, _, err = headerSlashProcessBlock(processor, importState, maliciousHeader, nil, nil)
	require.NoError(t, err)

	victimAfter, err := importState.ValidatorWrapper(victim, false, true)
	require.NoError(t, err)
	require.Equal(t, effective.Banned, victimAfter.Status)
}

func newHeaderSlashChain(
	t *testing.T,
	chainConfig *params.ChainConfig,
	beaconState shard.State,
) (*corepkg.BlockChainImpl, *state.DB, *coretypes.Block) {
	t.Helper()

	genesisState := beaconState
	genesisState.Epoch = common.Big0

	db := rawdb.NewMemoryDatabase()
	gspec := corepkg.Genesis{
		Config:     chainConfig,
		Factory:    blockfactory.ForMainnet,
		ShardID:    shard.BeaconChainShardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: genesisState,
	}
	genesis := gspec.MustCommit(db)

	encodedState, err := shard.EncodeWrapper(beaconState, true)
	require.NoError(t, err)
	require.NoError(t, rawdb.WriteShardStateBytes(db, beaconState.Epoch, encodedState))

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

func headerSlashProcessAndCommitBlock(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	processor *corepkg.StateProcessor,
	statedb *state.DB,
	header *block.Header,
	signers ...hmybls.PrivateKeyWrapper,
) (*coretypes.Block, error) {
	t.Helper()

	finalBlock, receipts, outcxs, stakeMsgs, usedGas, payout, err := headerSlashProcessBlock(
		processor,
		statedb,
		header,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}
	if len(signers) > 0 {
		finalBlock.SetCurrentCommitSig(headerSlashCommitSig(t, finalBlock, signers...))
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
	require.Equal(t, usedGas, finalBlock.GasUsed())
	return finalBlock, nil
}

func headerSlashProcessBlock(
	processor *corepkg.StateProcessor,
	statedb *state.DB,
	header *block.Header,
	txs []*coretypes.Transaction,
	incxs []*coretypes.CXReceiptsProof,
) (*coretypes.Block, coretypes.Receipts, coretypes.CXReceipts, []staking.StakeMsg, uint64, reward.Reader, error) {
	inputBlock := coretypes.NewBlockWithHeader(header).WithBody(txs, nil, nil, incxs)
	receipts, outcxs, stakeMsgs, _, usedGas, payout, newState, err := processor.Process(
		inputBlock,
		statedb,
		vm.Config{},
		false,
	)
	if err != nil {
		return nil, nil, nil, nil, 0, nil, err
	}

	header.SetGasUsed(usedGas)
	header.SetRoot(newState.IntermediateRoot(params.MainnetChainConfig.IsS3(header.Epoch())))
	finalBlock := coretypes.NewBlock(header, txs, receipts, outcxs, incxs, nil)
	return finalBlock, receipts, outcxs, stakeMsgs, usedGas, payout, nil
}

func headerSlashCommitSig(
	t *testing.T,
	block *coretypes.Block,
	signers ...hmybls.PrivateKeyWrapper,
) []byte {
	t.Helper()

	publics := make([]hmybls.PublicKeyWrapper, 0, len(signers))
	signatures := make([]*bls_core.Sign, 0, len(signers))
	for _, signer := range signers {
		publics = append(publics, *signer.Pub)
	}
	mask := hmybls.NewMask(publics)

	payload := consensus_sig.ConstructCommitPayload(
		params.MainnetChainConfig,
		block.Epoch(),
		block.Hash(),
		block.NumberU64(),
		block.Header().ViewID().Uint64(),
	)
	for i, signer := range signers {
		require.NoError(t, mask.SetBit(i, true))
		signatures = append(signatures, signer.Pri.SignHash(payload))
	}
	aggSig := hmybls.AggregateSig(signatures)

	return append(aggSig.Serialize(), mask.Mask()...)
}

func headerSlashInvalidRecord(epoch *big.Int, height uint64, offender common.Address, reporter common.Address) slash.Record {
	return slash.Record{
		Evidence: slash.Evidence{
			Moment: slash.Moment{
				Epoch:   new(big.Int).Set(epoch),
				ShardID: shard.BeaconChainShardID,
				Height:  height,
				ViewID:  38,
			},
			ConflictingVotes: slash.ConflictingVotes{
				FirstVote: slash.Vote{
					BlockHeaderHash: common.HexToHash("0x01"),
				},
				SecondVote: slash.Vote{
					BlockHeaderHash: common.HexToHash("0x02"),
				},
			},
			Offender: offender,
		},
		Reporter: reporter,
	}
}

func headerSlashValidatorWrapper(
	validator common.Address,
	delegator common.Address,
	slotKey hmybls.SerializedPublicKey,
	selfStake *big.Int,
	delegatorStake *big.Int,
) *staking.ValidatorWrapper {
	return &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              validator,
			SlotPubKeys:          []hmybls.SerializedPublicKey{slotKey},
			LastEpochInCommittee: big.NewInt(1),
			MinSelfDelegation:    headerSlashOnes(10_000),
			MaxTotalDelegation:   headerSlashOnes(200_000),
			Status:               effective.Active,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					Rate:          numeric.ZeroDec(),
					MaxRate:       numeric.OneDec(),
					MaxChangeRate: numeric.OneDec(),
				},
				UpdateHeight: big.NewInt(1),
			},
			Description: staking.Description{
				Name:            "header-slash-victim",
				Identity:        "header-slash-victim",
				Website:         "harmony.one",
				SecurityContact: "security",
				Details:         "header slash validation test",
			},
			CreationHeight: big.NewInt(1),
		},
		Delegations: staking.Delegations{
			staking.NewDelegation(validator, new(big.Int).Set(selfStake)),
			staking.NewDelegation(delegator, new(big.Int).Set(delegatorStake)),
		},
		BlockReward: big.NewInt(0),
	}
}

func headerSlashShardState(
	epoch *big.Int,
	leader common.Address,
	victim common.Address,
	leaderBLS hmybls.SerializedPublicKey,
	victimBLS hmybls.SerializedPublicKey,
) shard.State {
	leaderStake := numeric.OneDec()
	victimStake := numeric.OneDec()
	return shard.State{
		Epoch: new(big.Int).Set(epoch),
		Shards: []shard.Committee{
			{
				ShardID: shard.BeaconChainShardID,
				Slots: shard.SlotList{
					{
						EcdsaAddress:   leader,
						BLSPublicKey:   leaderBLS,
						EffectiveStake: &leaderStake,
					},
					{
						EcdsaAddress:   victim,
						BLSPublicKey:   victimBLS,
						EffectiveStake: &victimStake,
					},
				},
			},
		},
	}
}

func headerSlashCoinbaseAddress(signer hmybls.PrivateKeyWrapper) common.Address {
	var coinbase common.Address
	blsAddress := signer.Pub.Object.GetAddress()
	coinbase.SetBytes(blsAddress[:])
	return coinbase
}

func headerSlashNextBlock(epoch *big.Int) uint64 {
	blockNum := shardingconfig.MainnetSchedule.EpochLastBlock(epoch.Uint64()-1) + 1
	for shard.Schedule.IsLastBlock(blockNum+1) ||
		(blockNum+1)%shardingconfig.RewardFrequency == shardingconfig.RewardFrequency-1 {
		blockNum++
	}
	return blockNum
}

func headerSlashBlockHeader(
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

func headerSlashOnes(amount int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(amount), big.NewInt(denominations.One))
}
