// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package core implements the Ethereum consensus protocol.
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	bls2 "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/tikv"
	"github.com/harmony-one/harmony/internal/tikv/redis_helper"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"

	"github.com/harmony-one/harmony/hmy/tracers"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/apr"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	lru "github.com/hashicorp/golang-lru"
)

var (
	headBlockGauge     = metrics.NewRegisteredGauge("chain/head/block", nil)
	headHeaderGauge    = metrics.NewRegisteredGauge("chain/head/header", nil)
	headFastBlockGauge = metrics.NewRegisteredGauge("chain/head/receipt", nil)

	accountReadTimer   = metrics.NewRegisteredTimer("chain/account/reads", nil)
	accountHashTimer   = metrics.NewRegisteredTimer("chain/account/hashes", nil)
	accountUpdateTimer = metrics.NewRegisteredTimer("chain/account/updates", nil)
	accountCommitTimer = metrics.NewRegisteredTimer("chain/account/commits", nil)

	storageReadTimer   = metrics.NewRegisteredTimer("chain/storage/reads", nil)
	storageHashTimer   = metrics.NewRegisteredTimer("chain/storage/hashes", nil)
	storageUpdateTimer = metrics.NewRegisteredTimer("chain/storage/updates", nil)
	storageCommitTimer = metrics.NewRegisteredTimer("chain/storage/commits", nil)

	blockInsertTimer     = metrics.NewRegisteredTimer("chain/inserts", nil)
	blockValidationTimer = metrics.NewRegisteredTimer("chain/validation", nil)
	blockExecutionTimer  = metrics.NewRegisteredTimer("chain/execution", nil)
	blockWriteTimer      = metrics.NewRegisteredTimer("chain/write", nil)

	// ErrNoGenesis is the error when there is no genesis.
	ErrNoGenesis           = errors.New("Genesis not found in chain")
	ErrEmptyChain          = errors.New("empty chain")
	ErrNotLastBlockInEpoch = errors.New("not last block in epoch")
	// errExceedMaxPendingSlashes ..
	errExceedMaxPendingSlashes = errors.New("exceeed max pending slashes")
	errNilEpoch                = errors.New("nil epoch for voting power computation")
	errAlreadyExist            = errors.New("crosslink already exist")
	errDoubleSpent             = errors.New("[verifyIncomingReceipts] Double Spent")
)

const (
	bodyCacheLimit                     = 128
	blockCacheLimit                    = 128
	receiptsCacheLimit                 = 32
	badBlockLimit                      = 10
	triesInRedis                       = 1000
	shardCacheLimit                    = 10
	commitsCacheLimit                  = 10
	epochCacheLimit                    = 10
	randomnessCacheLimit               = 10
	validatorCacheLimit                = 128
	validatorStatsCacheLimit           = 128
	validatorListCacheLimit            = 10
	validatorListByDelegatorCacheLimit = 128
	pendingCrossLinksCacheLimit        = 2
	blockAccumulatorCacheLimit         = 64
	leaderPubKeyFromCoinbaseLimit      = 8
	maxPendingSlashes                  = 256
	// BlockChainVersion ensures that an incompatible database forces a resync from scratch.
	BlockChainVersion = 3
	pendingCLCacheKey = "pendingCLs"
)

// CacheConfig contains the configuration values for the trie caching/pruning
// that's resident in a blockchain.
type CacheConfig struct {
	Disabled          bool          // Whether to disable trie write caching (archive node)
	TrieNodeLimit     int           // Memory limit (MB) at which to flush the current in-memory trie to disk
	TrieTimeLimit     time.Duration // Time limit after which to flush the current in-memory trie to disk
	TriesInMemory     uint64        // Block number from the head stored in disk before exiting
	TrieDirtyLimit    int           // Memory limit (MB) at which to start flushing dirty trie nodes to disk
	TrieDirtyDisabled bool          // Whether to disable trie write caching and GC altogether (archive node)
	TrieCleanLimit    int           // Memory allowance (MB) to use for caching trie nodes in memory
	TrieCleanJournal  string        // Disk journal for saving clean cache entries.
	Preimages         bool          // Whether to store preimage of trie key to the disk
	SnapshotLimit     int           // Memory allowance (MB) to use for caching snapshot entries in memory
	SnapshotNoBuild   bool          // Whether the background generation is allowed
	SnapshotWait      bool          // Wait for snapshot construction on startup. TODO(karalabe): This is a dirty hack for testing, nuke it
}

// defaultCacheConfig are the default caching values if none are specified by the
// user (also used during testing).
var defaultCacheConfig = &CacheConfig{
	Disabled:       false,
	TrieCleanLimit: 256,
	TrieDirtyLimit: 256,
	TrieTimeLimit:  5 * time.Minute,
	SnapshotLimit:  256,
	SnapshotWait:   true,
	Preimages:      true,
}

type BlockChainImpl struct {
	chainConfig            *params.ChainConfig // Chain & network configuration
	cacheConfig            *CacheConfig        // Cache configuration for pruning
	pruneBeaconChainEnable bool                // pruneBeaconChainEnable is enable prune BeaconChain feature
	shardID                uint32              // Shard number

	db     ethdb.Database                   // Low level persistent database to store final content in
	snaps  *snapshot.Tree                   // Snapshot tree for fast trie leaf access
	triegc *prque.Prque[int64, common.Hash] // Priority queue mapping block numbers to tries to gc
	gcproc time.Duration                    // Accumulates canonical block processing for trie dumping
	triedb *trie.Database                   // The database handler for maintaining trie nodes.

	// The following two variables are used to clean up the cache of redis in tikv mode.
	// This can improve the cache hit rate of redis
	latestCleanCacheNum uint64      // The most recently cleaned cache of block num
	cleanCacheChan      chan uint64 // Used to notify blocks that will be cleaned up

	// redisPreempt used in tikv mode, write nodes preempt for write permissions and publish updates to reader nodes
	redisPreempt *redis_helper.RedisPreempt

	hc            *HeaderChain
	trace         bool       // atomic?
	traceFeed     event.Feed // send trace_block result to explorer
	rmLogsFeed    event.Feed
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

	chainmu                     sync.RWMutex // blockchain insertion lock
	pendingCrossLinksMutex      sync.RWMutex // pending crosslinks lock
	pendingSlashingCandidatesMU sync.RWMutex // pending slashing candidates

	currentBlock     atomic.Value // Current head of the block chain
	currentFastBlock atomic.Value // Current head of the fast-sync chain (may be above the block chain!)

	stateCache                    state.Database // State database to reuse between imports (contains state cache)
	bodyCache                     *lru.Cache     // Cache for the most recent block bodies
	bodyRLPCache                  *lru.Cache     // Cache for the most recent block bodies in RLP encoded format
	receiptsCache                 *lru.Cache     // Cache for the most recent receipts per block
	blockCache                    *lru.Cache     // Cache for the most recent entire blocks
	shardStateCache               *lru.Cache
	lastCommitsCache              *lru.Cache
	epochCache                    *lru.Cache        // Cache epoch number → first block number
	randomnessCache               *lru.Cache        // Cache for vrf/vdf
	validatorSnapshotCache        *lru.Cache        // Cache for validator snapshot
	validatorStatsCache           *lru.Cache        // Cache for validator stats
	validatorListCache            *lru.Cache        // Cache of validator list
	validatorListByDelegatorCache *lru.Cache        // Cache of validator list by delegator
	pendingCrossLinksCache        *lru.Cache        // Cache of last pending crosslinks
	blockAccumulatorCache         *lru.Cache        // Cache of block accumulators
	leaderPubKeyFromCoinbase      *lru.Cache        // Cache of leader public key from coinbase
	quit                          chan struct{}     // blockchain quit channel
	running                       int32             // running must be called atomically
	blockchainPruner              *blockchainPruner // use to prune beacon chain
	// procInterrupt must be atomically called
	procInterrupt int32 // interrupt signaler for block processing

	engine                 consensus_engine.Engine
	processor              *StateProcessor // block processor interface
	validator              *BlockValidator
	vmConfig               vm.Config
	badBlocks              *lru.Cache // Bad block cache
	pendingSlashes         slash.Records
	maxGarbCollectedBlkNum int64
	leaderRotationMeta     LeaderRotationMeta

	options Options
}

// NewBlockChainWithOptions same as NewBlockChain but can accept additional behaviour options.
func NewBlockChainWithOptions(
	db ethdb.Database, stateCache state.Database, beaconChain BlockChain, cacheConfig *CacheConfig, chainConfig *params.ChainConfig,
	engine consensus_engine.Engine, vmConfig vm.Config, options Options,
) (*BlockChainImpl, error) {
	return newBlockChainWithOptions(db, stateCache, beaconChain, cacheConfig, chainConfig, engine, vmConfig, options)
}

// NewBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Ethereum validator and
// Processor. As of Aug-23, this is only used by tests
func NewBlockChain(
	db ethdb.Database, stateCache state.Database, beaconChain BlockChain, cacheConfig *CacheConfig, chainConfig *params.ChainConfig,
	engine consensus_engine.Engine, vmConfig vm.Config,
) (*BlockChainImpl, error) {
	return newBlockChainWithOptions(db, stateCache, beaconChain, cacheConfig, chainConfig, engine, vmConfig, Options{})
}

func newBlockChainWithOptions(
	db ethdb.Database, stateCache state.Database, beaconChain BlockChain,
	cacheConfig *CacheConfig, chainConfig *params.ChainConfig,
	engine consensus_engine.Engine, vmConfig vm.Config, options Options) (*BlockChainImpl, error) {

	if cacheConfig == nil {
		cacheConfig = defaultCacheConfig
	}

	// Open trie database with provided config
	triedb := trie.NewDatabaseWithConfig(db, &trie.Config{
		Cache:     cacheConfig.TrieCleanLimit,
		Journal:   cacheConfig.TrieCleanJournal,
		Preimages: cacheConfig.Preimages,
	})

	if stateCache == nil {
		stateCache = state.NewDatabaseWithNodeDB(db, triedb)
	}

	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	receiptsCache, _ := lru.New(receiptsCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	badBlocks, _ := lru.New(badBlockLimit)
	shardCache, _ := lru.New(shardCacheLimit)
	commitsCache, _ := lru.New(commitsCacheLimit)
	epochCache, _ := lru.New(epochCacheLimit)
	randomnessCache, _ := lru.New(randomnessCacheLimit)
	validatorCache, _ := lru.New(validatorCacheLimit)
	validatorStatsCache, _ := lru.New(validatorStatsCacheLimit)
	validatorListCache, _ := lru.New(validatorListCacheLimit)
	validatorListByDelegatorCache, _ := lru.New(validatorListByDelegatorCacheLimit)
	pendingCrossLinksCache, _ := lru.New(pendingCrossLinksCacheLimit)
	blockAccumulatorCache, _ := lru.New(blockAccumulatorCacheLimit)
	leaderPubKeyFromCoinbase, _ := lru.New(leaderPubKeyFromCoinbaseLimit)

	bc := &BlockChainImpl{
		chainConfig:                   chainConfig,
		cacheConfig:                   cacheConfig,
		db:                            db,
		triegc:                        prque.New[int64, common.Hash](nil),
		triedb:                        triedb,
		stateCache:                    stateCache,
		quit:                          make(chan struct{}),
		bodyCache:                     bodyCache,
		bodyRLPCache:                  bodyRLPCache,
		receiptsCache:                 receiptsCache,
		blockCache:                    blockCache,
		shardStateCache:               shardCache,
		lastCommitsCache:              commitsCache,
		epochCache:                    epochCache,
		randomnessCache:               randomnessCache,
		validatorSnapshotCache:        validatorCache,
		validatorStatsCache:           validatorStatsCache,
		validatorListCache:            validatorListCache,
		validatorListByDelegatorCache: validatorListByDelegatorCache,
		pendingCrossLinksCache:        pendingCrossLinksCache,
		blockAccumulatorCache:         blockAccumulatorCache,
		leaderPubKeyFromCoinbase:      leaderPubKeyFromCoinbase,
		blockchainPruner:              newBlockchainPruner(db),
		engine:                        engine,
		vmConfig:                      vmConfig,
		badBlocks:                     badBlocks,
		pendingSlashes:                slash.Records{},
		maxGarbCollectedBlkNum:        -1,
		options:                       options,
	}

	var err error
	bc.hc, err = NewHeaderChain(db, chainConfig, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	var nilBlock *types.Block
	bc.currentBlock.Store(nilBlock)
	bc.currentFastBlock.Store(nilBlock)
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
	bc.shardID = bc.CurrentBlock().ShardID()
	if beaconChain == nil && bc.shardID == shard.BeaconChainShardID {
		beaconChain = bc
	}

	bc.validator = NewBlockValidator(bc)
	bc.processor = NewStateProcessor(bc, beaconChain)

	// Load any existing snapshot, regenerating it if loading failed
	if bc.cacheConfig.SnapshotLimit > 0 {
		// If the chain was rewound past the snapshot persistent layer (causing
		// a recovery block number to be persisted to disk), check if we're still
		// in recovery mode and in that case, don't invalidate the snapshot on a
		// head mismatch.
		var recover bool

		head := bc.CurrentBlock()
		if layer := rawdb.ReadSnapshotRecoveryNumber(bc.db); layer != nil && *layer >= head.NumberU64() {
			utils.Logger().Warn().Uint64("diskbase", *layer).Uint64("chainhead", head.NumberU64()).Msg("Enabling snapshot recovery")
			recover = true
		}
		snapconfig := snapshot.Config{
			CacheSize:  bc.cacheConfig.SnapshotLimit,
			Recovery:   recover,
			NoBuild:    bc.cacheConfig.SnapshotNoBuild,
			AsyncBuild: !bc.cacheConfig.SnapshotWait,
		}
		fmt.Println("loading/generating snapshot...")
		utils.Logger().Info().
			Str("Root", head.Root().Hex()).
			Msg("loading/generating snapshot")
		bc.snaps, _ = snapshot.New(snapconfig, bc.db, bc.triedb, head.Root())
	}

	curHeader := bc.CurrentBlock().Header()
	err = bc.buildLeaderRotationMeta(curHeader)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to build leader rotation meta")
	}

	if cacheConfig.Preimages {
		if _, _, err := rawdb.WritePreImageStartEndBlock(bc.ChainDb(), curHeader.NumberU64()+1, 0); err != nil {
			return nil, errors.WithMessage(err, "failed to write pre-image start end blocks")
		}
	}
	return bc, nil
}

// VerifyBlockCrossLinks verifies the crosslinks of the block.
// This function should be called from beacon chain.
func VerifyBlockCrossLinks(blockchain BlockChain, block *types.Block) error {
	cxLinksData := block.Header().CrossLinks()
	if len(cxLinksData) == 0 {
		utils.Logger().Debug().Msgf("[CrossLinkVerification] Zero CrossLinks in the header")
		return nil
	}

	crossLinks := types.CrossLinks{}
	err := rlp.DecodeBytes(cxLinksData, &crossLinks)
	if err != nil {
		return errors.Wrapf(
			err, "[CrossLinkVerification] failed to decode cross links",
		)
	}

	if !crossLinks.IsSorted() {
		return errors.New("[CrossLinkVerification] cross links are not sorted")
	}

	for _, crossLink := range crossLinks {
		// ReadCrossLink beacon chain usage.
		cl, err := blockchain.ReadCrossLink(crossLink.ShardID(), crossLink.BlockNum())
		if err == nil && cl != nil {
			utils.Logger().Err(errAlreadyExist).
				Uint64("beacon-block-number", block.NumberU64()).
				Interface("remote", crossLink).
				Interface("local", cl).
				Msg("[CrossLinkVerification]")
			// TODO Add slash for exist same blocknum but different crosslink
			return errors.Wrapf(
				errAlreadyExist,
				"[CrossLinkVerification] shard: %d block: %d on beacon block %d",
				crossLink.ShardID(),
				crossLink.BlockNum(),
				block.NumberU64(),
			)
		}
		if err := VerifyCrossLink(blockchain, crossLink); err != nil {
			return errors.Wrapf(err, "cannot VerifyBlockCrossLinks")
		}
	}
	return nil
}

// VerifyCrossLink verifies the header is valid
func VerifyCrossLink(blockchain BlockChain, cl types.CrossLink) error {
	if blockchain.ShardID() != shard.BeaconChainShardID {
		return errors.New("[VerifyCrossLink] Shard chains should not verify cross links")
	}
	engine := blockchain.Engine()

	if err := engine.VerifyCrossLink(blockchain, cl); err != nil {
		return errors.Wrap(err, "[VerifyCrossLink]")
	}
	return nil
}

func VerifyIncomingReceipts(blockchain BlockChain, block *types.Block) error {
	m := make(map[common.Hash]struct{})
	cxps := block.IncomingReceipts()
	for _, cxp := range cxps {
		// double spent
		if blockchain.IsSpent(cxp) {
			return errDoubleSpent
		}
		hash := cxp.MerkleProof.BlockHash
		// duplicated receipts
		if _, ok := m[hash]; ok {
			return errDoubleSpent
		}
		m[hash] = struct{}{}

		for _, item := range cxp.Receipts {
			if s := blockchain.ShardID(); item.ToShardID != s {
				return errors.Errorf(
					"[verifyIncomingReceipts] Invalid ToShardID %d expectShardID %d",
					s, item.ToShardID,
				)
			}
		}

		if err := NewBlockValidator(blockchain).ValidateCXReceiptsProof(cxp); err != nil {
			return errors.Wrapf(err, "[verifyIncomingReceipts] verification failed")
		}
	}

	incomingReceiptHash := types.EmptyRootHash
	if len(cxps) > 0 {
		incomingReceiptHash = types.DeriveSha(cxps)
	}
	if incomingReceiptHash != block.Header().IncomingReceiptHash() {
		return errors.New("[verifyIncomingReceipts] Invalid IncomingReceiptHash in block header")
	}

	return nil
}

func (bc *BlockChainImpl) ValidateNewBlock(block *types.Block, beaconChain BlockChain) error {
	if block == nil || block.Header() == nil {
		return errors.New("nil header or block asked to verify")
	}

	if block.ShardID() != bc.ShardID() {
		utils.Logger().Error().
			Uint32("my shard ID", bc.ShardID()).
			Uint32("new block's shard ID", block.ShardID()).
			Msg("[ValidateNewBlock] Wrong shard ID of the new block")
		return errors.New("[ValidateNewBlock] Wrong shard ID of the new block")
	}

	if block.NumberU64() <= bc.CurrentBlock().NumberU64() {
		return errors.Errorf("block with the same block number is already committed: %d", block.NumberU64())
	}
	if err := bc.validator.ValidateHeader(block, true); err != nil {
		utils.Logger().Error().
			Str("blockHash", block.Hash().Hex()).
			Err(err).
			Msg("[ValidateNewBlock] Cannot validate header for the new block")
		return err
	}
	if err := bc.Engine().VerifyVRF(
		bc, block.Header(),
	); err != nil {
		utils.Logger().Error().
			Uint64("blockNum", block.NumberU64()).
			Str("blockHash", block.Hash().Hex()).
			Err(err).
			Msg("[ValidateNewBlock] Cannot verify vrf for the new block")
		return errors.Wrap(err,
			"[ValidateNewBlock] Cannot verify vrf for the new block",
		)
	}
	err := bc.Engine().VerifyShardState(bc, beaconChain, block.Header())
	if err != nil {
		utils.Logger().Error().
			Str("blockHash", block.Hash().Hex()).
			Err(err).
			Msg("[ValidateNewBlock] Cannot verify shard state for the new block")
		return errors.Wrap(err,
			"[ValidateNewBlock] Cannot verify shard state for the new block",
		)
	}
	err = bc.validateNewBlock(block)
	if err != nil {
		return err
	}
	if bc.shardID == shard.BeaconChainShardID {
		err = VerifyBlockCrossLinks(bc, block)
		if err != nil {
			utils.Logger().Debug().Err(err).Msg("ops2 VerifyBlockCrossLinks Failed")
			return err
		}
	}

	return VerifyIncomingReceipts(bc, block)
}

func (bc *BlockChainImpl) validateNewBlock(block *types.Block) error {
	state, err := state.New(bc.CurrentBlock().Root(), bc.stateCache, bc.snaps)
	if err != nil {
		return err
	}

	// NOTE Order of mutating state here matters.
	// Process block using the parent state as reference point.
	// Do not read cache from processor.
	receipts, cxReceipts, _, _, usedGas, _, _, err := bc.processor.Process(
		block, state, bc.vmConfig, false,
	)
	if err != nil {
		bc.reportBlock(block, receipts, err)
		return err
	}

	// Verify all the hash roots (state, txns, receipts, cross-shard)
	if err := bc.validator.ValidateState(
		block, state, receipts, cxReceipts, usedGas,
	); err != nil {
		bc.reportBlock(block, receipts, err)
		return err
	}
	return nil
}

// IsEpochBlock returns whether this block is the first block of an epoch.
// by checking if the previous block is the last block of the previous epoch
func IsEpochBlock(block *types.Block) bool {
	if block.NumberU64() == 0 {
		// genesis block is the first epoch block
		return true
	}
	return shard.Schedule.IsLastBlock(block.NumberU64() - 1)
}

func (bc *BlockChainImpl) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *BlockChainImpl) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		utils.Logger().Warn().Msg("Empty database, resetting chain")
		return bc.reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		utils.Logger().Warn().Str("hash", head.Hex()).Msg("Head block missing, resetting chain")
		return bc.reset()
	}
	// Make sure the state associated with the block is available
	if _, err := state.New(currentBlock.Root(), bc.stateCache, bc.snaps); err != nil {
		// Dangling block without a state associated, init from scratch
		utils.Logger().Warn().
			Str("number", currentBlock.Number().String()).
			Str("hash", currentBlock.Hash().Hex()).
			Msg("Head state missing, repairing chain")
		if err := bc.repair(&currentBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	bc.latestCleanCacheNum = currentBlock.NumberU64() - triesInRedis
	bc.currentBlock.Store(currentBlock)
	headBlockGauge.Update(int64(currentBlock.NumberU64()))

	// We don't need the following as we want the current header and block to be consistent
	// Restore the last known head header
	//currentHeader := currentBlock.Header()
	//if head := rawdb.ReadHeadHeaderHash(bc.db); head != (common.Hash{}) {
	//	if header := bc.GetHeaderByHash(head); header != nil {
	//		currentHeader = header
	//	}
	//}
	currentHeader := currentBlock.Header()
	if err := bc.hc.SetCurrentHeader(currentHeader); err != nil {
		return errors.Wrap(err, "headerChain SetCurrentHeader")
	}

	// Restore the last known head fast block
	bc.currentFastBlock.Store(currentBlock)
	headFastBlockGauge.Update(int64(currentBlock.NumberU64()))
	if head := rawdb.ReadHeadFastBlockHash(bc.db); head != (common.Hash{}) {
		if block := bc.GetBlockByHash(head); block != nil {
			bc.currentFastBlock.Store(block)
			headFastBlockGauge.Update(int64(block.NumberU64()))
		}
	}

	// Issue a status log for the user
	currentFastBlock := bc.CurrentFastBlock()

	headerTd := bc.GetTd(currentHeader.Hash(), currentHeader.Number().Uint64())
	blockTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	fastTd := bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64())

	utils.Logger().Info().
		Str("number", currentHeader.Number().String()).
		Str("hash", currentHeader.Hash().Hex()).
		Str("td", headerTd.String()).
		Str("age", common.PrettyAge(time.Unix(currentHeader.Time().Int64(), 0)).String()).
		Msg("Loaded most recent local header")
	utils.Logger().Info().
		Str("number", currentBlock.Number().String()).
		Str("hash", currentBlock.Hash().Hex()).
		Str("td", blockTd.String()).
		Str("age", common.PrettyAge(time.Unix(currentBlock.Time().Int64(), 0)).String()).
		Msg("Loaded most recent local full block")
	utils.Logger().Info().
		Str("number", currentFastBlock.Number().String()).
		Str("hash", currentFastBlock.Hash().Hex()).
		Str("td", fastTd.String()).
		Str("age", common.PrettyAge(time.Unix(currentFastBlock.Time().Int64(), 0)).String()).
		Msg("Loaded most recent local fast block")

	return nil
}

func (bc *BlockChainImpl) setHead(head uint64) error {
	utils.Logger().Warn().Uint64("target", head).Msg("Rewinding blockchain")

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db rawdb.DatabaseDeleter, hash common.Hash, num uint64) error {
		return rawdb.DeleteBody(db, hash, num)
	}
	if err := bc.hc.SetHead(head, delFn); err != nil {
		return errors.Wrap(err, "headerChain SetHeader")
	}
	currentHeader := bc.hc.CurrentHeader()

	// Clear out any stale content from the caches
	bc.bodyCache.Purge()
	bc.bodyRLPCache.Purge()
	bc.receiptsCache.Purge()
	bc.blockCache.Purge()
	bc.shardStateCache.Purge()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentHeader.Number().Uint64() < currentBlock.NumberU64() {
		newHeadBlock := bc.GetBlock(currentHeader.Hash(), currentHeader.Number().Uint64())
		bc.currentBlock.Store(newHeadBlock)
		headBlockGauge.Update(int64(newHeadBlock.NumberU64()))
	}
	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(currentBlock.Root(), bc.stateCache, bc.snaps); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			bc.currentBlock.Store(bc.genesisBlock)
			headBlockGauge.Update(int64(bc.genesisBlock.NumberU64()))
		}
	}
	// Rewind the fast block in a simpleton way to the target head
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentHeader.Number().Uint64() < currentFastBlock.NumberU64() {
		newHeadFastBlock := bc.GetBlock(currentHeader.Hash(), currentHeader.Number().Uint64())
		bc.currentFastBlock.Store(newHeadFastBlock)
		headFastBlockGauge.Update(int64(newHeadFastBlock.NumberU64()))
	}
	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
		headBlockGauge.Update(int64(bc.genesisBlock.NumberU64()))
	}
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock == nil {
		bc.currentFastBlock.Store(bc.genesisBlock)
		headFastBlockGauge.Update(int64(bc.genesisBlock.NumberU64()))
	}
	currentBlock := bc.CurrentBlock()
	currentFastBlock := bc.CurrentFastBlock()

	if err := rawdb.WriteHeadBlockHash(bc.db, currentBlock.Hash()); err != nil {
		return err
	}
	if err := rawdb.WriteHeadFastBlockHash(bc.db, currentFastBlock.Hash()); err != nil {
		return err
	}

	return bc.loadLastState()
}

func (bc *BlockChainImpl) ShardID() uint32 {
	return bc.shardID
}

func (bc *BlockChainImpl) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// CurrentFastBlock retrieves the current fast-sync head block of the canonical
// chain. The block is retrieved from the blockchain's internal cache.
func (bc *BlockChainImpl) CurrentFastBlock() *types.Block {
	return bc.currentFastBlock.Load().(*types.Block)
}

// Validator returns the current validator.
func (bc *BlockChainImpl) Validator() Validator {
	return bc.validator
}

func (bc *BlockChainImpl) Processor() Processor {
	return bc.processor
}

func (bc *BlockChainImpl) State() (*state.DB, error) {
	return bc.StateAt(bc.CurrentBlock().Root())
}

func (bc *BlockChainImpl) StateAt(root common.Hash) (*state.DB, error) {
	return state.New(root, bc.stateCache, bc.snaps)
}

// Snapshots returns the blockchain snapshot tree.
func (bc *BlockChainImpl) Snapshots() *snapshot.Tree {
	return bc.snaps
}

func (bc *BlockChainImpl) reset() error {
	return bc.resetWithGenesisBlock(bc.genesisBlock)
}

func (bc *BlockChainImpl) resetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := bc.setHead(0); err != nil {
		return err
	}

	// Prepare the genesis block and reinitialise the chain
	if err := rawdb.WriteBlock(bc.db, genesis); err != nil {
		return err
	}

	bc.genesisBlock = genesis
	if err := bc.insert(bc.genesisBlock); err != nil {
		return err
	}
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	if err := bc.hc.SetCurrentHeader(bc.genesisBlock.Header()); err != nil {
		return err
	}
	bc.currentBlock.Store(bc.genesisBlock)
	headBlockGauge.Update(int64(bc.genesisBlock.NumberU64()))
	bc.currentFastBlock.Store(bc.genesisBlock)
	headFastBlockGauge.Update(int64(bc.genesisBlock.NumberU64()))
	return nil
}

func (bc *BlockChainImpl) repairRecreateStateTries(head **types.Block) error {
	for {
		blk := bc.GetBlockByNumber((*head).NumberU64() + 1)
		if blk != nil {
			_, _, _, err := bc.insertChain([]*types.Block{blk}, true)
			if err != nil {
				return err
			}
			*head = blk
			continue
		}
	}
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *BlockChainImpl) repair(head **types.Block) error {
	if err := bc.repairValidatorsAndCommitSigs(head); err != nil {
		return errors.WithMessage(err, "failed to repair validators and commit sigs")
	}
	if err := bc.repairRecreateStateTries(head); err != nil {
		return errors.WithMessage(err, "failed to recreate state tries")
	}
	return nil
}

func (bc *BlockChainImpl) repairValidatorsAndCommitSigs(head **types.Block) error {
	valsToRemove := map[common.Address]struct{}{}
	for {
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New((*head).Root(), bc.stateCache, bc.snaps); err == nil {
			utils.Logger().Info().
				Str("number", (*head).Number().String()).
				Str("hash", (*head).Hash().Hex()).
				Msg("Rewound blockchain to past state")
			if err := rawdb.WriteHeadBlockHash(bc.db, (*head).Hash()); err != nil {
				return errors.WithMessagef(err, "failed to write head block hash number %d", (*head).NumberU64())
			}
			return bc.removeInValidatorList(valsToRemove)
		}
		// Repair last commit sigs
		lastSig := (*head).Header().LastCommitSignature()
		sigAndBitMap := append(lastSig[:], (*head).Header().LastCommitBitmap()...)
		bc.WriteCommitSig((*head).NumberU64()-1, sigAndBitMap)

		err := rawdb.DeleteBlock(bc.db, (*head).Hash(), (*head).NumberU64())
		if err != nil {
			return errors.WithMessagef(err, "failed to delete block %d", (*head).NumberU64())
		}
		if err := rawdb.WriteHeadBlockHash(bc.db, (*head).ParentHash()); err != nil {
			return errors.WithMessagef(err, "failed to write head block hash number %d", (*head).NumberU64()-1)
		}

		// Otherwise rewind one block and recheck state availability there
		for _, stkTxn := range (*head).StakingTransactions() {
			if stkTxn.StakingType() == staking.DirectiveCreateValidator {
				if addr, err := stkTxn.SenderAddress(); err == nil {
					valsToRemove[addr] = struct{}{}
				} else {
					return err
				}
			}
		}
		block := bc.GetBlock((*head).ParentHash(), (*head).NumberU64()-1)
		if block == nil {
			return fmt.Errorf("missing block %d [%x]", (*head).NumberU64()-1, (*head).ParentHash())
		}
		*head = block
	}
}

// This func is used to remove the validator addresses from the validator list.
func (bc *BlockChainImpl) removeInValidatorList(toRemove map[common.Address]struct{}) error {
	if len(toRemove) == 0 {
		return nil
	}
	utils.Logger().Info().
		Interface("validators", toRemove).
		Msg("Removing validators from validator list")

	existingVals, err := bc.ReadValidatorList()
	if err != nil {
		return err
	}
	newVals := []common.Address{}
	for _, addr := range existingVals {
		if _, ok := toRemove[addr]; !ok {
			newVals = append(newVals, addr)
		}
	}
	return bc.WriteValidatorList(bc.db, newVals)
}

// Export writes the active chain to the given writer.
func (bc *BlockChainImpl) Export(w io.Writer) error {
	return bc.ExportN(w, uint64(0), bc.CurrentBlock().NumberU64())
}

// ExportN writes a subset of the active chain to the given writer.
func (bc *BlockChainImpl) ExportN(w io.Writer, first uint64, last uint64) error {
	bc.chainmu.RLock()
	defer bc.chainmu.RUnlock()

	if first > last {
		return fmt.Errorf("export failed: first (%d) is greater than last (%d)", first, last)
	}
	utils.Logger().Info().Uint64("count", last-first+1).Msg("Exporting batch of blocks")

	start, reported := time.Now(), time.Now()
	for nr := first; nr <= last; nr++ {
		block := bc.GetBlockByNumber(nr)
		if block == nil {
			return fmt.Errorf("export failed on #%d: not found", nr)
		}
		if err := block.EncodeRLP(w); err != nil {
			return err
		}
		if time.Since(reported) >= statsReportLimit {
			utils.Logger().Info().
				Uint64("exported", block.NumberU64()-first).
				Str("elapsed", common.PrettyDuration(time.Since(start)).String()).
				Msg("Exporting blocks")
			reported = time.Now()
		}
	}

	return nil
}

func (bc *BlockChainImpl) WriteHeadBlock(block *types.Block) error {
	return bc.writeHeadBlock(block)
}

// writeHeadBlock writes a new head block
func (bc *BlockChainImpl) writeHeadBlock(block *types.Block) error {
	// If the block is on a side chain or an unknown one, force other heads onto it too
	updateHeads := bc.GetCanonicalHash(block.NumberU64()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	batch := bc.ChainDb().NewBatch()
	if err := rawdb.WriteCanonicalHash(batch, block.Hash(), block.NumberU64()); err != nil {
		return err
	}
	if err := rawdb.WriteHeadBlockHash(batch, block.Hash()); err != nil {
		return err
	}
	if err := rawdb.WriteHeadHeaderHash(batch, block.Hash()); err != nil {
		return err
	}
	if err := rawdb.WriteHeaderNumber(batch, block.Hash(), block.NumberU64()); err != nil {
		return err
	}

	isNewEpoch := block.IsLastBlockInEpoch()
	if isNewEpoch {
		epoch := block.Header().Epoch()
		nextEpoch := epoch.Add(epoch, common.Big1)
		if err := rawdb.WriteShardStateBytes(batch, nextEpoch, block.Header().ShardState()); err != nil {
			utils.Logger().Error().Err(err).Msg("failed to store shard state")
			return err
		}
	}

	if err := batch.Write(); err != nil {
		return err
	}

	bc.currentBlock.Store(block)
	headBlockGauge.Update(int64(block.NumberU64()))

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		if err := bc.hc.SetCurrentHeader(block.Header()); err != nil {
			return errors.Wrap(err, "HeaderChain SetCurrentHeader")
		}
		if err := rawdb.WriteHeadFastBlockHash(bc.db, block.Hash()); err != nil {
			return err
		}

		bc.currentFastBlock.Store(block)
		headFastBlockGauge.Update(int64(block.NumberU64()))
	}
	return nil
}

// tikvFastForward writes a new head block in tikv mode, used for reader node or follower writer node
func (bc *BlockChainImpl) tikvFastForward(block *types.Block, logs []*types.Log) error {
	bc.currentBlock.Store(block)
	headBlockGauge.Update(int64(block.NumberU64()))

	if err := bc.hc.SetCurrentHeader(block.Header()); err != nil {
		return errors.Wrap(err, "HeaderChain SetCurrentHeader")
	}

	bc.currentFastBlock.Store(block)
	headFastBlockGauge.Update(int64(block.NumberU64()))

	var events []interface{}
	events = append(events, ChainEvent{block, block.Hash(), logs})
	events = append(events, ChainHeadEvent{block})

	if block.NumberU64() > triesInRedis {
		bc.latestCleanCacheNum = block.NumberU64() - triesInRedis
	}

	bc.PostChainEvents(events, logs)
	return nil
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *BlockChainImpl) insert(block *types.Block) error {
	return bc.writeHeadBlock(block)
}

// Genesis retrieves the chain's genesis block.
func (bc *BlockChainImpl) Genesis() *types.Block {
	return bc.genesisBlock
}

// GetBody retrieves a block body (transactions and uncles) from the database by
// hash, caching it if found.
func (bc *BlockChainImpl) GetBody(hash common.Hash) *types.Body {
	// Short circuit if the body's already in the cache, retrieve otherwise
	if cached, ok := bc.bodyCache.Get(hash); ok {
		body := cached.(*types.Body)
		return body
	}
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	body := rawdb.ReadBody(bc.db, hash, *number)
	if body == nil {
		return nil
	}
	// Cache the found body for next time and return
	bc.bodyCache.Add(hash, body)
	return body
}

// GetBodyRLP retrieves a block body in RLP encoding from the database by hash,
// caching it if found.
func (bc *BlockChainImpl) GetBodyRLP(hash common.Hash) rlp.RawValue {
	// Short circuit if the body's already in the cache, retrieve otherwise
	if cached, ok := bc.bodyRLPCache.Get(hash); ok {
		return cached.(rlp.RawValue)
	}
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	body := rawdb.ReadBodyRLP(bc.db, hash, *number)
	if len(body) == 0 {
		return nil
	}
	// Cache the found body for next time and return
	bc.bodyRLPCache.Add(hash, body)
	return body
}

func (bc *BlockChainImpl) HasBlock(hash common.Hash, number uint64) bool {
	if bc.blockCache.Contains(hash) {
		return true
	}
	return rawdb.HasBody(bc.db, hash, number)
}

func (bc *BlockChainImpl) HasState(hash common.Hash) bool {
	_, err := bc.stateCache.OpenTrie(hash)
	return err == nil
}

func (bc *BlockChainImpl) HasBlockAndState(hash common.Hash, number uint64) bool {
	// Check first that the block itself is known
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return bc.HasState(block.Root())
}

func (bc *BlockChainImpl) GetBlock(hash common.Hash, number uint64) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := rawdb.ReadBlock(bc.db, hash, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.blockCache.Add(block.Hash(), block)
	return block
}

func (bc *BlockChainImpl) GetBlockByHash(hash common.Hash) *types.Block {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetBlock(hash, *number)
}

func (bc *BlockChainImpl) GetBlockByNumber(number uint64) *types.Block {
	hash := bc.GetCanonicalHash(number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, number)
}

func (bc *BlockChainImpl) GetReceiptsByHash(hash common.Hash) types.Receipts {
	if receipts, ok := bc.receiptsCache.Get(hash); ok {
		return receipts.(types.Receipts)
	}

	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number == nil {
		return nil
	}

	receipts := rawdb.ReadReceipts(bc.db, hash, *number, nil)
	bc.receiptsCache.Add(hash, receipts)
	return receipts
}

// GetBlocksFromHash returns the block corresponding to hash and up to n-1 ancestors.
// [deprecated by eth/62]
func (bc *BlockChainImpl) GetBlocksFromHash(hash common.Hash, n int) (blocks []*types.Block) {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	for i := 0; i < n; i++ {
		block := bc.GetBlock(hash, *number)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.ParentHash()
		*number--
	}
	return
}

// ContractCode retrieves a blob of data associated with a contract
// hash either from ephemeral in-memory cache, or from persistent storage.
//
// If the code doesn't exist in the in-memory cache, check the storage with
// new code scheme.
func (bc *BlockChainImpl) ContractCode(hash common.Hash) ([]byte, error) {
	return bc.stateCache.ContractCode(common.Hash{}, hash)
}

// ValidatorCode retrieves a blob of data associated with a validator
// hash either from ephemeral in-memory cache, or from persistent storage.
//
// If the code doesn't exist in the in-memory cache, check the storage with
// new code scheme.
func (bc *BlockChainImpl) ValidatorCode(hash common.Hash) ([]byte, error) {
	return bc.stateCache.ValidatorCode(common.Hash{}, hash)
}

func (bc *BlockChainImpl) GetUnclesInChain(b *types.Block, length int) []*block.Header {
	uncles := []*block.Header{}
	for i := 0; b != nil && i < length; i++ {
		uncles = append(uncles, b.Uncles()...)
		b = bc.GetBlock(b.ParentHash(), b.NumberU64()-1)
	}
	return uncles
}

// TrieDB returns trie database
func (bc *BlockChainImpl) TrieDB() *trie.Database {
	return bc.stateCache.TrieDB()
}

// TrieNode retrieves a blob of data associated with a trie node (or code hash)
// either from ephemeral in-memory cache, or from persistent storage.
func (bc *BlockChainImpl) TrieNode(hash common.Hash) ([]byte, error) {
	return bc.stateCache.TrieDB().Node(hash)
}

func (bc *BlockChainImpl) Stop() {
	if bc == nil {
		return
	}

	// Ensure that the entirety of the state snapshot is journalled to disk.
	var snapBase common.Hash
	if bc.snaps != nil {
		var err error
		if snapBase, err = bc.snaps.Journal(bc.CurrentBlock().Header().Root()); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to journal state snapshot")
		}
	}

	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	if err := bc.SavePendingCrossLinks(); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to save pending cross links")
	}

	// Unsubscribe all subscriptions registered from blockchain
	bc.scope.Close()
	close(bc.quit)
	atomic.StoreInt32(&bc.procInterrupt, 1)

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing three different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-TriesInMemory: So we have a configurable hard limit on the number of blocks reexecuted (default 128)
	if !bc.cacheConfig.Disabled {
		triedb := bc.stateCache.TrieDB()

		for _, offset := range []uint64{0, 1, bc.cacheConfig.TriesInMemory - 1} {
			if number := bc.CurrentBlock().NumberU64(); number > offset {
				recent := bc.GetHeaderByNumber(number - offset)
				if recent != nil {
					utils.Logger().Info().
						Str("block", recent.Number().String()).
						Str("hash", recent.Hash().Hex()).
						Str("root", recent.Root().Hex()).
						Msg("Writing cached state to disk")
					if err := triedb.Commit(recent.Root(), true); err != nil {
						utils.Logger().Error().Err(err).Msg("Failed to commit recent state trie")
					}
				}
			}
		}
		if snapBase != (common.Hash{}) {
			utils.Logger().Info().Interface("root", snapBase).Msg("Writing snapshot state to disk")
			if err := triedb.Commit(snapBase, true); err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to commit recent state trie")
			}
		}
		for !bc.triegc.Empty() {
			v := common.Hash(bc.triegc.PopItem())
			triedb.Dereference(v)
		}
		if size, _ := triedb.Size(); size != 0 {
			utils.Logger().Error().Msg("Dangling trie nodes after full cleanup")
		}
	}
	// Flush the collected preimages to disk
	if err := bc.stateCache.TrieDB().CommitPreimages(); err != nil {
		utils.Logger().Error().Interface("err", err).Msg("Failed to commit trie preimages")
	} else {
		if _, _, err := rawdb.WritePreImageStartEndBlock(bc.ChainDb(), 0, bc.CurrentBlock().NumberU64()); err != nil {
			utils.Logger().Error().Interface("err", err).Msg("Failed to mark preimages end block")
		}
	}
	// Ensure all live cached entries be saved into disk, so that we can skip
	// cache warmup when node restarts.
	if bc.cacheConfig.TrieCleanJournal != "" {
		bc.triedb.SaveCache(bc.cacheConfig.TrieCleanJournal)
	}
	utils.Logger().Info().Msg("Blockchain manager stopped")
}

// WriteStatus status of write
type WriteStatus byte

// Constants for WriteStatus
const (
	NonStatTy WriteStatus = iota
	CanonStatTy
	SideStatTy
)

func (bc *BlockChainImpl) Rollback(chain []common.Hash) error {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	valsToRemove := map[common.Address]struct{}{}
	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		currentHeader := bc.hc.CurrentHeader()
		if currentHeader != nil && currentHeader.Hash() == hash {
			parentHeader := bc.GetHeader(currentHeader.ParentHash(), currentHeader.Number().Uint64()-1)
			if parentHeader != nil {
				if err := bc.hc.SetCurrentHeader(parentHeader); err != nil {
					return errors.Wrap(err, "HeaderChain SetCurrentHeader")
				}
			}
		}
		if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentFastBlock.Hash() == hash {
			newFastBlock := bc.GetBlock(currentFastBlock.ParentHash(), currentFastBlock.NumberU64()-1)
			if newFastBlock != nil {
				bc.currentFastBlock.Store(newFastBlock)
				headFastBlockGauge.Update(int64(newFastBlock.NumberU64()))
				rawdb.WriteHeadFastBlockHash(bc.db, newFastBlock.Hash())
			}
		}
		if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentBlock.Hash() == hash {
			newBlock := bc.GetBlock(currentBlock.ParentHash(), currentBlock.NumberU64()-1)
			if newBlock != nil {
				bc.currentBlock.Store(newBlock)
				headBlockGauge.Update(int64(newBlock.NumberU64()))
				if err := rawdb.WriteHeadBlockHash(bc.db, newBlock.Hash()); err != nil {
					return err
				}

				for _, stkTxn := range currentBlock.StakingTransactions() {
					if stkTxn.StakingType() == staking.DirectiveCreateValidator {
						if addr, err := stkTxn.SenderAddress(); err == nil {
							valsToRemove[addr] = struct{}{}
						}
					}
				}
			}
		}
	}
	return bc.removeInValidatorList(valsToRemove)
}

// SetReceiptsData computes all the non-consensus fields of the receipts
func SetReceiptsData(config *params.ChainConfig, block *types.Block, receipts types.Receipts) error {
	signer := types.MakeSigner(config, block.Epoch())
	ethSigner := types.NewEIP155Signer(config.EthCompatibleChainID)

	transactions, stakingTransactions, logIndex := block.Transactions(), block.StakingTransactions(), uint(0)
	if len(transactions)+len(stakingTransactions) != len(receipts) {
		return errors.New("transaction+stakingTransactions and receipt count mismatch")
	}

	// The used gas can be calculated based on previous receipts
	if len(receipts) > 0 && len(transactions) > 0 {
		receipts[0].GasUsed = receipts[0].CumulativeGasUsed
	}
	for j := 1; j < len(transactions); j++ {
		// The transaction hash can be retrieved from the transaction itself
		receipts[j].TxHash = transactions[j].Hash()
		receipts[j].GasUsed = receipts[j].CumulativeGasUsed - receipts[j-1].CumulativeGasUsed
		// The contract address can be derived from the transaction itself
		if transactions[j].To() == nil {
			// Deriving the signer is expensive, only do if it's actually needed
			var from common.Address
			if transactions[j].IsEthCompatible() {
				from, _ = types.Sender(ethSigner, transactions[j])
			} else {
				from, _ = types.Sender(signer, transactions[j])
			}
			receipts[j].ContractAddress = crypto.CreateAddress(from, transactions[j].Nonce())
		}
		// The derived log fields can simply be set from the block and transaction
		for k := 0; k < len(receipts[j].Logs); k++ {
			receipts[j].Logs[k].BlockNumber = block.NumberU64()
			receipts[j].Logs[k].BlockHash = block.Hash()
			receipts[j].Logs[k].TxHash = receipts[j].TxHash
			receipts[j].Logs[k].TxIndex = uint(j)
			receipts[j].Logs[k].Index = logIndex
			logIndex++
		}
	}

	// The used gas can be calculated based on previous receipts
	if len(receipts) > len(transactions) && len(stakingTransactions) > 0 {
		receipts[len(transactions)].GasUsed = receipts[len(transactions)].CumulativeGasUsed
	}
	// in a block, txns are processed before staking txns
	for j := len(transactions) + 1; j < len(transactions)+len(stakingTransactions); j++ {
		// The transaction hash can be retrieved from the staking transaction itself
		receipts[j].TxHash = stakingTransactions[j].Hash()
		receipts[j].GasUsed = receipts[j].CumulativeGasUsed - receipts[j-1].CumulativeGasUsed
		// The derived log fields can simply be set from the block and transaction
		for k := 0; k < len(receipts[j].Logs); k++ {
			receipts[j].Logs[k].BlockNumber = block.NumberU64()
			receipts[j].Logs[k].BlockHash = block.Hash()
			receipts[j].Logs[k].TxHash = receipts[j].TxHash
			receipts[j].Logs[k].TxIndex = uint(j) + uint(len(transactions))
			receipts[j].Logs[k].Index = logIndex
			logIndex++
		}
	}
	return nil
}

// InsertReceiptChain attempts to complete an already existing header chain with
// transaction and receipt data.
func (bc *BlockChainImpl) InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error) {
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(blockChain); i++ {
		if blockChain[i].NumberU64() != blockChain[i-1].NumberU64()+1 || blockChain[i].ParentHash() != blockChain[i-1].Hash() {
			utils.Logger().Error().
				Str("number", blockChain[i].Number().String()).
				Str("hash", blockChain[i].Hash().Hex()).
				Str("parent", blockChain[i].ParentHash().Hex()).
				Str("prevnumber", blockChain[i-1].Number().String()).
				Str("prevhash", blockChain[i-1].Hash().Hex()).
				Msg("Non contiguous receipt insert")
			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, blockChain[i-1].NumberU64(),
				blockChain[i-1].Hash().Bytes()[:4], i, blockChain[i].NumberU64(), blockChain[i].Hash().Bytes()[:4], blockChain[i].ParentHash().Bytes()[:4])
		}
	}

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	var (
		stats = struct{ processed, ignored int32 }{}
		start = time.Now()
		bytes = 0
		batch = bc.db.NewBatch()
	)
	for i, block := range blockChain {
		receipts := receiptChain[i]
		// Short circuit insertion if shutting down or processing failed
		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
			return 0, fmt.Errorf("Premature abort during blocks processing")
		}
		// Add header if the owner header is unknown
		if !bc.HasHeader(block.Hash(), block.NumberU64()) {
			if err := rawdb.WriteHeader(batch, block.Header()); err != nil {
				return 0, err
			}
			// return 0, fmt.Errorf("containing header #%d [%x…] unknown", block.Number(), block.Hash().Bytes()[:4])
		}
		// Skip if the entire data is already known
		if bc.HasBlock(block.Hash(), block.NumberU64()) {
			stats.ignored++
			continue
		}
		// Compute all the non-consensus fields of the receipts
		if err := SetReceiptsData(bc.chainConfig, block, receipts); err != nil {
			return 0, fmt.Errorf("failed to set receipts data: %v", err)
		}
		// Write all the data out into the database
		if err := rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body()); err != nil {
			return 0, err
		}
		if err := rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts); err != nil {
			return 0, err
		}
		if err := rawdb.WriteBlockTxLookUpEntries(batch, block); err != nil {
			return 0, err
		}
		if err := rawdb.WriteBlockStxLookUpEntries(batch, block); err != nil {
			return 0, err
		}

		isNewEpoch := block.IsLastBlockInEpoch()
		if isNewEpoch {
			epoch := block.Header().Epoch()
			nextEpoch := epoch.Add(epoch, common.Big1)
			err := rawdb.WriteShardStateBytes(batch, nextEpoch, block.Header().ShardState())
			if err != nil {
				utils.Logger().Error().Err(err).Msg("failed to store shard state")
				return 0, err
			}
		}

		stats.processed++

		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return 0, err
			}
			bytes += batch.ValueSize()
			batch.Reset()
		}
	}
	if batch.ValueSize() > 0 {
		bytes += batch.ValueSize()
		if err := batch.Write(); err != nil {
			return 0, err
		}
	}

	// Update the head fast sync block if better
	head := blockChain[len(blockChain)-1]
	rawdb.WriteHeadFastBlockHash(bc.db, head.Hash())
	bc.currentFastBlock.Store(head)

	utils.Logger().Info().
		Int32("count", stats.processed).
		Str("elapsed", common.PrettyDuration(time.Since(start)).String()).
		Str("age", common.PrettyAge(time.Unix(head.Time().Int64(), 0)).String()).
		Str("head", head.Number().String()).
		Str("hash", head.Hash().Hex()).
		Str("size", common.StorageSize(bytes).String()).
		Int32("ignored", stats.ignored).
		Msg("Imported new block receipts")

	return int(stats.processed), nil
}

var lastWrite uint64

func (bc *BlockChainImpl) WriteBlockWithoutState(block *types.Block) (err error) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	if err := rawdb.WriteBlock(bc.db, block); err != nil {
		return err
	}

	return nil
}

func (bc *BlockChainImpl) WriteBlockWithState(
	block *types.Block, receipts []*types.Receipt,
	cxReceipts []*types.CXReceipt,
	stakeMsgs []staking.StakeMsg,
	paid reward.Reader,
	state *state.DB,
) (status WriteStatus, err error) {
	currentBlock := bc.CurrentBlock()
	if currentBlock == nil {
		return NonStatTy, errors.New("Current block is nil")
	}
	if block.ParentHash() != currentBlock.Hash() {
		return NonStatTy, errors.Errorf("Hash of parent block %s doesn't match the current block hash %s", currentBlock.Hash().Hex(), block.ParentHash().Hex())
	}

	// Commit state object changes to in-memory trie
	root, err := state.Commit(bc.chainConfig.IsS3(block.Epoch()))
	if err != nil {
		return NonStatTy, err
	}

	// Flush trie state into disk if it's archival node or the block is epoch block
	triedb := bc.stateCache.TrieDB()
	if bc.cacheConfig.Disabled || block.IsLastBlockInEpoch() {
		if err := triedb.Commit(root, false); err != nil {
			if isUnrecoverableErr(err) {
				fmt.Printf("Unrecoverable error when committing triedb: %v\nExitting\n", err)
				os.Exit(1)
			}
			return NonStatTy, err
		}

		// clean block tire info in redis, used for tikv mode
		if block.NumberU64() > triesInRedis {
			select {
			case bc.cleanCacheChan <- block.NumberU64() - triesInRedis:
			default:
			}
		}
	} else {
		// Full but not archive node, do proper garbage collection
		triedb.Reference(root, common.Hash{}) // metadata reference to keep trie alive
		// r := common.Hash(root)
		bc.triegc.Push(root, -int64(block.NumberU64()))

		if current := block.NumberU64(); current > bc.cacheConfig.TriesInMemory {
			// If we exceeded our memory allowance, flush matured singleton nodes to disk
			var (
				nodes, imgs = triedb.Size()
				limit       = common.StorageSize(bc.cacheConfig.TrieNodeLimit) * 1024 * 1024
			)
			if nodes > limit || imgs > 4*1024*1024 {
				triedb.Cap(limit - ethdb.IdealBatchSize)
			}
			// Find the next state trie we need to commit
			header := bc.GetHeaderByNumber(current - bc.cacheConfig.TriesInMemory)
			if header != nil {
				chosen := header.Number().Uint64()

				// If we exceeded out time allowance, flush an entire trie to disk
				if bc.gcproc > bc.cacheConfig.TrieTimeLimit {
					// If we're exceeding limits but haven't reached a large enough memory gap,
					// warn the user that the system is becoming unstable.
					if chosen < lastWrite+bc.cacheConfig.TriesInMemory && bc.gcproc >= 2*bc.cacheConfig.TrieTimeLimit {
						utils.Logger().Info().
							Dur("time", bc.gcproc).
							Dur("allowance", bc.cacheConfig.TrieTimeLimit).
							Float64("optimum", float64(chosen-lastWrite)/float64(bc.cacheConfig.TriesInMemory)).
							Msg("State in memory for too long, committing")
					}
					// Flush an entire trie and restart the counters
					triedb.Commit(header.Root(), true)
					lastWrite = chosen
					bc.gcproc = 0
				}
				// Garbage collect anything below our required write retention
				for !bc.triegc.Empty() {
					root, number := bc.triegc.Pop()
					if uint64(-number) > chosen {
						bc.triegc.Push(root, number)
						break
					}
					if -number > bc.maxGarbCollectedBlkNum {
						bc.maxGarbCollectedBlkNum = -number
					}
					triedb.Dereference(root)
				}
			}
		}
	}

	batch := bc.db.NewBatch()
	// Write the raw block
	if err := rawdb.WriteBlock(batch, block); err != nil {
		return NonStatTy, err
	}

	// Write offchain data
	if status, err := bc.CommitOffChainData(
		batch, block, receipts,
		cxReceipts, stakeMsgs,
		paid, state,
	); err != nil {
		return status, err
	}

	// Write the positional metadata for transaction/receipt lookups and preimages
	if err := rawdb.WriteBlockTxLookUpEntries(batch, block); err != nil {
		return NonStatTy, err
	}
	if err := rawdb.WriteBlockStxLookUpEntries(batch, block); err != nil {
		return NonStatTy, err
	}
	if err := rawdb.WriteCxLookupEntries(batch, block); err != nil {
		return NonStatTy, err
	}
	if err := rawdb.WritePreimages(batch, state.Preimages()); err != nil {
		return NonStatTy, err
	}

	if bc.IsEnablePruneBeaconChainFeature() {
		if block.Number().Cmp(big.NewInt(pruneBeaconChainBlockBefore)) > 0 && block.Epoch().Cmp(big.NewInt(pruneBeaconChainBeforeEpoch)) > 0 {
			maxBlockNum := big.NewInt(0).Sub(block.Number(), big.NewInt(pruneBeaconChainBlockBefore)).Uint64()
			maxEpoch := big.NewInt(0).Sub(block.Epoch(), big.NewInt(pruneBeaconChainBeforeEpoch))
			go func() {
				err := bc.blockchainPruner.Start(maxBlockNum, maxEpoch)
				if err != nil {
					utils.Logger().Info().Err(err).Msg("pruneBeaconChain init error")
					return
				}
			}()
		}
	}

	if err := batch.Write(); err != nil {
		if isUnrecoverableErr(err) {
			fmt.Printf("Unrecoverable error when writing leveldb: %v\nExitting\n", err)
			os.Exit(1)
		}
		return NonStatTy, err
	}

	// Update current block
	if err := bc.writeHeadBlock(block); err != nil {
		return NonStatTy, errors.Wrap(err, "writeHeadBlock")
	}

	return CanonStatTy, nil
}

func (bc *BlockChainImpl) GetMaxGarbageCollectedBlockNumber() int64 {
	return bc.maxGarbCollectedBlkNum
}

func (bc *BlockChainImpl) InsertChain(chain types.Blocks, verifyHeaders bool) (int, error) {
	// if in tikv mode, writer node need preempt master or come be a follower
	if bc.isInitTiKV() && !bc.tikvPreemptMaster(bc.rangeBlock(chain)) {
		return len(chain), nil
	}

	for _, b := range chain {
		// check if blocks already exist
		if bc.HasBlock(b.Hash(), b.NumberU64()) {
			return 0, errors.Wrapf(ErrKnownBlock, "block %s %d already exists", b.Hash().Hex(), b.NumberU64())
		}
	}

	prevHash := bc.CurrentBlock().Hash()
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()
	n, events, logs, err := bc.insertChain(chain, verifyHeaders)
	bc.PostChainEvents(events, logs)
	if err == nil {
		if prevHash == bc.CurrentBlock().Hash() {
			panic("insertChain failed to update current block")
		}
		// there should be only 1 block.
		for _, b := range chain {
			if b.Epoch().Uint64() > 0 {
				err := bc.saveLeaderRotationMeta(b.Header())
				if err != nil {
					utils.Logger().Error().Err(err).Msg("save leader continuous blocks count error")
					return n, err
				}
			}
		}
	}
	if bc.isInitTiKV() && err != nil {
		// if has some error, master writer node will release the permission
		_, _ = bc.redisPreempt.Unlock()
	}
	return n, err
}

func (bc *BlockChainImpl) LeaderRotationMeta() LeaderRotationMeta {
	return bc.leaderRotationMeta.Clone()
}

// insertChain will execute the actual chain insertion and event aggregation. The
// only reason this method exists as a separate one is to make locking cleaner
// with deferred statements.
func (bc *BlockChainImpl) insertChain(chain types.Blocks, verifyHeaders bool) (int, []interface{}, []*types.Log, error) {
	// Sanity check that we have something meaningful to import
	if len(chain) == 0 {
		return 0, nil, nil, ErrEmptyChain
	}
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].NumberU64() != chain[i-1].NumberU64()+1 || chain[i].ParentHash() != chain[i-1].Hash() {
			// Chain broke ancestry, log a message (programming error) and skip insertion
			utils.Logger().Error().
				Str("number", chain[i].Number().String()).
				Str("hash", chain[i].Hash().Hex()).
				Str("parent", chain[i].ParentHash().Hex()).
				Str("prevnumber", chain[i-1].Number().String()).
				Str("prevhash", chain[i-1].Hash().Hex()).
				Msg("insertChain: non contiguous block insert")

			return 0, nil, nil, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].NumberU64(),
				chain[i-1].Hash().Bytes()[:4], i, chain[i].NumberU64(), chain[i].Hash().Bytes()[:4], chain[i].ParentHash().Bytes()[:4])
		}
	}

	// A queued approach to delivering events. This is generally
	// faster than direct delivery and requires much less mutex
	// acquiring.
	var (
		stats         = insertStats{startTime: mclock.Now()}
		events        = make([]interface{}, 0, len(chain))
		lastCanon     *types.Block
		coalescedLogs []*types.Log
	)

	var verifyHeadersResults <-chan error

	// If the block header chain has not been verified, conduct header verification here.
	if verifyHeaders {
		headers := make([]*block.Header, len(chain))
		seals := make([]bool, len(chain))

		for i, block := range chain {
			headers[i] = block.Header()
			seals[i] = true
		}
		// Note that VerifyHeaders verifies headers in the chain in parallel
		abort, results := bc.Engine().VerifyHeaders(bc, headers, seals)
		verifyHeadersResults = results
		defer close(abort)
	}

	// Start a parallel signature recovery (signer will fluke on fork transition, minimal perf loss)
	//senderCacher.recoverFromBlocks(types.MakeSigner(bc.chainConfig, chain[0].Number()), chain)

	// Iterate over the blocks and insert when the verifier permits
	for i, block := range chain {
		// If the chain is terminating, stop processing blocks
		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
			utils.Logger().Debug().Msg("Premature abort during blocks processing")
			break
		}
		// Wait for the block's verification to complete
		bstart := time.Now()

		var err error
		if verifyHeaders {
			err = <-verifyHeadersResults
		}
		if err == nil {
			err = NewBlockValidator(bc).ValidateBody(block)
		}
		switch {
		case errors.Is(err, ErrKnownBlock):
			return i, events, coalescedLogs, err

		case err == consensus_engine.ErrFutureBlock:
			return i, events, coalescedLogs, err

		case errors.Is(err, consensus_engine.ErrUnknownAncestor):
			return i, events, coalescedLogs, err

		case errors.Is(err, consensus_engine.ErrPrunedAncestor):
			// TODO: add fork choice mechanism
			// Block competing with the canonical chain, store in the db, but don't process
			// until the competitor TD goes above the canonical TD
			//currentBlock := bc.CurrentBlock()
			//localTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
			//externTd := new(big.Int).Add(bc.GetTd(block.ParentHash(), block.NumberU64()-1), block.Difficulty())
			//if localTd.Cmp(externTd) > 0 {
			//	if err = bc.WriteBlockWithoutState(block, externTd); err != nil {
			//		return i, events, coalescedLogs, err
			//	}
			//	continue
			//}
			// Competitor chain beat canonical, gather all blocks from the common ancestor
			var winner []*types.Block

			parent := bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
			for parent != nil && !bc.HasState(parent.Root()) {
				winner = append(winner, parent)
				parent = bc.GetBlock(parent.ParentHash(), parent.NumberU64()-1)
			}
			for j := 0; j < len(winner)/2; j++ {
				winner[j], winner[len(winner)-1-j] = winner[len(winner)-1-j], winner[j]
			}
			// Prune in case non-empty winner chain
			if len(winner) > 0 {
				// Import all the pruned blocks to make the state available
				_, evs, logs, err := bc.insertChain(winner, true /* verifyHeaders */)
				events, coalescedLogs = evs, logs

				if err != nil {
					return i, events, coalescedLogs, err
				}
			}

		case err != nil:
			bc.reportBlock(block, nil, err)
			return i, events, coalescedLogs, err
		}

		// Create a new statedb using the parent block and report an
		// error if it fails.
		var parent *types.Block
		if i == 0 {
			parent = bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
		} else {
			parent = chain[i-1]
		}
		state, err := state.New(parent.Root(), bc.stateCache, bc.snaps)
		if err != nil {
			return i, events, coalescedLogs, err
		}
		vmConfig := bc.vmConfig
		if bc.trace {
			ev := TraceEvent{
				Tracer: &tracers.ParityBlockTracer{
					Hash:   block.Hash(),
					Number: block.NumberU64(),
				},
			}
			vmConfig = vm.Config{
				Debug:  true,
				Tracer: ev.Tracer,
			}
			events = append(events, ev)
		}
		// Process block using the parent state as reference point.
		substart := time.Now()
		receipts, cxReceipts, stakeMsgs, logs, usedGas, payout, newState, err := bc.processor.Process(
			block, state, vmConfig, true,
		)
		state = newState // update state in case the new state is cached.
		if err != nil {
			bc.reportBlock(block, receipts, err)
			return i, events, coalescedLogs, err
		}

		// Update the metrics touched during block processing
		accountReadTimer.Update(state.AccountReads)           // Account reads are complete, we can mark them
		storageReadTimer.Update(state.StorageReads)           // Storage reads are complete, we can mark them
		accountUpdateTimer.Update(state.AccountUpdates)       // Account updates are complete, we can mark them
		storageUpdateTimer.Update(state.StorageUpdates)       // Storage updates are complete, we can mark them
		triehash := state.AccountHashes + state.StorageHashes // Save to not double count in validation
		trieproc := state.AccountReads + state.AccountUpdates
		trieproc += state.StorageReads + state.StorageUpdates
		blockExecutionTimer.Update(time.Since(substart) - trieproc - triehash)

		// Validate the state using the default validator
		substart = time.Now()
		if err := bc.validator.ValidateState(
			block, state, receipts, cxReceipts, usedGas,
		); err != nil {
			bc.reportBlock(block, receipts, err)
			return i, events, coalescedLogs, err
		}
		proctime := time.Since(bstart)

		// Update the metrics touched during block validation
		accountHashTimer.Update(state.AccountHashes) // Account hashes are complete, we can mark them
		storageHashTimer.Update(state.StorageHashes) // Storage hashes are complete, we can mark them
		blockValidationTimer.Update(time.Since(substart) - (state.AccountHashes + state.StorageHashes - triehash))

		// Write the block to the chain and get the status.
		substart = time.Now()
		status, err := bc.WriteBlockWithState(
			block, receipts, cxReceipts, stakeMsgs, payout, state,
		)
		if err != nil {
			return i, events, coalescedLogs, err
		}
		logger := utils.Logger().With().
			Str("number", block.Number().String()).
			Str("hash", block.Hash().Hex()).
			Int("uncles", len(block.Uncles())).
			Int("txs", len(block.Transactions())).
			Int("stakingTxs", len(block.StakingTransactions())).
			Uint64("gas", block.GasUsed()).
			Str("elapsed", common.PrettyDuration(time.Since(bstart)).String()).
			Logger()

		// Update the metrics touched during block commit
		accountCommitTimer.Update(state.AccountCommits) // Account commits are complete, we can mark them
		storageCommitTimer.Update(state.StorageCommits) // Storage commits are complete, we can mark them

		blockWriteTimer.Update(time.Since(substart) - state.AccountCommits - state.StorageCommits)
		blockInsertTimer.UpdateSince(bstart)

		switch status {
		case CanonStatTy:
			logger.Info().Msgf("Inserted new block s: %d e: %d n:%d", block.ShardID(), block.Epoch().Uint64(), block.NumberU64())
			coalescedLogs = append(coalescedLogs, logs...)
			blockInsertTimer.UpdateSince(bstart)
			events = append(events, ChainEvent{block, block.Hash(), logs})
			lastCanon = block

			// used for tikv mode, writer node will publish update to all reader node
			if bc.isInitTiKV() {
				err = redis_helper.PublishShardUpdate(bc.ShardID(), block.NumberU64(), logs)
				if err != nil {
					utils.Logger().Warn().Err(err).Msg("redis publish shard update error")
				}
			}

			// Only count canonical blocks for GC processing time
			bc.gcproc += proctime
		}

		stats.processed++
		stats.usedGas += usedGas
		cache, _ := bc.stateCache.TrieDB().Size()
		stats.report(chain, i, cache)
	}
	// Append a single chain head event if we've progressed the chain
	if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, ChainHeadEvent{lastCanon})
	}

	return 0, events, coalescedLogs, nil
}

// insertStats tracks and reports on block insertion.
type insertStats struct {
	queued, processed int
	usedGas           uint64
	lastIndex         int
	startTime         mclock.AbsTime
}

// statsReportLimit is the time limit during import and export after which we
// always print out progress. This avoids the user wondering what's going on.
const statsReportLimit = 8 * time.Second

// report prints statistics if some number of blocks have been processed
// or more than a few seconds have passed since the last message.
func (st *insertStats) report(chain []*types.Block, index int, cache common.StorageSize) {
	// Fetch the timings for the batch
	var (
		now     = mclock.Now()
		elapsed = time.Duration(now) - time.Duration(st.startTime)
	)
	// If we're at the last block of the batch or report period reached, log
	if index == len(chain)-1 || elapsed >= statsReportLimit {
		var (
			end = chain[index]
			txs = countTransactions(chain[st.lastIndex : index+1])
		)

		context := utils.Logger().With().
			Int("blocks", st.processed).
			Int("txs", txs).
			Float64("mgas", float64(st.usedGas)/1000000).
			Str("elapsed", common.PrettyDuration(elapsed).String()).
			Float64("mgasps", float64(st.usedGas)*1000/float64(elapsed)).
			Str("number", end.Number().String()).
			Str("hash", end.Hash().Hex()).
			Str("cache", cache.String())

		if timestamp := time.Unix(end.Time().Int64(), 0); time.Since(timestamp) > time.Minute {
			context = context.Str("age", common.PrettyAge(timestamp).String())
		}

		if st.queued > 0 {
			context = context.Int("queued", st.queued)
		}

		logger := context.Logger()
		logger.Info().Msg("Imported new chain segment")

		*st = insertStats{startTime: now, lastIndex: index + 1}
	}
}

func countTransactions(chain []*types.Block) (c int) {
	for _, b := range chain {
		c += len(b.Transactions())
	}
	return c
}

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (bc *BlockChainImpl) PostChainEvents(events []interface{}, logs []*types.Log) {
	// post event logs for further processing
	if logs != nil {
		bc.logsFeed.Send(logs)
	}
	for _, event := range events {
		switch ev := event.(type) {
		case ChainEvent:
			bc.chainFeed.Send(ev)

		case ChainHeadEvent:
			bc.chainHeadFeed.Send(ev)

		case ChainSideEvent:
			bc.chainSideFeed.Send(ev)

		case TraceEvent:
			bc.traceFeed.Send(ev)
		}
	}
}

// BadBlock ..
type BadBlock struct {
	Block  *types.Block
	Reason error
}

// MarshalJSON ..
func (b BadBlock) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Block  *block.Header `json:"header"`
		Reason string        `json:"error-cause"`
	}{
		b.Block.Header(),
		b.Reason.Error(),
	})
}

func (bc *BlockChainImpl) BadBlocks() []BadBlock {
	blocks := make([]BadBlock, bc.badBlocks.Len())
	for _, hash := range bc.badBlocks.Keys() {
		if blk, exist := bc.badBlocks.Peek(hash); exist {
			blocks = append(blocks, blk.(BadBlock))
		}
	}
	return blocks
}

// addBadBlock adds a bad block to the bad-block LRU cache
func (bc *BlockChainImpl) addBadBlock(block *types.Block, reason error) {
	bc.badBlocks.Add(block.Hash(), BadBlock{block, reason})
}

// reportBlock logs a bad block error.
func (bc *BlockChainImpl) reportBlock(
	block *types.Block, receipts types.Receipts, err error,
) {
	bc.addBadBlock(block, err)
	var receiptString string
	for _, receipt := range receipts {
		receiptString += fmt.Sprintf("\t%v\n", receipt)
	}
	utils.Logger().Error().Msgf(`
########## BAD BLOCK #########
Chain config: %v

Number: %v
Epoch: %v
NumTxn: %v
NumStkTxn: %v
Hash: 0x%x
%v

Error: %v
##############################
`, bc.chainConfig,
		block.Number(),
		block.Epoch(),
		len(block.Transactions()),
		len(block.StakingTransactions()),
		block.Hash(),
		receiptString,
		err,
	)
	for i, tx := range block.StakingTransactions() {
		utils.Logger().Error().
			Msgf("StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage())
	}
}

func (bc *BlockChainImpl) CurrentHeader() *block.Header {
	return bc.hc.CurrentHeader()
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (bc *BlockChainImpl) GetTd(hash common.Hash, number uint64) *big.Int {
	return bc.hc.GetTd(hash, number)
}

// GetTdByHash retrieves a block's total difficulty in the canonical chain from the
// database by hash, caching it if found.
func (bc *BlockChainImpl) GetTdByHash(hash common.Hash) *big.Int {
	return bc.hc.GetTdByHash(hash)
}

func (bc *BlockChainImpl) GetHeader(hash common.Hash, number uint64) *block.Header {
	return bc.hc.GetHeader(hash, number)
}

func (bc *BlockChainImpl) GetHeaderByHash(hash common.Hash) *block.Header {
	return bc.hc.GetHeaderByHash(hash)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (bc *BlockChainImpl) HasHeader(hash common.Hash, number uint64) bool {
	return bc.hc.HasHeader(hash, number)
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (bc *BlockChainImpl) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return bc.hc.GetBlockHashesFromHash(hash, max)
}

func (bc *BlockChainImpl) GetCanonicalHash(number uint64) common.Hash {
	return bc.hc.GetCanonicalHash(number)
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (bc *BlockChainImpl) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	return bc.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

func (bc *BlockChainImpl) GetHeaderByNumber(number uint64) *block.Header {
	return bc.hc.GetHeaderByNumber(number)
}

func (bc *BlockChainImpl) Config() *params.ChainConfig { return bc.chainConfig }

func (bc *BlockChainImpl) Engine() consensus_engine.Engine { return bc.engine }

func (bc *BlockChainImpl) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return bc.scope.Track(bc.rmLogsFeed.Subscribe(ch))
}

func (bc *BlockChainImpl) SubscribeTraceEvent(ch chan<- TraceEvent) event.Subscription {
	bc.trace = true
	return bc.scope.Track(bc.traceFeed.Subscribe(ch))
}

func (bc *BlockChainImpl) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return bc.scope.Track(bc.chainFeed.Subscribe(ch))
}

func (bc *BlockChainImpl) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

func (bc *BlockChainImpl) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	return bc.scope.Track(bc.chainSideFeed.Subscribe(ch))
}

func (bc *BlockChainImpl) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}

func (bc *BlockChainImpl) ReadShardState(epoch *big.Int) (*shard.State, error) {
	cacheKey := string(epoch.Bytes())
	if cached, ok := bc.shardStateCache.Get(cacheKey); ok {
		shardState := cached.(*shard.State)
		return shardState, nil
	}
	shardState, err := rawdb.ReadShardState(bc.db, epoch)
	if err != nil {
		if strings.Contains(err.Error(), rawdb.MsgNoShardStateFromDB) &&
			shard.Schedule.IsSkippedEpoch(bc.ShardID(), epoch) {
			return nil, fmt.Errorf("epoch skipped on chain: %w", err)
		}
		return nil, err
	}
	bc.shardStateCache.Add(cacheKey, shardState)
	return shardState, nil
}

func (bc *BlockChainImpl) WriteShardStateBytes(db rawdb.DatabaseWriter,
	epoch *big.Int, shardState []byte,
) (*shard.State, error) {
	decodeShardState, err := shard.DecodeWrapper(shardState)
	if err != nil {
		return nil, err
	}
	err = rawdb.WriteShardStateBytes(db, epoch, shardState)
	if err != nil {
		return nil, err
	}
	cacheKey := string(epoch.Bytes())
	bc.shardStateCache.Add(cacheKey, decodeShardState)
	return decodeShardState, nil
}

func (bc *BlockChainImpl) ReadCommitSig(blockNum uint64) ([]byte, error) {
	if cached, ok := bc.lastCommitsCache.Get(blockNum); ok {
		lastCommits := cached.([]byte)
		return lastCommits, nil
	}
	lastCommits, err := rawdb.ReadBlockCommitSig(bc.db, blockNum)
	if err != nil {
		return nil, err
	}
	return lastCommits, nil
}

func (bc *BlockChainImpl) WriteCommitSig(blockNum uint64, lastCommits []byte) error {
	err := rawdb.WriteBlockCommitSig(bc.db, blockNum, lastCommits)
	if err != nil {
		return err
	}
	bc.lastCommitsCache.Add(blockNum, lastCommits)
	return nil
}

func (bc *BlockChainImpl) GetVdfByNumber(number uint64) []byte {
	header := bc.GetHeaderByNumber(number)
	if header == nil {
		return []byte{}
	}

	return header.Vdf()
}

func (bc *BlockChainImpl) GetVrfByNumber(number uint64) []byte {
	header := bc.GetHeaderByNumber(number)
	if header == nil {
		return []byte{}
	}
	return header.Vrf()
}

func (bc *BlockChainImpl) ChainDb() ethdb.Database { return bc.db }

func (bc *BlockChainImpl) GetEpochBlockNumber(epoch *big.Int) (*big.Int, error) {
	// Try cache first
	cacheKey := string(epoch.Bytes())
	if cachedValue, ok := bc.epochCache.Get(cacheKey); ok {
		return (&big.Int{}).SetBytes([]byte(cachedValue.(string))), nil
	}
	blockNum, err := rawdb.ReadEpochBlockNumber(bc.db, epoch)
	if err != nil {
		return nil, errors.Wrapf(
			err, "cannot read epoch block number from database",
		)
	}
	cachedValue := []byte(blockNum.Bytes())
	bc.epochCache.Add(cacheKey, cachedValue)
	return blockNum, nil
}

func (bc *BlockChainImpl) StoreEpochBlockNumber(
	epoch *big.Int, blockNum *big.Int,
) error {
	cacheKey := string(epoch.Bytes())
	cachedValue := []byte(blockNum.Bytes())
	bc.epochCache.Add(cacheKey, cachedValue)
	if err := rawdb.WriteEpochBlockNumber(bc.db, epoch, blockNum); err != nil {
		return errors.Wrapf(
			err, "cannot write epoch block number to database",
		)
	}
	return nil
}

func (bc *BlockChainImpl) ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error) {
	vrfNumbers := []uint64{}
	if cached, ok := bc.randomnessCache.Get("vrf-" + string(epoch.Bytes())); ok {
		encodedVrfNumbers := cached.([]byte)
		if err := rlp.DecodeBytes(encodedVrfNumbers, &vrfNumbers); err != nil {
			return nil, err
		}
		return vrfNumbers, nil
	}

	encodedVrfNumbers, err := rawdb.ReadEpochVrfBlockNums(bc.db, epoch)
	if err != nil {
		return nil, err
	}

	if err := rlp.DecodeBytes(encodedVrfNumbers, &vrfNumbers); err != nil {
		return nil, err
	}
	return vrfNumbers, nil
}

func (bc *BlockChainImpl) WriteEpochVrfBlockNums(epoch *big.Int, vrfNumbers []uint64) error {
	encodedVrfNumbers, err := rlp.EncodeToBytes(vrfNumbers)
	if err != nil {
		return err
	}

	err = rawdb.WriteEpochVrfBlockNums(bc.db, epoch, encodedVrfNumbers)
	if err != nil {
		return err
	}
	bc.randomnessCache.Add("vrf-"+string(epoch.Bytes()), encodedVrfNumbers)
	return nil
}

func (bc *BlockChainImpl) ReadEpochVdfBlockNum(epoch *big.Int) (*big.Int, error) {
	if cached, ok := bc.randomnessCache.Get("vdf-" + string(epoch.Bytes())); ok {
		encodedVdfNumber := cached.([]byte)
		return new(big.Int).SetBytes(encodedVdfNumber), nil
	}

	encodedVdfNumber, err := rawdb.ReadEpochVdfBlockNum(bc.db, epoch)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(encodedVdfNumber), nil
}

func (bc *BlockChainImpl) WriteEpochVdfBlockNum(epoch *big.Int, blockNum *big.Int) error {
	err := rawdb.WriteEpochVdfBlockNum(bc.db, epoch, blockNum.Bytes())
	if err != nil {
		return err
	}

	bc.randomnessCache.Add("vdf-"+string(epoch.Bytes()), blockNum.Bytes())
	return nil
}

func (bc *BlockChainImpl) WriteCrossLinks(batch rawdb.DatabaseWriter, cls []types.CrossLink) error {
	var err error
	for i := 0; i < len(cls); i++ {
		cl := cls[i]
		err = rawdb.WriteCrossLinkShardBlock(batch, cl.ShardID(), cl.BlockNum(), cl.Serialize())
	}
	return err
}

func (bc *BlockChainImpl) DeleteCrossLinks(cls []types.CrossLink) error {
	var err error
	for i := 0; i < len(cls); i++ {
		cl := cls[i]
		err = rawdb.DeleteCrossLinkShardBlock(bc.db, cl.ShardID(), cl.BlockNum())
	}
	return err
}

func (bc *BlockChainImpl) ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error) {
	bytes, err := rawdb.ReadCrossLinkShardBlock(bc.db, shardID, blockNum)
	if err != nil {
		return nil, err
	}
	crossLink, err := types.DeserializeCrossLink(bytes)

	return crossLink, err
}

func (bc *BlockChainImpl) LastContinuousCrossLink(batch rawdb.DatabaseWriter, shardID uint32) error {
	oldLink, err := bc.ReadShardLastCrossLink(shardID)
	if oldLink == nil || err != nil {
		return err
	}
	newLink := oldLink
	// Starting from last checkpoint, keeping reading immediate next crosslink until there is a gap
	for i := oldLink.BlockNum() + 1; ; i++ {
		tmp, err := bc.ReadCrossLink(shardID, i)
		if err == nil && tmp != nil && tmp.BlockNum() == i {
			newLink = tmp
		} else {
			break
		}
	}

	if newLink.BlockNum() > oldLink.BlockNum() {
		utils.Logger().Debug().Msgf("LastContinuousCrossLink: latest checkpoint blockNum %d", newLink.BlockNum())
		return rawdb.WriteShardLastCrossLink(batch, shardID, newLink.Serialize())
	}
	return nil
}

func (bc *BlockChainImpl) ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error) {
	bytes, err := rawdb.ReadShardLastCrossLink(bc.db, shardID)
	if err != nil {
		return nil, err
	}
	return types.DeserializeCrossLink(bytes)
}

func (bc *BlockChainImpl) writeSlashes(processed slash.Records) error {
	bytes, err := rlp.EncodeToBytes(processed)
	if err != nil {
		const msg = "failed to encode slashing candidates"
		utils.Logger().Error().Msg(msg)
		return err
	}
	if err := rawdb.WritePendingSlashingCandidates(bc.db, bytes); err != nil {
		return err
	}
	return nil
}

func (bc *BlockChainImpl) DeleteFromPendingSlashingCandidates(
	processed slash.Records,
) error {
	bc.pendingSlashingCandidatesMU.Lock()
	defer bc.pendingSlashingCandidatesMU.Unlock()
	current := bc.ReadPendingSlashingCandidates()
	bc.pendingSlashes = processed.SetDifference(current)
	return bc.writeSlashes(bc.pendingSlashes)
}

func (bc *BlockChainImpl) ReadPendingSlashingCandidates() slash.Records {
	if !bc.Config().IsStaking(bc.CurrentHeader().Epoch()) {
		return slash.Records{}
	}
	return append(bc.pendingSlashes[0:0], bc.pendingSlashes...)
}

func (bc *BlockChainImpl) ReadPendingCrossLinks() ([]types.CrossLink, error) {
	cls := []types.CrossLink{}
	bytes := []byte{}
	if cached, ok := bc.pendingCrossLinksCache.Get(pendingCLCacheKey); ok {
		cls = cached.([]types.CrossLink)
		return cls, nil
	} else {
		by, err := rawdb.ReadPendingCrossLinks(bc.db)
		if err != nil || len(by) == 0 {
			return nil, err
		}
		bytes = by
	}
	if err := rlp.DecodeBytes(bytes, &cls); err != nil {
		utils.Logger().Error().Err(err).Msg("Invalid pending crosslink RLP decoding")
		return nil, err
	}

	bc.pendingCrossLinksCache.Add(pendingCLCacheKey, cls)
	return cls, nil
}

func (bc *BlockChainImpl) CachePendingCrossLinks(crossLinks []types.CrossLink) error {
	// deduplicate crosslinks if any
	m := map[uint32]map[uint64]types.CrossLink{}
	for _, cl := range crossLinks {
		if _, ok := m[cl.ShardID()]; !ok {
			m[cl.ShardID()] = map[uint64]types.CrossLink{}
		}
		m[cl.ShardID()][cl.BlockNum()] = cl
	}

	cls := []types.CrossLink{}
	for _, m1 := range m {
		for _, cl := range m1 {
			cls = append(cls, cl)
		}
	}
	utils.Logger().Debug().Msgf("[CachePendingCrossLinks] Before Dedup has %d cls, after Dedup has %d cls", len(crossLinks), len(cls))

	bc.pendingCrossLinksCache.Add(pendingCLCacheKey, cls)
	return nil
}

func (bc *BlockChainImpl) SavePendingCrossLinks() error {
	if cached, ok := bc.pendingCrossLinksCache.Get(pendingCLCacheKey); ok {
		cls := cached.([]types.CrossLink)
		bytes, err := rlp.EncodeToBytes(cls)
		if err != nil {
			return err
		}
		if err := rawdb.WritePendingCrossLinks(bc.db, bytes); err != nil {
			return err
		}
	}
	return nil
}

func (bc *BlockChainImpl) AddPendingSlashingCandidates(
	candidates slash.Records,
) error {
	bc.pendingSlashingCandidatesMU.Lock()
	defer bc.pendingSlashingCandidatesMU.Unlock()
	current := bc.ReadPendingSlashingCandidates()

	state, err := bc.State()
	if err != nil {
		return err
	}

	valid := slash.Records{}
	for i := range candidates {
		if err := slash.Verify(bc, state, &candidates[i]); err == nil {
			valid = append(valid, candidates[i])
		}
	}

	pendingSlashes := append(
		bc.pendingSlashes, current.SetDifference(valid)...,
	)

	if l, c := len(pendingSlashes), len(current); l > maxPendingSlashes {
		return errors.Wrapf(
			errExceedMaxPendingSlashes, "current %d with-additional %d", c, l,
		)
	}
	bc.pendingSlashes = pendingSlashes
	return bc.writeSlashes(bc.pendingSlashes)
}

func (bc *BlockChainImpl) AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error) {
	bc.pendingCrossLinksMutex.Lock()
	defer bc.pendingCrossLinksMutex.Unlock()

	cls, err := bc.ReadPendingCrossLinks()
	if err != nil || len(cls) == 0 {
		err := bc.CachePendingCrossLinks(pendingCLs)
		return len(pendingCLs), err
	}
	cls = append(cls, pendingCLs...)
	err = bc.CachePendingCrossLinks(cls)
	return len(cls), err
}

func (bc *BlockChainImpl) DeleteFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error) {
	bc.pendingCrossLinksMutex.Lock()
	defer bc.pendingCrossLinksMutex.Unlock()

	cls, err := bc.ReadPendingCrossLinks()
	if err != nil || len(cls) == 0 {
		return 0, err
	}

	m := map[uint32]map[uint64]struct{}{}
	for _, cl := range crossLinks {
		if _, ok := m[cl.ShardID()]; !ok {
			m[cl.ShardID()] = map[uint64]struct{}{}
		}
		m[cl.ShardID()][cl.BlockNum()] = struct{}{}
	}

	pendingCLs := []types.CrossLink{}

	for _, cl := range cls {
		if _, ok := m[cl.ShardID()]; ok {
			if _, ok1 := m[cl.ShardID()][cl.BlockNum()]; ok1 {
				continue
			}
		}
		pendingCLs = append(pendingCLs, cl)
	}
	err = bc.CachePendingCrossLinks(pendingCLs)
	return len(pendingCLs), err
}

func (bc *BlockChainImpl) IsSameLeaderAsPreviousBlock(block *types.Block) bool {
	if IsEpochBlock(block) {
		return false
	}

	previousHeader := bc.GetHeaderByNumber(block.NumberU64() - 1)
	if previousHeader == nil {
		return false
	}
	return block.Coinbase() == previousHeader.Coinbase()
}

func (bc *BlockChainImpl) GetVMConfig() *vm.Config {
	return &bc.vmConfig
}

func (bc *BlockChainImpl) ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error) {
	cxs, err := rawdb.ReadCXReceipts(bc.db, shardID, blockNum, blockHash)
	if err != nil || len(cxs) == 0 {
		return nil, err
	}
	return cxs, nil
}

func (bc *BlockChainImpl) CXMerkleProof(toShardID uint32, block *block.Header) (*types.CXMerkleProof, error) {
	proof := &types.CXMerkleProof{BlockNum: block.Number(), BlockHash: block.Hash(), ShardID: block.ShardID(), CXReceiptHash: block.OutgoingReceiptHash(), CXShardHashes: []common.Hash{}, ShardIDs: []uint32{}}

	epoch := block.Epoch()
	shardingConfig := shard.Schedule.InstanceForEpoch(epoch)
	shardNum := int(shardingConfig.NumShards())

	for i := 0; i < shardNum; i++ {
		receipts, err := bc.ReadCXReceipts(uint32(i), block.NumberU64(), block.Hash())
		if err != nil || len(receipts) == 0 {
			continue
		} else {
			hash := types.DeriveSha(receipts)
			proof.CXShardHashes = append(proof.CXShardHashes, hash)
			proof.ShardIDs = append(proof.ShardIDs, uint32(i))
		}
	}
	if len(proof.ShardIDs) == 0 {
		return nil, nil
	}
	return proof, nil
}

func (bc *BlockChainImpl) WriteCXReceiptsProofSpent(db rawdb.DatabaseWriter, cxps []*types.CXReceiptsProof) error {
	for _, cxp := range cxps {
		if err := rawdb.WriteCXReceiptsProofSpent(db, cxp); err != nil {
			return err
		}
	}
	return nil
}

func (bc *BlockChainImpl) IsSpent(cxp *types.CXReceiptsProof) bool {
	shardID := cxp.MerkleProof.ShardID
	blockNum := cxp.MerkleProof.BlockNum.Uint64()
	by, _ := rawdb.ReadCXReceiptsProofSpent(bc.db, shardID, blockNum)
	return by == rawdb.SpentByte
}

func (bc *BlockChainImpl) ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64) {
	return rawdb.ReadTxLookupEntry(bc.db, txID)
}

func (bc *BlockChainImpl) ReadValidatorInformationAtRoot(
	addr common.Address, root common.Hash,
) (*staking.ValidatorWrapper, error) {
	state, err := bc.StateAt(root)
	if err != nil || state == nil {
		return nil, errors.Wrapf(err, "at root: %s", root.Hex())
	}
	return bc.ReadValidatorInformationAtState(addr, state)
}

func (bc *BlockChainImpl) ReadValidatorInformationAtState(
	addr common.Address, state *state.DB,
) (*staking.ValidatorWrapper, error) {
	if state == nil {
		return nil, errors.New("empty state")
	}
	wrapper, err := state.ValidatorWrapper(addr, true, false)
	if err != nil {
		return nil, err
	}
	return wrapper, nil
}

func (bc *BlockChainImpl) ReadValidatorInformation(
	addr common.Address,
) (*staking.ValidatorWrapper, error) {
	return bc.ReadValidatorInformationAtRoot(addr, bc.CurrentBlock().Root())
}

func (bc *BlockChainImpl) ReadValidatorSnapshotAtEpoch(
	epoch *big.Int,
	addr common.Address,
) (*staking.ValidatorSnapshot, error) {
	return rawdb.ReadValidatorSnapshot(bc.db, addr, epoch)
}

func (bc *BlockChainImpl) ReadValidatorSnapshot(
	addr common.Address,
) (*staking.ValidatorSnapshot, error) {
	epoch := bc.CurrentBlock().Epoch()
	key := addr.Hex() + epoch.String()
	if cached, ok := bc.validatorSnapshotCache.Get(key); ok {
		return cached.(*staking.ValidatorSnapshot), nil
	}
	vs, err := rawdb.ReadValidatorSnapshot(bc.db, addr, epoch)
	if err != nil {
		return nil, err
	}
	bc.validatorSnapshotCache.Add(key, vs)
	return vs, nil
}

func (bc *BlockChainImpl) WriteValidatorSnapshot(
	batch rawdb.DatabaseWriter, snapshot *staking.ValidatorSnapshot,
) error {
	// Batch write the current data as snapshot
	if err := rawdb.WriteValidatorSnapshot(batch, snapshot.Validator, snapshot.Epoch); err != nil {
		return err
	}

	// Update cache
	key := snapshot.Validator.Address.Hex() + snapshot.Epoch.String()
	bc.validatorSnapshotCache.Add(key, snapshot)
	return nil
}

func (bc *BlockChainImpl) ReadValidatorStats(
	addr common.Address,
) (*staking.ValidatorStats, error) {
	return rawdb.ReadValidatorStats(bc.db, addr)
}

func UpdateValidatorVotingPower(
	bc BlockChain,
	block *types.Block,
	newEpochSuperCommittee, currentEpochSuperCommittee *shard.State,
	state *state.DB,
) (map[common.Address]*staking.ValidatorStats, error) {
	if newEpochSuperCommittee == nil {
		return nil, shard.ErrSuperCommitteeNil
	}

	validatorStats := map[common.Address]*staking.ValidatorStats{}

	existing, replacing :=
		currentEpochSuperCommittee.StakedValidators(),
		newEpochSuperCommittee.StakedValidators()

	// TODO could also keep track of the BLS keys which
	// lost a slot because just losing slots doesn't mean that the
	// validator was booted, just that some of their keys lost slots
	for currentValidator := range existing.LookupSet {
		if _, keptSlot := replacing.LookupSet[currentValidator]; !keptSlot {
			// NOTE Think carefully about when time comes to delete offchain things
			// TODO Someone: collect and then delete every 30 epochs
			// rawdb.DeleteValidatorSnapshot(
			// 	bc.db, currentValidator, currentEpochSuperCommittee.Epoch,
			// )
			// rawdb.DeleteValidatorStats(bc.db, currentValidator)
			stats, err := rawdb.ReadValidatorStats(bc.ChainDb(), currentValidator)
			if err != nil {
				stats = staking.NewEmptyStats()
			}
			// This means it's already in staking epoch
			if currentEpochSuperCommittee.Epoch != nil {
				wrapper, err := state.ValidatorWrapper(currentValidator, true, false)
				if err != nil {
					return nil, err
				}

				if slash.IsBanned(wrapper) {
					stats.BootedStatus = effective.BannedForDoubleSigning
				} else if wrapper.Status == effective.Inactive {
					stats.BootedStatus = effective.TurnedInactiveOrInsufficientUptime
				} else {
					stats.BootedStatus = effective.LostEPoSAuction
				}

				// compute APR for the exiting validators
				if err := bc.ComputeAndUpdateAPR(
					block, currentEpochSuperCommittee.Epoch, wrapper, stats,
				); err != nil {
					return nil, err
				}
			}
			validatorStats[currentValidator] = stats
		}
	}

	rosters := make([]*votepower.Roster, len(newEpochSuperCommittee.Shards))
	for i := range newEpochSuperCommittee.Shards {
		subCommittee := &newEpochSuperCommittee.Shards[i]
		if newEpochSuperCommittee.Epoch == nil {
			return nil, errors.Wrapf(
				errNilEpoch,
				"block epoch %v current-committee-epoch %v",
				block.Epoch(),
				currentEpochSuperCommittee.Epoch,
			)
		}
		roster, err := votepower.Compute(subCommittee, newEpochSuperCommittee.Epoch)
		if err != nil {
			return nil, err
		}
		rosters[i] = roster
	}

	networkWide := votepower.AggregateRosters(rosters)
	for key, value := range networkWide {
		stats, err := rawdb.ReadValidatorStats(bc.ChainDb(), key)
		if err != nil {
			stats = staking.NewEmptyStats()
		}
		total := numeric.ZeroDec()
		for i := range value {
			total = total.Add(value[i].EffectiveStake)
		}
		stats.TotalEffectiveStake = total
		earningWrapping := make([]staking.VoteWithCurrentEpochEarning, len(value))
		for i := range value {
			earningWrapping[i] = staking.VoteWithCurrentEpochEarning{
				Vote:   value[i],
				Earned: big.NewInt(0),
			}
		}
		stats.MetricsPerShard = earningWrapping

		// fetch raw-stake from snapshot and update per-key metrics
		if snapshot, err := bc.ReadValidatorSnapshotAtEpoch(
			newEpochSuperCommittee.Epoch, key,
		); err == nil {
			spread := snapshot.RawStakePerSlot()
			for i := range stats.MetricsPerShard {
				stats.MetricsPerShard[i].Vote.RawStake = spread
			}
		}

		// This means it's already in staking epoch, and
		// compute APR for validators in current committee only
		if currentEpochSuperCommittee.Epoch != nil {
			if _, ok := existing.LookupSet[key]; ok {
				wrapper, err := state.ValidatorWrapper(key, true, false)
				if err != nil {
					return nil, err
				}

				if err := bc.ComputeAndUpdateAPR(
					block, currentEpochSuperCommittee.Epoch, wrapper, stats,
				); err != nil {
					return nil, err
				}
			}
		}
		validatorStats[key] = stats
	}

	return validatorStats, nil
}

func (bc *BlockChainImpl) ComputeAndUpdateAPR(
	block *types.Block, now *big.Int,
	wrapper *staking.ValidatorWrapper, stats *staking.ValidatorStats,
) error {
	if aprComputed, err := apr.ComputeForValidator(
		bc, block, wrapper,
	); err != nil {
		if errors.Cause(err) == apr.ErrInsufficientEpoch {
			utils.Logger().Info().Err(err).Msg("apr could not be computed")
		} else {
			return err
		}
	} else {
		// only insert if APR for current epoch does not exists
		aprEntry := staking.APREntry{Epoch: now, Value: *aprComputed}
		l := len(stats.APRs)
		// first time inserting apr for validator or
		// apr for current epoch does not exists
		// check the last entry's epoch, if not same, insert
		if l == 0 || stats.APRs[l-1].Epoch.Cmp(now) != 0 {
			stats.APRs = append(stats.APRs, aprEntry)
		}
		// if history is more than staking.APRHistoryLength, pop front
		if l > staking.APRHistoryLength {
			stats.APRs = stats.APRs[1:]
		}
	}
	return nil
}

func (bc *BlockChainImpl) UpdateValidatorSnapshots(
	batch rawdb.DatabaseWriter, epoch *big.Int, state *state.DB, newValidators []common.Address,
) error {
	// Note this is reading the validator list from last block.
	// It's fine since the new validators from this block is already snapshot when created.
	allValidators, err := bc.ReadValidatorList()
	if err != nil {
		return err
	}

	allValidators = append(allValidators, newValidators...)

	// Read all validator's current data and snapshot them
	for i := range allValidators {
		// The snapshot will be captured in the state after the last epoch block is finalized
		validator, err := state.ValidatorWrapper(allValidators[i], true, false)
		if err != nil {
			return err
		}

		snapshot := &staking.ValidatorSnapshot{Validator: validator, Epoch: epoch}
		if err := bc.WriteValidatorSnapshot(batch, snapshot); err != nil {
			return err
		}
	}

	return nil
}

func (bc *BlockChainImpl) ReadValidatorList() ([]common.Address, error) {
	if cached, ok := bc.validatorListCache.Get("validatorList"); ok {
		m, ok := cached.([]common.Address)
		if !ok {
			return nil, errors.New("failed to get validator list")
		}
		return m, nil
	}
	return rawdb.ReadValidatorList(bc.db)
}

func (bc *BlockChainImpl) WriteValidatorList(
	db rawdb.DatabaseWriter, addrs []common.Address,
) error {
	if err := rawdb.WriteValidatorList(db, addrs); err != nil {
		return err
	}
	bc.validatorListCache.Add("validatorList", addrs)
	return nil
}

func (bc *BlockChainImpl) ReadDelegationsByDelegator(
	delegator common.Address,
) (m staking.DelegationIndexes, err error) {
	rawResult := staking.DelegationIndexes{}
	if cached, ok := bc.validatorListByDelegatorCache.Get(string(delegator.Bytes())); ok {
		by := cached.([]byte)
		if err := rlp.DecodeBytes(by, &rawResult); err != nil {
			return nil, err
		}
	} else {
		if rawResult, err = rawdb.ReadDelegationsByDelegator(bc.db, delegator); err != nil {
			return nil, err
		}
	}
	blockNum := bc.CurrentBlock().Number()
	for _, index := range rawResult {
		if index.BlockNum.Cmp(blockNum) <= 0 {
			m = append(m, index)
		} else {
			// Filter out index that's created beyond current height of chain.
			// This only happens when there is a chain rollback.
			utils.Logger().Warn().Msgf("Future delegation index encountered. Skip: %+v", index)
		}
	}
	return m, nil
}

func (bc *BlockChainImpl) ReadDelegationsByDelegatorAt(
	delegator common.Address, blockNum *big.Int,
) (m staking.DelegationIndexes, err error) {
	rawResult := staking.DelegationIndexes{}
	if cached, ok := bc.validatorListByDelegatorCache.Get(string(delegator.Bytes())); ok {
		by := cached.([]byte)
		if err := rlp.DecodeBytes(by, &rawResult); err != nil {
			return nil, err
		}
	} else {
		if rawResult, err = rawdb.ReadDelegationsByDelegator(bc.db, delegator); err != nil {
			return nil, err
		}
	}
	for _, index := range rawResult {
		if index.BlockNum.Cmp(blockNum) <= 0 {
			m = append(m, index)
		} else {
			// Filter out index that's created beyond current height of chain.
			// This only happens when there is a chain rollback.
			utils.Logger().Warn().Msgf("Future delegation index encountered. Skip: %+v", index)
		}
	}
	return m, nil
}

// writeDelegationsByDelegator writes the list of validator addresses to database
func (bc *BlockChainImpl) writeDelegationsByDelegator(
	batch rawdb.DatabaseWriter,
	delegator common.Address,
	indices []staking.DelegationIndex,
) error {
	if err := rawdb.WriteDelegationsByDelegator(
		batch, delegator, indices,
	); err != nil {
		return err
	}
	bytes, err := rlp.EncodeToBytes(indices)
	if err == nil {
		bc.validatorListByDelegatorCache.Add(string(delegator.Bytes()), bytes)
	}
	return nil
}

func (bc *BlockChainImpl) UpdateStakingMetaData(
	batch rawdb.DatabaseWriter, block *types.Block,
	stakeMsgs []staking.StakeMsg,
	state *state.DB, epoch, newEpoch *big.Int,
) (newValidators []common.Address, err error) {
	newValidators, newDelegations, err := bc.prepareStakingMetaData(block, stakeMsgs, state)
	if err != nil {
		utils.Logger().Warn().Msgf("oops, prepareStakingMetaData failed, err: %+v", err)
		return newValidators, err
	}

	if len(newValidators) > 0 {
		list, err := bc.ReadValidatorList()
		if err != nil {
			return newValidators, err
		}

		valMap := map[common.Address]struct{}{}
		for _, addr := range list {
			valMap[addr] = struct{}{}
		}

		newAddrs := []common.Address{}
		for _, addr := range newValidators {
			if _, ok := valMap[addr]; !ok {
				newAddrs = append(newAddrs, addr)
			}

			// Update validator snapshot for the new validator
			validator, err := state.ValidatorWrapper(addr, true, false)
			if err != nil {
				return newValidators, err
			}

			if err := bc.WriteValidatorSnapshot(batch, &staking.ValidatorSnapshot{Validator: validator, Epoch: epoch}); err != nil {
				return newValidators, err
			}
			// For validator created at exactly the last block of an epoch, we should create the snapshot
			// for next epoch too.
			if newEpoch.Cmp(epoch) > 0 {
				if err := bc.WriteValidatorSnapshot(batch, &staking.ValidatorSnapshot{Validator: validator, Epoch: newEpoch}); err != nil {
					return newValidators, err
				}
			}
		}

		// Update validator list
		list = append(list, newAddrs...)
		if err = bc.WriteValidatorList(batch, list); err != nil {
			return newValidators, err
		}
	}

	for addr, delegations := range newDelegations {
		if err := bc.writeDelegationsByDelegator(batch, addr, delegations); err != nil {
			return newValidators, err
		}
	}
	return newValidators, nil
}

// prepareStakingMetaData prepare the updates of validator's
// and the delegator's meta data according to staking transaction.
// The following return values are cached end state to be written to DB.
// The reason for the cached state is to solve the issue that batch DB changes
// won't be reflected immediately so the intermediary state can't be read from DB.
// newValidators - the addresses of the newly created validators
// newDelegations - the map of delegator address and their updated delegation indexes
func (bc *BlockChainImpl) prepareStakingMetaData(
	block *types.Block, stakeMsgs []staking.StakeMsg, state *state.DB,
) ([]common.Address,
	map[common.Address]staking.DelegationIndexes,
	error,
) {
	var newValidators []common.Address
	newDelegations := map[common.Address]staking.DelegationIndexes{}
	blockNum := block.Number()
	for _, stakeMsg := range stakeMsgs {
		if delegate, ok := stakeMsg.(*staking.Delegate); ok {
			if err := processDelegateMetadata(delegate,
				newDelegations,
				state,
				bc,
				blockNum); err != nil {
				return nil, nil, err
			}
		} else {
			panic("Only *staking.Delegate stakeMsgs are supported at the moment")
		}
	}
	for _, txn := range block.StakingTransactions() {
		payload, err := txn.RLPEncodeStakeMsg()
		if err != nil {
			return nil, nil, err
		}
		decodePayload, err := staking.RLPDecodeStakeMsg(payload, txn.StakingType())
		if err != nil {
			return nil, nil, err
		}

		switch txn.StakingType() {
		case staking.DirectiveCreateValidator:
			createValidator := decodePayload.(*staking.CreateValidator)
			newList, appended := utils.AppendIfMissing(
				newValidators, createValidator.ValidatorAddress,
			)
			if !appended {
				return nil, nil, errValidatorExist
			}
			newValidators = newList

			// Add self delegation into the index
			selfIndex := staking.DelegationIndex{
				ValidatorAddress: createValidator.ValidatorAddress,
				Index:            uint64(0),
				BlockNum:         blockNum,
			}
			delegations, ok := newDelegations[createValidator.ValidatorAddress]
			if !ok {
				// If the cache doesn't have it, load it from DB for the first time.
				delegations, err = bc.ReadDelegationsByDelegator(createValidator.ValidatorAddress)
				if err != nil {
					return nil, nil, err
				}
			}
			delegations = append(delegations, selfIndex)
			newDelegations[createValidator.ValidatorAddress] = delegations
		case staking.DirectiveEditValidator:
		case staking.DirectiveDelegate:
			delegate := decodePayload.(*staking.Delegate)
			if err := processDelegateMetadata(delegate,
				newDelegations,
				state,
				bc,
				blockNum); err != nil {
				return nil, nil, err
			}

		case staking.DirectiveUndelegate:
		case staking.DirectiveCollectRewards:
		default:
		}
	}

	return newValidators, newDelegations, nil
}

func processDelegateMetadata(delegate *staking.Delegate,
	newDelegations map[common.Address]staking.DelegationIndexes,
	state *state.DB, bc *BlockChainImpl, blockNum *big.Int,
) (err error) {
	delegations, ok := newDelegations[delegate.DelegatorAddress]
	if !ok {
		// If the cache doesn't have it, load it from DB for the first time.
		delegations, err = bc.ReadDelegationsByDelegator(delegate.DelegatorAddress)
		if err != nil {
			return err
		}
	}
	if delegations, err = bc.addDelegationIndex(
		delegations, delegate.DelegatorAddress, delegate.ValidatorAddress, state, blockNum,
	); err != nil {
		return err
	}
	newDelegations[delegate.DelegatorAddress] = delegations
	return nil
}

func (bc *BlockChainImpl) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	if !bc.chainConfig.IsStaking(shard.Schedule.CalcEpochNumber(number)) {
		return big.NewInt(0), nil
	}
	if cached, ok := bc.blockAccumulatorCache.Get(number); ok {
		return cached.(*big.Int), nil
	}
	return rawdb.ReadBlockRewardAccumulator(bc.db, number)
}

func (bc *BlockChainImpl) WriteBlockRewardAccumulator(
	batch rawdb.DatabaseWriter, reward *big.Int, number uint64,
) error {
	if err := rawdb.WriteBlockRewardAccumulator(
		batch, reward, number,
	); err != nil {
		return err
	}
	bc.blockAccumulatorCache.Add(number, reward)
	return nil
}

func (bc *BlockChainImpl) UpdateBlockRewardAccumulator(
	batch rawdb.DatabaseWriter, diff *big.Int, number uint64,
) error {
	current, err := bc.ReadBlockRewardAccumulator(number - 1)
	if err != nil {
		// one-off fix for pangaea, return after pangaea enter staking.
		current = big.NewInt(0)
		bc.WriteBlockRewardAccumulator(batch, current, number)
	}
	return bc.WriteBlockRewardAccumulator(batch, new(big.Int).Add(current, diff), number)
}

// Note this should read from the state of current block in concern (root == newBlock.root)
func (bc *BlockChainImpl) addDelegationIndex(
	delegations staking.DelegationIndexes,
	delegatorAddress, validatorAddress common.Address, state *state.DB, blockNum *big.Int,
) (staking.DelegationIndexes, error) {
	// If there is an existing delegation, just return
	validatorAddressBytes := validatorAddress.Bytes()
	for _, delegation := range delegations {
		if bytes.Equal(delegation.ValidatorAddress[:], validatorAddressBytes[:]) {
			return delegations, nil
		}
	}

	// Found the delegation from state and add the delegation index
	// Note this should read from the state of current block in concern
	wrapper, err := state.ValidatorWrapper(validatorAddress, true, false)
	if err != nil {
		return delegations, err
	}
	for i := range wrapper.Delegations {
		if bytes.Equal(
			wrapper.Delegations[i].DelegatorAddress[:], delegatorAddress[:],
		) {
			// TODO(audit): change the way of indexing if we allow delegation deletion.
			delegations = append(delegations, staking.DelegationIndex{
				ValidatorAddress: validatorAddress,
				Index:            uint64(i),
				BlockNum:         blockNum,
			})
		}
	}
	return delegations, nil
}

func (bc *BlockChainImpl) ValidatorCandidates() []common.Address {
	list, err := bc.ReadValidatorList()
	if err != nil {
		return make([]common.Address, 0)
	}
	return list
}

func (bc *BlockChainImpl) DelegatorsInformation(addr common.Address) []*staking.Delegation {
	return make([]*staking.Delegation, 0)
}

// TODO: optimize this func by adding cache etc.
func (bc *BlockChainImpl) GetECDSAFromCoinbase(header *block.Header) (common.Address, error) {
	// backward compatibility: before isStaking epoch, coinbase address is the ecdsa address
	coinbase := header.Coinbase()
	isStaking := bc.Config().IsStaking(header.Epoch())
	if !isStaking {
		return coinbase, nil
	}

	shardState, err := bc.ReadShardState(header.Epoch())
	if err != nil {
		return common.Address{}, errors.Wrapf(
			err, "cannot read shard state",
		)
	}

	committee, err := shardState.FindCommitteeByID(header.ShardID())
	if err != nil {
		return common.Address{}, errors.Wrapf(
			err, "cannot find shard in the shard state",
		)
	}
	for _, member := range committee.Slots {
		// After staking the coinbase address will be the address of bls public key
		if bytes.Equal(member.EcdsaAddress[:], coinbase[:]) {
			return member.EcdsaAddress, nil
		}

		if utils.GetAddressFromBLSPubKeyBytes(member.BLSPublicKey[:]) == coinbase {
			return member.EcdsaAddress, nil
		}
	}
	return common.Address{}, errors.Errorf(
		"cannot find corresponding ECDSA Address for coinbase %s",
		header.Coinbase().Hash().Hex(),
	)
}

func (bc *BlockChainImpl) SuperCommitteeForNextEpoch(
	beacon consensus_engine.ChainReader,
	header *block.Header,
	isVerify bool,
) (*shard.State, error) {
	var (
		nextCommittee = new(shard.State)
		err           error
		beaconEpoch   = new(big.Int)
		shardState    = shard.State{}
	)
	switch header.ShardID() {
	case shard.BeaconChainShardID:
		if shard.Schedule.IsLastBlock(header.Number().Uint64()) {
			nextCommittee, err = committee.WithStakingEnabled.Compute(
				new(big.Int).Add(header.Epoch(), common.Big1),
				beacon,
			)
		}
	default:
		// TODO: needs to make sure beacon chain sync works.
		if isVerify {
			//verify
			shardState, err = header.GetShardState()
			if err != nil {
				return &shard.State{}, err
			}
			// before staking epoch
			if shardState.Epoch == nil {
				beaconEpoch = new(big.Int).Add(header.Epoch(), common.Big1)
			} else { // after staking epoch
				beaconEpoch = shardState.Epoch
			}
		} else {
			//propose
			h := beacon.CurrentHeader()
			if h.IsLastBlockInEpoch() {
				beaconEpoch = beacon.CurrentHeader().Epoch()
				beaconEpoch = beaconEpoch.Add(beaconEpoch, common.Big1)
			} else {
				beaconEpoch = beacon.CurrentHeader().Epoch()
			}
		}
		utils.Logger().Debug().Msgf("[SuperCommitteeCalculation] isVerify: %+v, realBeaconEpoch:%+v, beaconEpoch: %+v, headerEpoch:%+v, shardStateEpoch:%+v",
			isVerify, beacon.CurrentHeader().Epoch(), beaconEpoch, header.Epoch(), shardState.Epoch)
		nextEpoch := new(big.Int).Add(header.Epoch(), common.Big1)
		if bc.Config().IsStaking(nextEpoch) {
			// If next epoch is staking epoch, I should wait and listen for beacon chain for epoch changes
			switch beaconEpoch.Cmp(header.Epoch()) {
			case 1:
				// If beacon chain is bigger than shard chain in epoch, it means I should catch up with beacon chain now
				nextCommittee, err = committee.WithStakingEnabled.ReadFromDB(
					beaconEpoch, beacon,
				)

				utils.Logger().Debug().
					Uint64("blockNum", header.Number().Uint64()).
					Uint64("myCurEpoch", header.Epoch().Uint64()).
					Uint64("beaconEpoch", beaconEpoch.Uint64()).
					Msg("Propose new epoch as beacon chain's epoch")
			case 0:
				// If it's same epoch, no need to propose new shard state (new epoch change)
			case -1:
				// If beacon chain is behind, shard chain should wait for the beacon chain by not changing epochs.
			}
		} else {
			if bc.Config().IsStaking(beaconEpoch) {
				// If I am not even in the last epoch before staking epoch and beacon chain is already in staking epoch,
				// I should just catch up with beacon chain's epoch
				nextCommittee, err = committee.WithStakingEnabled.ReadFromDB(
					beaconEpoch, beacon,
				)

				utils.Logger().Debug().
					Uint64("blockNum", header.Number().Uint64()).
					Uint64("myCurEpoch", header.Epoch().Uint64()).
					Uint64("beaconEpoch", beaconEpoch.Uint64()).
					Msg("Propose entering staking along with beacon chain's epoch")
			} else {
				// If I are not in staking nor has beacon chain proposed a staking-based shard state,
				// do pre-staking committee calculation
				if shard.Schedule.IsLastBlock(header.Number().Uint64()) {
					nextCommittee, err = committee.WithStakingEnabled.Compute(
						nextEpoch,
						bc,
					)
				}
			}
		}

	}
	return nextCommittee, err
}

// GetLeaderPubKeyFromCoinbase retrieve corresponding blsPublicKey from Coinbase Address
func (bc *BlockChainImpl) GetLeaderPubKeyFromCoinbase(h *block.Header) (*bls.PublicKeyWrapper, error) {
	if cached, ok := bc.leaderPubKeyFromCoinbase.Get(h.Number().Uint64()); ok {
		return cached.(*bls.PublicKeyWrapper), nil
	}
	rs, err := bc.getLeaderPubKeyFromCoinbase(h)
	if err != nil {
		return nil, err
	}
	bc.leaderPubKeyFromCoinbase.Add(h.Number().Uint64(), rs)
	return rs, nil
}

// getLeaderPubKeyFromCoinbase retrieve corresponding blsPublicKey from Coinbase Address
func (bc *BlockChainImpl) getLeaderPubKeyFromCoinbase(h *block.Header) (*bls.PublicKeyWrapper, error) {
	shardState, err := bc.ReadShardState(h.Epoch())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read shard state %v %s",
			h.Epoch(),
			h.Coinbase().Hash().Hex(),
		)
	}

	committee, err := shardState.FindCommitteeByID(h.ShardID())
	if err != nil {
		return nil, err
	}

	committerKey := new(bls2.PublicKey)
	isStaking := bc.Config().IsStaking(h.Epoch())
	for _, member := range committee.Slots {
		if isStaking {
			// After staking the coinbase address will be the address of bls public key
			if utils.GetAddressFromBLSPubKeyBytes(member.BLSPublicKey[:]) == h.Coinbase() {
				if committerKey, err = bls.BytesToBLSPublicKey(member.BLSPublicKey[:]); err != nil {
					return nil, err
				}
				return &bls.PublicKeyWrapper{Object: committerKey, Bytes: member.BLSPublicKey}, nil
			}
		} else {
			if member.EcdsaAddress == h.Coinbase() {
				if committerKey, err = bls.BytesToBLSPublicKey(member.BLSPublicKey[:]); err != nil {
					return nil, err
				}
				return &bls.PublicKeyWrapper{Object: committerKey, Bytes: member.BLSPublicKey}, nil
			}
		}
	}
	return nil, errors.Errorf(
		"cannot find corresponding BLS Public Key coinbase %s",
		h.Coinbase().Hex(),
	)
}

func (bc *BlockChainImpl) EnablePruneBeaconChainFeature() {
	bc.pruneBeaconChainEnable = true
}

func (bc *BlockChainImpl) IsEnablePruneBeaconChainFeature() bool {
	return bc.pruneBeaconChainEnable
}

// SyncFromTiKVWriter used for tikv mode, all reader or follower writer used to sync block from master writer
func (bc *BlockChainImpl) SyncFromTiKVWriter(newBlkNum uint64, logs []*types.Log) error {
	head := rawdb.ReadHeadBlockHash(bc.db)
	dbBlock := bc.GetBlockByHash(head)
	currentBlock := bc.CurrentBlock()

	if dbBlock == nil || currentBlock == nil {
		return nil
	}

	currentBlockNum := currentBlock.NumberU64()
	if currentBlockNum < newBlkNum {
		start := time.Now()
		for i := currentBlockNum; i <= newBlkNum; i++ {
			blk := bc.GetBlockByNumber(i)
			if blk == nil {
				// cluster synchronization may be in progress
				utils.Logger().Warn().
					Uint64("currentBlockNum", i).
					Msg("[tikv sync] sync from writer got block nil, cluster synchronization may be in progress")
				return nil
			}

			err := bc.tikvFastForward(blk, logs)
			if err != nil {
				return err
			}
		}
		utils.Logger().Info().
			Uint64("currentBlockNum", currentBlockNum).
			Dur("usedTime", time.Now().Sub(start)).
			Msg("[tikv sync] sync from writer")
	}

	return nil
}

// tikvCleanCache used for tikv mode, clean block tire data from redis
func (bc *BlockChainImpl) tikvCleanCache() {
	var count int
	for to := range bc.cleanCacheChan {
		for i := bc.latestCleanCacheNum + 1; i <= to; i++ {
			// build previous block statedb
			fromBlock := bc.GetBlockByNumber(i)
			fromTrie, err := state.New(fromBlock.Root(), bc.stateCache, bc.snaps)
			if err != nil {
				continue
			}

			// build current block statedb
			toBlock := bc.GetBlockByNumber(i + 1)
			toTrie, err := state.New(toBlock.Root(), bc.stateCache, bc.snaps)
			if err != nil {
				continue
			}

			// diff two statedb and delete redis cache
			start := time.Now()
			count, err = fromTrie.DiffAndCleanCache(bc.ShardID(), toTrie)
			if err != nil {
				utils.Logger().Warn().
					Err(err).
					Msg("[tikv clean cache] error")
				break
			}

			utils.Logger().Info().
				Uint64("blockNum", i).
				Int("removeCacheEntriesCount", count).
				Dur("usedTime", time.Now().Sub(start)).
				Msg("[tikv clean cache] success")
			bc.latestCleanCacheNum = i
		}
	}
}

func (bc *BlockChainImpl) isInitTiKV() bool {
	return bc.redisPreempt != nil
}

// tikvPreemptMaster used for tikv mode, writer node need preempt master or come be a follower
func (bc *BlockChainImpl) tikvPreemptMaster(fromBlock, toBlock uint64) bool {
	for {
		// preempt master
		lockType, _ := bc.redisPreempt.TryLock(60)
		switch lockType {
		case redis_helper.LockResultRenewalSuccess:
			return true
		case redis_helper.LockResultSuccess:
			// first to master
			bc.validatorListByDelegatorCache.Purge()
			return true
		}

		// follower
		if bc.CurrentBlock().NumberU64() >= toBlock {
			return false
		}

		time.Sleep(time.Second)
	}
}

// rangeBlock get the block range of blocks
func (bc *BlockChainImpl) rangeBlock(blocks types.Blocks) (uint64, uint64) {
	if len(blocks) == 0 {
		return 0, 0
	}
	max := blocks[0].NumberU64()
	min := max
	for _, tmpBlock := range blocks {
		if tmpBlock.NumberU64() > max {
			max = tmpBlock.NumberU64()
		} else if tmpBlock.NumberU64() < min {
			min = tmpBlock.NumberU64()
		}
	}

	return min, max
}

// RedisPreempt used for tikv mode, get the redis preempt instance
func (bc *BlockChainImpl) RedisPreempt() *redis_helper.RedisPreempt {
	if bc == nil {
		return nil
	}
	return bc.redisPreempt
}

func (bc *BlockChainImpl) IsTikvWriterMaster() bool {
	if bc == nil || bc.redisPreempt == nil {
		return false
	}

	return bc.redisPreempt.LastLockStatus()
}

// InitTiKV used for tikv mode, init the tikv mode
func (bc *BlockChainImpl) InitTiKV(conf *harmonyconfig.TiKVConfig) {
	bc.cleanCacheChan = make(chan uint64, 10)

	if conf.Role == tikv.RoleWriter {
		// only writer need preempt permission
		bc.redisPreempt = redis_helper.CreatePreempt(fmt.Sprintf("shard_%d_preempt", bc.ShardID()))
	}

	if conf.Debug {
		// used for init redis cache
		// If redis is empty, the hit rate will be too low and the synchronization block speed will be slow
		// set LOAD_PRE_FETCH is yes can significantly improve this.
		if os.Getenv("LOAD_PRE_FETCH") == "yes" {
			if trie, err := state.New(bc.CurrentBlock().Root(), bc.stateCache, bc.snaps); err == nil {
				trie.Prefetch(512)
			} else {
				log.Println("LOAD_PRE_FETCH ERR: ", err)
			}
		}

		// If redis is empty, there is no need to clear the cache of nearly 1000 blocks
		// set CLEAN_INIT is the latest block num can skip slow and unneeded cleanup process
		if os.Getenv("CLEAN_INIT") != "" {
			from, err := strconv.Atoi(os.Getenv("CLEAN_INIT"))
			if err == nil {
				bc.latestCleanCacheNum = uint64(from)
			}
		}
	}

	// start clean block tire data process
	go bc.tikvCleanCache()
}

func (bc *BlockChainImpl) CommitPreimages() error {
	return bc.stateCache.TrieDB().CommitPreimages()
}

func (bc *BlockChainImpl) GetStateCache() state.Database {
	return bc.stateCache
}

func (bc *BlockChainImpl) GetSnapshotTrie() *snapshot.Tree {
	return bc.snaps
}

var (
	leveldbErrSpec         = "leveldb"
	tooManyOpenFilesErrStr = "Too many open files"
)

// isUnrecoverableErr check whether the input error is not recoverable.
// When writing db, there could be some possible errors from storage level (leveldb).
// Known possible leveldb errors are:
//  1. Leveldb is already closed. (leveldb.ErrClosed)
//  2. ldb file missing from disk. (leveldb.ErrNotFound)
//  3. Corrupted db data. (leveldb.errors.ErrCorrupted)
//  4. OS error when open file (too many open files, ...)
//  5. OS error when write file (read-only, not enough disk space, ...)
//
// Among all the above leveldb errors, only `too many open files` error is known to be recoverable,
// thus the unrecoverable errors refers to error that is
//  1. The error is from the lower storage level (from module leveldb)
//  2. The error is not too many files error.
func isUnrecoverableErr(err error) bool {
	isLeveldbErr := strings.Contains(err.Error(), leveldbErrSpec)
	isTooManyOpenFiles := strings.Contains(err.Error(), tooManyOpenFilesErrStr)
	return isLeveldbErr && !isTooManyOpenFiles
}
