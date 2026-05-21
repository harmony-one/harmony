package worker

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/stretchr/testify/require"
)

const (
	slashTestEpoch       = int64(2)
	slashTestBlockNumber = uint64(37)
	slashTestViewID      = uint64(38)
)

// TestVerifySlashes_RejectsReporterVariantDuplicateEvidence is a regression test for
// bug07: duplicate evidence with different reporters must not reach successes.
func TestVerifySlashes_RejectsReporterVariantDuplicateEvidence(t *testing.T) {
	offenderKey := newSlashTestBLSKeyPair(t)
	offenderAddr := slashTestAddress("offender")
	reporterA := slashTestAddress("reporter-a")
	reporterB := slashTestAddress("reporter-b")

	chain, _ := newSlashVerifyBeaconChain(t, offenderKey, offenderAddr)
	headState, err := chain.State()
	require.NoError(t, err)
	require.NoError(t, slash.Verify(chain, headState, &slash.Record{
		Evidence: slashTestEvidence(t, offenderKey, offenderAddr),
		Reporter: reporterA,
	}))

	evidence := slashTestEvidence(t, offenderKey, offenderAddr)
	records := slash.Records{
		{Evidence: evidence, Reporter: reporterA},
		{Evidence: evidence, Reporter: reporterB},
	}

	w := New(chain, chain)
	successes, failures := w.verifySlashes(records)

	require.Len(t, successes, 1, "only one evidence payload should be accepted")
	require.Len(t, failures, 1, "reporter-variant clone must be rejected as duplicate evidence")
	require.Equal(t, reporterB, failures[0].Reporter)
	require.Equal(t, hash.FromRLPNew256(evidence), hash.FromRLPNew256(successes[0].Evidence))
}

func newSlashVerifyBeaconChain(
	t *testing.T,
	offenderKey slashTestBLSKeyPair,
	offenderAddr common.Address,
) (*core.BlockChainImpl, *state.DB) {
	t.Helper()

	slashEpoch := big.NewInt(slashTestEpoch)
	genesisCommittee := slashTestCommittee(common.Big0, offenderAddr, offenderKey.pub)
	committeeAtSlashEpoch := slashTestCommittee(slashEpoch, offenderAddr, offenderKey.pub)

	db := rawdb.NewMemoryDatabase()
	gspec := core.Genesis{
		Config:     params.TestChainConfig,
		Factory:    blockfactory.ForTest,
		ShardID:    shard.BeaconChainShardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: genesisCommittee,
	}
	genesis := gspec.MustCommit(db)

	encodedCommittee, err := shard.EncodeWrapper(committeeAtSlashEpoch, true)
	require.NoError(t, err)
	require.NoError(t, rawdb.WriteShardStateBytes(db, slashEpoch, encodedCommittee))

	chain, err := core.NewBlockChain(
		db, nil, nil, &core.CacheConfig{SnapshotLimit: 0}, gspec.Config, chain2.NewEngine(), vm.Config{},
	)
	require.NoError(t, err)

	statedb, err := chain.StateAt(genesis.Root())
	require.NoError(t, err)

	validator := slashTestValidator(offenderAddr, offenderKey.pub, slashEpoch)
	require.NoError(t, statedb.UpdateValidatorWrapper(offenderAddr, validator))
	require.NoError(t, chain.WriteValidatorSnapshot(
		statedb.Database().DiskDB(),
		&staking.ValidatorSnapshot{
			Validator: validator,
			Epoch:     new(big.Int).Set(slashEpoch),
		},
	))

	processor := core.NewStateProcessor(chain, chain)
	head := slashTestHeader(slashEpoch, 1, slashTestViewID, 1, genesis.Hash(), offenderAddr)
	inputBlock := types.NewBlockWithHeader(head)
	receipts, outcxs, stakeMsgs, _, usedGas, payout, _, err := processor.Process(
		inputBlock, statedb, vm.Config{}, false,
	)
	require.NoError(t, err)

	head.SetGasUsed(usedGas)
	head.SetRoot(statedb.IntermediateRoot(params.TestChainConfig.IsS3(head.Epoch())))
	finalBlock := types.NewBlock(head, nil, receipts, outcxs, nil, nil)
	status, err := chain.WriteBlockWithState(finalBlock, receipts, outcxs, stakeMsgs, payout, statedb)
	require.NoError(t, err)
	require.Equal(t, core.CanonStatTy, status)

	return chain, statedb
}

func slashTestEvidence(
	t *testing.T,
	offenderKey slashTestBLSKeyPair,
	offenderAddr common.Address,
) slash.Evidence {
	t.Helper()

	firstBlock := slashTestBlock(big.NewInt(slashTestEpoch), slashTestBlockNumber, slashTestViewID, 0, common.HexToHash("0x01"), offenderAddr)
	secondBlock := slashTestBlock(big.NewInt(slashTestEpoch), slashTestBlockNumber, slashTestViewID, 1, common.HexToHash("0x02"), offenderAddr)
	require.NotEqual(t, firstBlock.Hash(), secondBlock.Hash())

	return slash.Evidence{
		ConflictingVotes: slash.ConflictingVotes{
			FirstVote:  slashTestVote(offenderKey, firstBlock),
			SecondVote: slashTestVote(offenderKey, secondBlock),
		},
		Moment: slash.Moment{
			Epoch:   big.NewInt(slashTestEpoch),
			ShardID: shard.BeaconChainShardID,
			Height:  slashTestBlockNumber,
			ViewID:  slashTestViewID,
		},
		Offender: offenderAddr,
	}
}

func slashTestCommittee(
	epoch *big.Int,
	offenderAddr common.Address,
	offenderPub bls.SerializedPublicKey,
) shard.State {
	stake := numeric.OneDec()
	return shard.State{
		Epoch: new(big.Int).Set(epoch),
		Shards: []shard.Committee{{
			ShardID: shard.BeaconChainShardID,
			Slots: shard.SlotList{{
				EcdsaAddress:   offenderAddr,
				BLSPublicKey:   offenderPub,
				EffectiveStake: &stake,
			}},
		}},
	}
}

func slashTestValidator(
	offender common.Address,
	offenderPub bls.SerializedPublicKey,
	epoch *big.Int,
) *staking.ValidatorWrapper {
	selfStake := new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	return &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offender,
			SlotPubKeys:          []bls.SerializedPublicKey{offenderPub},
			LastEpochInCommittee: new(big.Int).Add(epoch, common.Big1),
			MinSelfDelegation:    new(big.Int).Set(selfStake),
			MaxTotalDelegation:   new(big.Int).Mul(selfStake, common.Big2),
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
				Name: "offender", Identity: "offender", Website: "offender",
				SecurityContact: "offender", Details: "offender",
			},
			CreationHeight: common.Big0,
		},
		Delegations: staking.Delegations{{
			DelegatorAddress: offender,
			Amount:           new(big.Int).Set(selfStake),
		}},
	}
}

func slashTestHeader(
	epoch *big.Int,
	number uint64,
	viewID uint64,
	rootIndex int,
	parent common.Hash,
	coinbase common.Address,
) *block.Header {
	return blockfactory.ForTest.NewHeader(epoch).With().
		Number(new(big.Int).SetUint64(number)).
		Epoch(epoch).
		ShardID(shard.BeaconChainShardID).
		ViewID(new(big.Int).SetUint64(viewID)).
		Coinbase(coinbase).
		ParentHash(parent).
		Root(common.BigToHash(big.NewInt(int64(rootIndex)))).
		GasLimit(params.TestGenesisGasLimit).
		Header()
}

func slashTestBlock(
	epoch *big.Int,
	number uint64,
	viewID uint64,
	rootIndex int,
	parent common.Hash,
	coinbase common.Address,
) *types.Block {
	return types.NewBlockWithHeader(slashTestHeader(epoch, number, viewID, rootIndex, parent, coinbase))
}

func slashTestVote(kp slashTestBLSKeyPair, block *types.Block) slash.Vote {
	payload := consensus_sig.ConstructCommitPayload(
		params.TestChainConfig,
		block.Epoch(),
		block.Hash(),
		block.NumberU64(),
		block.Header().ViewID().Uint64(),
	)
	return slash.Vote{
		SignerPubKeys:   []bls.SerializedPublicKey{kp.pub},
		BlockHeaderHash: block.Hash(),
		Signature:       kp.pri.SignHash(payload).Serialize(),
	}
}

type slashTestBLSKeyPair struct {
	pri *bls_core.SecretKey
	pub bls.SerializedPublicKey
}

func newSlashTestBLSKeyPair(t *testing.T) slashTestBLSKeyPair {
	t.Helper()
	pri := bls.RandPrivateKey()
	pubKey := pri.GetPublicKey()
	var pub bls.SerializedPublicKey
	copy(pub[:], pubKey.Serialize())
	return slashTestBLSKeyPair{pri: pri, pub: pub}
}

func slashTestAddress(label string) common.Address {
	return common.BytesToAddress([]byte(fmt.Sprintf("harmony.slash.%s", label)))
}
