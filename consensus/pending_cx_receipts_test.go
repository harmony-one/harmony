package consensus

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/consensus/quorum"
	corepkg "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	coretypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	hmybls "github.com/harmony-one/harmony/crypto/bls"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/multibls"
	workerpkg "github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/require"
)

// pendingCXReceiptsConfig is the mainnet config - which the block proposal path
// is known to work under - with the cross-shard forks moved to epoch 0 so the
// receipt paths are live from genesis.
func pendingCXReceiptsConfig() *params.ChainConfig {
	cfg := *params.MainnetChainConfig
	cfg.CrossTxEpoch = big.NewInt(0)
	cfg.CXMerkleProofReplayFixEpoch = big.NewInt(0)
	return &cfg
}

func pendingCXReceiptsChain(
	t *testing.T, shardID uint32, beacon corepkg.BlockChain, st shard.State,
) *corepkg.BlockChainImpl {
	t.Helper()

	db := rawdb.NewMemoryDatabase()
	gspec := corepkg.Genesis{
		Config:     pendingCXReceiptsConfig(),
		Factory:    blockfactory.ForMainnet,
		Alloc:      corepkg.GenesisAlloc{},
		ShardID:    shardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: st,
	}
	gspec.MustCommit(db)

	chain, err := corepkg.NewBlockChain(
		db, nil, beacon,
		&corepkg.CacheConfig{SnapshotLimit: 0},
		gspec.Config, chain2.NewEngine(), vm.Config{},
	)
	require.NoError(t, err)
	return chain
}

// pendingCXReceiptsHarness builds a shard-1 consensus backed by a real chain so
// that AddPendingReceipts can consult CurrentHeader/Config. The chain is grown
// past block 1 because header signature verification is skipped below that
// height, which would make every proof validate regardless of its signature.
func pendingCXReceiptsHarness(t *testing.T) *Consensus {
	t.Helper()

	beaconSigner := hmybls.RandPrivateKey()
	beaconPub := hmybls.PublicKeyWrapper{Object: beaconSigner.GetPublicKey()}
	require.NoError(t, beaconPub.Bytes.FromLibBLSPublicKey(beaconPub.Object))
	shardSigner := hmybls.RandPrivateKey()
	shardPub := hmybls.PublicKeyWrapper{Object: shardSigner.GetPublicKey()}
	require.NoError(t, shardPub.Bytes.FromLibBLSPublicKey(shardPub.Object))

	st := crossLinkCacheShardState(
		common.Big0,
		common.BytesToAddress([]byte{0xb0}),
		common.BytesToAddress([]byte{0x51}),
		beaconPub.Bytes,
		shardPub.Bytes,
	)
	beaconChain := pendingCXReceiptsChain(t, 0, nil, st)
	shardChain := pendingCXReceiptsChain(t, 1, beaconChain, st)

	txPoolConfig := corepkg.DefaultTxPoolConfig
	txPoolConfig.Journal = ""
	txPool := corepkg.NewTxPool(
		txPoolConfig, pendingCXReceiptsConfig(), shardChain,
		coretypes.NewTransactionErrorSink(),
	)
	t.Cleanup(txPool.Stop)

	reg := registry.New().
		SetBlockchain(shardChain).
		SetBeaconchain(beaconChain).
		SetTxPool(txPool).
		SetCxPool(corepkg.NewCxPool(corepkg.CxPoolSize)).
		SetWorker(workerpkg.New(shardChain, beaconChain)).
		SetAddressToBLSKey(crossLinkCacheAddressToBLSKey{shardID: shardChain.ShardID()})

	decider := quorum.NewDecider(quorum.SuperMajorityStake, shardChain.ShardID())
	consensus, err := New(
		nil, shardChain.ShardID(), multibls.GetPrivateKeys(shardSigner),
		reg, decider, 1, false,
	)
	require.NoError(t, err)
	consensus.SetLeaderPubKey(&shardPub)
	consensus.SetViewIDs(shardChain.CurrentBlock().NumberU64())

	signer := hmybls.PrivateKeyWrapper{Pri: shardSigner, Pub: &shardPub}
	var lastCommitSig []byte
	for shardChain.CurrentBlock().NumberU64() < 2 {
		blk := crossLinkCacheProposeAndInsertBeaconBlock(
			t, consensus, shardChain, signer, lastCommitSig, nil,
		)
		lastCommitSig = blk.GetCurrentCommitSig()
	}
	require.Greater(t, shardChain.CurrentHeader().Number().Uint64(), uint64(1))
	return consensus
}

// makeCXProof builds a CXReceiptsProof whose merkle proof and header agree with
// each other. Every field here is derived locally, so the proof is internally
// consistent but carries no commit signature from the source shard committee.
func makeCXProof(t *testing.T, srcShard uint32, blockNum uint64, epoch *big.Int, toShard uint32) *coretypes.CXReceiptsProof {
	t.Helper()

	to := common.BytesToAddress([]byte{0x42})
	receipts := coretypes.CXReceipts{{
		TxHash:    common.Hash{0x01},
		From:      common.BytesToAddress([]byte{0x11}),
		To:        &to,
		ShardID:   srcShard,
		ToShardID: toShard,
		Amount:    big.NewInt(1),
	}}
	shardHash := coretypes.DeriveSha(receipts)

	proof := &coretypes.CXMerkleProof{
		BlockNum:      new(big.Int).SetUint64(blockNum),
		ShardID:       srcShard,
		ShardIDs:      []uint32{toShard},
		CXShardHashes: []common.Hash{shardHash},
	}

	// Same derivation ValidateCXReceiptsProof performs over the merkle proof.
	buf := bytes.Buffer{}
	for j := range proof.ShardIDs {
		sKey := make([]byte, 4)
		binary.BigEndian.PutUint32(sKey, proof.ShardIDs[j])
		buf.Write(sKey)
		buf.Write(proof.CXShardHashes[j][:])
	}

	header := blockfactory.ForMainnet.NewHeader(epoch)
	header.SetNumber(new(big.Int).SetUint64(blockNum))
	header.SetShardID(srcShard)
	header.SetOutgoingReceiptHash(crypto.Keccak256Hash(buf.Bytes()))
	proof.CXReceiptHash = header.OutgoingReceiptHash()
	proof.BlockHash = header.Hash()

	return &coretypes.CXReceiptsProof{
		Receipts:     receipts,
		MerkleProof:  proof,
		Header:       header,
		CommitSig:    make([]byte, 96),
		CommitBitmap: []byte{0x01},
	}
}

// TestAddPendingReceiptsRejectsFutureEpoch checks that a receipts proof claiming
// an epoch far beyond the current one is not admitted to the pending pool. Only
// the epoch the chain is about to enter can legitimately lack a shard state.
func TestAddPendingReceiptsRejectsFutureEpoch(t *testing.T) {
	consensus := pendingCXReceiptsHarness(t)
	myShard := consensus.Blockchain().ShardID()
	curEpoch := consensus.Blockchain().CurrentHeader().Epoch()

	farFuture := new(big.Int).Add(curEpoch, big.NewInt(1000))
	consensus.AddPendingReceipts(makeCXProof(t, 0, 7, farFuture, myShard))
	require.Empty(t, consensus.PendingCXReceipts(),
		"a proof claiming a far future epoch should not be pending")
}
