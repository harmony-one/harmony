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
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

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
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	lru "github.com/hashicorp/golang-lru"
)

var (
	// blockInsertTimer
	blockInsertTimer = metrics.NewRegisteredTimer("chain/inserts", nil)
	// ErrNoGenesis is the error when there is no genesis.
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

const (
	bodyCacheLimit                     = 256
	blockCacheLimit                    = 256
	receiptsCacheLimit                 = 32
	maxFutureBlocks                    = 256
	maxTimeFutureBlocks                = 30
	badBlockLimit                      = 10
	triesInMemory                      = 128
	shardCacheLimit                    = 10
	commitsCacheLimit                  = 10
	epochCacheLimit                    = 10
	randomnessCacheLimit               = 10
	validatorCacheLimit                = 1024
	validatorStatsCacheLimit           = 1024
	validatorListCacheLimit            = 10
	validatorListByDelegatorCacheLimit = 1024
	pendingCrossLinksCacheLimit        = 2

	// BlockChainVersion ensures that an incompatible database forces a resync from scratch.
	BlockChainVersion = 3
)

// CacheConfig contains the configuration values for the trie caching/pruning
// that's resident in a blockchain.
type CacheConfig struct {
	Disabled      bool          // Whether to disable trie write caching (archive node)
	TrieNodeLimit int           // Memory limit (MB) at which to flush the current in-memory trie to disk
	TrieTimeLimit time.Duration // Time limit after which to flush the current in-memory trie to disk
}

// BlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the block chain happens according to the set of rules
// defined by the two stage Validator. Processing of blocks is done using the
// Processor which processes the included transaction. The validation of the state
// is done in the second part of the Validator. Failing results in aborting of
// the import.
//
// The BlockChain also helps in returning blocks from **any** chain included
// in the database as well as blocks that represents the canonical chain. It's
// important to note that GetBlock can return any block and does not need to be
// included in the canonical one where as GetBlockByNumber always represents the
// canonical chain.
type BlockChain struct {
	chainConfig *params.ChainConfig // Chain & network configuration
	cacheConfig *CacheConfig        // Cache configuration for pruning

	db     ethdb.Database // Low level persistent database to store final content in
	triegc *prque.Prque   // Priority queue mapping block numbers to tries to gc
	gcproc time.Duration  // Accumulates canonical block processing for trie dumping

	hc            *HeaderChain
	rmLogsFeed    event.Feed
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

	mu      sync.RWMutex // global mutex for locking chain operations
	chainmu sync.RWMutex // blockchain insertion lock
	procmu  sync.RWMutex // block processor lock

	checkpoint       int          // checkpoint counts towards the new checkpoint
	currentBlock     atomic.Value // Current head of the block chain
	currentFastBlock atomic.Value // Current head of the fast-sync chain (may be above the block chain!)

	stateCache                    state.Database // State database to reuse between imports (contains state cache)
	bodyCache                     *lru.Cache     // Cache for the most recent block bodies
	bodyRLPCache                  *lru.Cache     // Cache for the most recent block bodies in RLP encoded format
	receiptsCache                 *lru.Cache     // Cache for the most recent receipts per block
	blockCache                    *lru.Cache     // Cache for the most recent entire blocks
	futureBlocks                  *lru.Cache     // future blocks are blocks added for later processing
	shardStateCache               *lru.Cache
	lastCommitsCache              *lru.Cache
	epochCache                    *lru.Cache // Cache epoch number → first block number
	randomnessCache               *lru.Cache // Cache for vrf/vdf
	validatorCache                *lru.Cache // Cache for validator info
	validatorStatsCache           *lru.Cache // Cache for validator stats
	validatorListCache            *lru.Cache // Cache of validator list
	validatorListByDelegatorCache *lru.Cache // Cache of validator list by delegator
	pendingCrossLinksCache        *lru.Cache // Cache of last pending crosslinks

	quit    chan struct{} // blockchain quit channel
	running int32         // running must be called atomically
	// procInterrupt must be atomically called
	procInterrupt int32          // interrupt signaler for block processing
	wg            sync.WaitGroup // chain processing wait group for shutting down

	engine    consensus_engine.Engine
	processor Processor // block processor interface
	validator Validator // block and state validator interface
	vmConfig  vm.Config

	badBlocks      *lru.Cache              // Bad block cache
	shouldPreserve func(*types.Block) bool // Function used to determine whether should preserve the given block.
}

// NewBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Ethereum Validator and
// Processor.
func NewBlockChain(db ethdb.Database, cacheConfig *CacheConfig, chainConfig *params.ChainConfig, engine consensus_engine.Engine, vmConfig vm.Config, shouldPreserve func(block *types.Block) bool) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = &CacheConfig{
			TrieNodeLimit: 256 * 1024 * 1024,
			TrieTimeLimit: 5 * time.Minute,
		}
	}
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	receiptsCache, _ := lru.New(receiptsCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
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

	bc := &BlockChain{
		chainConfig:                   chainConfig,
		cacheConfig:                   cacheConfig,
		db:                            db,
		triegc:                        prque.New(nil),
		stateCache:                    state.NewDatabase(db),
		quit:                          make(chan struct{}),
		shouldPreserve:                shouldPreserve,
		bodyCache:                     bodyCache,
		bodyRLPCache:                  bodyRLPCache,
		receiptsCache:                 receiptsCache,
		blockCache:                    blockCache,
		futureBlocks:                  futureBlocks,
		shardStateCache:               shardCache,
		lastCommitsCache:              commitsCache,
		epochCache:                    epochCache,
		randomnessCache:               randomnessCache,
		validatorCache:                validatorCache,
		validatorStatsCache:           validatorStatsCache,
		validatorListCache:            validatorListCache,
		validatorListByDelegatorCache: validatorListByDelegatorCache,
		pendingCrossLinksCache:        pendingCrossLinksCache,
		engine:                        engine,
		vmConfig:                      vmConfig,
		badBlocks:                     badBlocks,
	}
	bc.SetValidator(NewBlockValidator(chainConfig, bc, engine))
	bc.SetProcessor(NewStateProcessor(chainConfig, bc, engine))

	var err error
	bc.hc, err = NewHeaderChain(db, chainConfig, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
	// Take ownership of this particular state
	go bc.update()
	return bc, nil
}

// ValidateNewBlock validates new block.
func (bc *BlockChain) ValidateNewBlock(block *types.Block) error {
	state, err := state.New(bc.CurrentBlock().Root(), bc.stateCache)

	if err != nil {
		return err
	}

	// Process block using the parent state as reference point.
	receipts, cxReceipts, _, usedGas, _, err := bc.processor.Process(block, state, bc.vmConfig)
	if err != nil {
		bc.reportBlock(block, receipts, err)
		return err
	}

	err = bc.Validator().ValidateState(block, bc.CurrentBlock(), state, receipts, cxReceipts, usedGas)
	if err != nil {
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

// EpochFirstBlock returns the block number of the first block of an epoch.
// TODO: instead of using fixed epoch schedules, determine the first block by epoch changes.
func EpochFirstBlock(epoch *big.Int) *big.Int {
	if epoch.Cmp(big.NewInt(GenesisEpoch)) == 0 {
		return big.NewInt(GenesisEpoch)
	}
	return big.NewInt(int64(shard.Schedule.EpochLastBlock(epoch.Uint64()-1) + 1))
}

func (bc *BlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *BlockChain) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		utils.Logger().Warn().Msg("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		utils.Logger().Warn().Str("hash", head.Hex()).Msg("Head block missing, resetting chain")
		return bc.Reset()
	}
	// Make sure the state associated with the block is available
	if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
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
	bc.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(bc.db); head != (common.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	bc.hc.SetCurrentHeader(currentHeader)

	// Restore the last known head fast block
	bc.currentFastBlock.Store(currentBlock)
	if head := rawdb.ReadHeadFastBlockHash(bc.db); head != (common.Hash{}) {
		if block := bc.GetBlockByHash(head); block != nil {
			bc.currentFastBlock.Store(block)
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

// SetHead rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *BlockChain) SetHead(head uint64) error {
	utils.Logger().Warn().Uint64("target", head).Msg("Rewinding blockchain")

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db rawdb.DatabaseDeleter, hash common.Hash, num uint64) {
		rawdb.DeleteBody(db, hash, num)
	}
	bc.hc.SetHead(head, delFn)
	currentHeader := bc.hc.CurrentHeader()

	// Clear out any stale content from the caches
	bc.bodyCache.Purge()
	bc.bodyRLPCache.Purge()
	bc.receiptsCache.Purge()
	bc.blockCache.Purge()
	bc.futureBlocks.Purge()
	bc.shardStateCache.Purge()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentHeader.Number().Uint64() < currentBlock.NumberU64() {
		bc.currentBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number().Uint64()))
	}
	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			bc.currentBlock.Store(bc.genesisBlock)
		}
	}
	// Rewind the fast block in a simpleton way to the target head
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentHeader.Number().Uint64() < currentFastBlock.NumberU64() {
		bc.currentFastBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number().Uint64()))
	}
	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock == nil {
		bc.currentFastBlock.Store(bc.genesisBlock)
	}
	currentBlock := bc.CurrentBlock()
	currentFastBlock := bc.CurrentFastBlock()

	rawdb.WriteHeadBlockHash(bc.db, currentBlock.Hash())
	rawdb.WriteHeadFastBlockHash(bc.db, currentFastBlock.Hash())

	return bc.loadLastState()
}

// FastSyncCommitHead sets the current head block to the one defined by the hash
// irrelevant what the chain contents were prior.
func (bc *BlockChain) FastSyncCommitHead(hash common.Hash) error {
	// Make sure that both the block as well at its state trie exists
	block := bc.GetBlockByHash(hash)
	if block == nil {
		return fmt.Errorf("non existent block [%x…]", hash[:4])
	}
	if _, err := trie.NewSecure(block.Root(), bc.stateCache.TrieDB(), 0); err != nil {
		return err
	}
	// If all checks out, manually set the head block
	bc.mu.Lock()
	bc.currentBlock.Store(block)
	bc.mu.Unlock()

	utils.Logger().Info().
		Str("number", block.Number().String()).
		Str("hash", hash.Hex()).
		Msg("Committed new head block")
	return nil
}

// ShardID returns the shard Id of the blockchain.
// TODO: use a better solution before resharding shuffle nodes to different shards
func (bc *BlockChain) ShardID() uint32 {
	return bc.CurrentBlock().ShardID()
}

// GasLimit returns the gas limit of the current HEAD block.
func (bc *BlockChain) GasLimit() uint64 {
	return bc.CurrentBlock().GasLimit()
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// CurrentFastBlock retrieves the current fast-sync head block of the canonical
// chain. The block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentFastBlock() *types.Block {
	return bc.currentFastBlock.Load().(*types.Block)
}

// SetProcessor sets the processor required for making state modifications.
func (bc *BlockChain) SetProcessor(processor Processor) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.processor = processor
}

// SetValidator sets the validator which is used to validate incoming blocks.
func (bc *BlockChain) SetValidator(validator Validator) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.validator = validator
}

// Validator returns the current validator.
func (bc *BlockChain) Validator() Validator {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.validator
}

// Processor returns the current processor.
func (bc *BlockChain) Processor() Processor {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.processor
}

// State returns a new mutable state based on the current HEAD block.
func (bc *BlockChain) State() (*state.DB, error) {
	return bc.StateAt(bc.CurrentBlock().Root())
}

// StateAt returns a new mutable state based on a particular point in time.
func (bc *BlockChain) StateAt(root common.Hash) (*state.DB, error) {
	return state.New(root, bc.stateCache)
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *BlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *BlockChain) ResetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := bc.SetHead(0); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Prepare the genesis block and reinitialise the chain
	rawdb.WriteBlock(bc.db, genesis)

	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())
	bc.currentFastBlock.Store(bc.genesisBlock)

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *BlockChain) repair(head **types.Block) error {
	for {
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New((*head).Root(), bc.stateCache); err == nil {
			utils.Logger().Info().
				Str("number", (*head).Number().String()).
				Str("hash", (*head).Hash().Hex()).
				Msg("Rewound blockchain to past state")
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		(*head) = bc.GetBlock((*head).ParentHash(), (*head).NumberU64()-1)
	}
}

// Export writes the active chain to the given writer.
func (bc *BlockChain) Export(w io.Writer) error {
	return bc.ExportN(w, uint64(0), bc.CurrentBlock().NumberU64())
}

// ExportN writes a subset of the active chain to the given writer.
func (bc *BlockChain) ExportN(w io.Writer, first uint64, last uint64) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

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

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *BlockChain) insert(block *types.Block) {
	// If the block is on a side chain or an unknown one, force other heads onto it too
	updateHeads := rawdb.ReadCanonicalHash(bc.db, block.NumberU64()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	rawdb.WriteCanonicalHash(bc.db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(bc.db, block.Hash())

	bc.currentBlock.Store(block)

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		bc.hc.SetCurrentHeader(block.Header())
		rawdb.WriteHeadFastBlockHash(bc.db, block.Hash())

		bc.currentFastBlock.Store(block)
	}
}

// Genesis retrieves the chain's genesis block.
func (bc *BlockChain) Genesis() *types.Block {
	return bc.genesisBlock
}

// GetBody retrieves a block body (transactions and uncles) from the database by
// hash, caching it if found.
func (bc *BlockChain) GetBody(hash common.Hash) *types.Body {
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
func (bc *BlockChain) GetBodyRLP(hash common.Hash) rlp.RawValue {
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

// HasBlock checks if a block is fully present in the database or not.
func (bc *BlockChain) HasBlock(hash common.Hash, number uint64) bool {
	if bc.blockCache.Contains(hash) {
		return true
	}
	return rawdb.HasBody(bc.db, hash, number)
}

// HasState checks if state trie is fully present in the database or not.
func (bc *BlockChain) HasState(hash common.Hash) bool {
	_, err := bc.stateCache.OpenTrie(hash)
	return err == nil
}

// HasBlockAndState checks if a block and associated state trie is fully present
// in the database or not, caching it if present.
func (bc *BlockChain) HasBlockAndState(hash common.Hash, number uint64) bool {
	// Check first that the block itself is known
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return bc.HasState(block.Root())
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
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

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (bc *BlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetBlock(hash, *number)
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *BlockChain) GetBlockByNumber(number uint64) *types.Block {
	hash := rawdb.ReadCanonicalHash(bc.db, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, number)
}

// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
func (bc *BlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts {
	if receipts, ok := bc.receiptsCache.Get(hash); ok {
		return receipts.(types.Receipts)
	}

	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number == nil {
		return nil
	}

	receipts := rawdb.ReadReceipts(bc.db, hash, *number)
	bc.receiptsCache.Add(hash, receipts)
	return receipts
}

// GetBlocksFromHash returns the block corresponding to hash and up to n-1 ancestors.
// [deprecated by eth/62]
func (bc *BlockChain) GetBlocksFromHash(hash common.Hash, n int) (blocks []*types.Block) {
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

// GetUnclesInChain retrieves all the uncles from a given block backwards until
// a specific distance is reached.
func (bc *BlockChain) GetUnclesInChain(b *types.Block, length int) []*block.Header {
	uncles := []*block.Header{}
	for i := 0; b != nil && i < length; i++ {
		uncles = append(uncles, b.Uncles()...)
		b = bc.GetBlock(b.ParentHash(), b.NumberU64()-1)
	}
	return uncles
}

// TrieNode retrieves a blob of data associated with a trie node (or code hash)
// either from ephemeral in-memory cache, or from persistent storage.
func (bc *BlockChain) TrieNode(hash common.Hash) ([]byte, error) {
	return bc.stateCache.TrieDB().Node(hash)
}

// Stop stops the blockchain service. If any imports are currently in progress
// it will abort them using the procInterrupt.
func (bc *BlockChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}
	// Unsubscribe all subscriptions registered from blockchain
	bc.scope.Close()
	close(bc.quit)
	atomic.StoreInt32(&bc.procInterrupt, 1)

	bc.wg.Wait()

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing three different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-127: So we have a hard limit on the number of blocks reexecuted
	if !bc.cacheConfig.Disabled {
		triedb := bc.stateCache.TrieDB()

		for _, offset := range []uint64{0, 1, triesInMemory - 1} {
			if number := bc.CurrentBlock().NumberU64(); number > offset {
				recent := bc.GetHeaderByNumber(number - offset)

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
		for !bc.triegc.Empty() {
			triedb.Dereference(bc.triegc.PopItem().(common.Hash))
		}
		if size, _ := triedb.Size(); size != 0 {
			utils.Logger().Error().Msg("Dangling trie nodes after full cleanup")
		}
	}
	utils.Logger().Info().Msg("Blockchain manager stopped")
}

func (bc *BlockChain) procFutureBlocks() {
	blocks := make([]*types.Block, 0, bc.futureBlocks.Len())
	for _, hash := range bc.futureBlocks.Keys() {
		if block, exist := bc.futureBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.Block))
		}
	}
	if len(blocks) > 0 {
		types.BlockBy(types.Number).Sort(blocks)

		// Insert one by one as chain insertion needs contiguous ancestry between blocks
		for i := range blocks {
			bc.InsertChain(blocks[i:i+1], true /* verifyHeaders */)
		}
	}
}

// WriteStatus status of write
type WriteStatus byte

// Constants for WriteStatus
const (
	NonStatTy WriteStatus = iota
	CanonStatTy
	SideStatTy
)

// Rollback is designed to remove a chain of links from the database that aren't
// certain enough to be valid.
func (bc *BlockChain) Rollback(chain []common.Hash) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		currentHeader := bc.hc.CurrentHeader()
		if currentHeader != nil && currentHeader.Hash() == hash {
			parentHeader := bc.GetHeader(currentHeader.ParentHash(), currentHeader.Number().Uint64()-1)
			if parentHeader != nil {
				bc.hc.SetCurrentHeader(parentHeader)
			}
		}
		if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentFastBlock.Hash() == hash {
			newFastBlock := bc.GetBlock(currentFastBlock.ParentHash(), currentFastBlock.NumberU64()-1)
			if newFastBlock != nil {
				bc.currentFastBlock.Store(newFastBlock)
				rawdb.WriteHeadFastBlockHash(bc.db, newFastBlock.Hash())
			}
		}
		if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentBlock.Hash() == hash {
			newBlock := bc.GetBlock(currentBlock.ParentHash(), currentBlock.NumberU64()-1)
			if newBlock != nil {
				bc.currentBlock.Store(newBlock)
				rawdb.WriteHeadBlockHash(bc.db, newBlock.Hash())
			}
		}
	}
}

// SetReceiptsData computes all the non-consensus fields of the receipts
func SetReceiptsData(config *params.ChainConfig, block *types.Block, receipts types.Receipts) error {
	signer := types.MakeSigner(config, block.Epoch())

	transactions, logIndex := block.Transactions(), uint(0)
	if len(transactions) != len(receipts) {
		return errors.New("transaction and receipt count mismatch")
	}

	for j := 0; j < len(receipts); j++ {
		// The transaction hash can be retrieved from the transaction itself
		receipts[j].TxHash = transactions[j].Hash()

		// The contract address can be derived from the transaction itself
		if transactions[j].To() == nil {
			// Deriving the signer is expensive, only do if it's actually needed
			from, _ := types.Sender(signer, transactions[j])
			receipts[j].ContractAddress = crypto.CreateAddress(from, transactions[j].Nonce())
		}
		// The used gas can be calculated based on previous receipts
		if j == 0 {
			receipts[j].GasUsed = receipts[j].CumulativeGasUsed
		} else {
			receipts[j].GasUsed = receipts[j].CumulativeGasUsed - receipts[j-1].CumulativeGasUsed
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
	return nil
}

// InsertReceiptChain attempts to complete an already existing header chain with
// transaction and receipt data.
func (bc *BlockChain) InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

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
			return 0, nil
		}
		// Short circuit if the owner header is unknown
		if !bc.HasHeader(block.Hash(), block.NumberU64()) {
			return i, fmt.Errorf("containing header #%d [%x…] unknown", block.Number(), block.Hash().Bytes()[:4])
		}
		// Skip if the entire data is already known
		if bc.HasBlock(block.Hash(), block.NumberU64()) {
			stats.ignored++
			continue
		}
		// Compute all the non-consensus fields of the receipts
		if err := SetReceiptsData(bc.chainConfig, block, receipts); err != nil {
			return i, fmt.Errorf("failed to set receipts data: %v", err)
		}
		// Write all the data out into the database
		rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body())
		rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)
		rawdb.WriteTxLookupEntries(batch, block)

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
	bc.mu.Lock()
	head := blockChain[len(blockChain)-1]
	if td := bc.GetTd(head.Hash(), head.NumberU64()); td != nil { // Rewind may have occurred, skip in that case
		currentFastBlock := bc.CurrentFastBlock()
		if bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64()).Cmp(td) < 0 {
			rawdb.WriteHeadFastBlockHash(bc.db, head.Hash())
			bc.currentFastBlock.Store(head)
		}
	}
	bc.mu.Unlock()

	utils.Logger().Info().
		Int32("count", stats.processed).
		Str("elapsed", common.PrettyDuration(time.Since(start)).String()).
		Str("age", common.PrettyAge(time.Unix(head.Time().Int64(), 0)).String()).
		Str("head", head.Number().String()).
		Str("hash", head.Hash().Hex()).
		Str("size", common.StorageSize(bytes).String()).
		Int32("ignored", stats.ignored).
		Msg("Imported new block receipts")

	return 0, nil
}

var lastWrite uint64

// WriteBlockWithoutState writes only the block and its metadata to the database,
// but does not write any state. This is used to construct competing side forks
// up to the point where they exceed the canonical total difficulty.
func (bc *BlockChain) WriteBlockWithoutState(block *types.Block, td *big.Int) (err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	if err := bc.hc.WriteTd(block.Hash(), block.NumberU64(), td); err != nil {
		return err
	}
	rawdb.WriteBlock(bc.db, block)

	return nil
}

// WriteBlockWithState writes the block and all associated state to the database.
func (bc *BlockChain) WriteBlockWithState(
	block *types.Block, receipts []*types.Receipt,
	cxReceipts []*types.CXReceipt, payout *big.Int, state *state.DB,
) (status WriteStatus, err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// Make sure no inconsistent state is leaked during insertion
	bc.mu.Lock()
	defer bc.mu.Unlock()

	currentBlock := bc.CurrentBlock()

	rawdb.WriteBlock(bc.db, block)

	root, err := state.Commit(bc.chainConfig.IsS3(block.Epoch()))
	if err != nil {
		return NonStatTy, err
	}
	triedb := bc.stateCache.TrieDB()

	// If we're running an archive node, always flush
	if bc.cacheConfig.Disabled {
		if err := triedb.Commit(root, false); err != nil {
			return NonStatTy, err
		}
	} else {
		// Full but not archive node, do proper garbage collection
		triedb.Reference(root, common.Hash{}) // metadata reference to keep trie alive
		bc.triegc.Push(root, -int64(block.NumberU64()))

		if current := block.NumberU64(); current > triesInMemory {
			// If we exceeded our memory allowance, flush matured singleton nodes to disk
			var (
				nodes, imgs = triedb.Size()
				limit       = common.StorageSize(bc.cacheConfig.TrieNodeLimit) * 1024 * 1024
			)
			if nodes > limit || imgs > 4*1024*1024 {
				triedb.Cap(limit - ethdb.IdealBatchSize)
			}
			// Find the next state trie we need to commit
			header := bc.GetHeaderByNumber(current - triesInMemory)
			chosen := header.Number().Uint64()

			// If we exceeded out time allowance, flush an entire trie to disk
			if bc.gcproc > bc.cacheConfig.TrieTimeLimit {
				// If we're exceeding limits but haven't reached a large enough memory gap,
				// warn the user that the system is becoming unstable.
				if chosen < lastWrite+triesInMemory && bc.gcproc >= 2*bc.cacheConfig.TrieTimeLimit {
					utils.Logger().Info().
						Dur("time", bc.gcproc).
						Dur("allowance", bc.cacheConfig.TrieTimeLimit).
						Float64("optimum", float64(chosen-lastWrite)/triesInMemory).
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
				triedb.Dereference(root.(common.Hash))
			}
		}
	}

	// Write other block data using a batch.
	// TODO: put following into a func
	/////////////////////////// START
	batch := bc.db.NewBatch()
	rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)

	//// Cross-shard txns
	epoch := block.Header().Epoch()
	if bc.chainConfig.IsCrossTx(block.Epoch()) {
		shardingConfig := shard.Schedule.InstanceForEpoch(epoch)
		shardNum := int(shardingConfig.NumShards())
		for i := 0; i < shardNum; i++ {
			if i == int(block.ShardID()) {
				continue
			}
			shardReceipts := GetToShardReceipts(cxReceipts, uint32(i))
			err := rawdb.WriteCXReceipts(batch, uint32(i), block.NumberU64(), block.Hash(), shardReceipts)
			if err != nil {
				utils.Logger().Debug().Err(err).Interface("shardReceipts", shardReceipts).Int("toShardID", i).Msg("WriteCXReceipts cannot write into database")
				return NonStatTy, err
			}
		}
		// Mark incomingReceipts in the block as spent
		bc.WriteCXReceiptsProofSpent(block.IncomingReceipts())
	}

	//// VRF + VDF
	//check non zero VRF field in header and add to local db
	if len(block.Vrf()) > 0 {
		vrfBlockNumbers, _ := bc.ReadEpochVrfBlockNums(block.Header().Epoch())
		if (len(vrfBlockNumbers) > 0) && (vrfBlockNumbers[len(vrfBlockNumbers)-1] == block.NumberU64()) {
			utils.Logger().Error().
				Str("number", block.Number().String()).
				Str("epoch", block.Header().Epoch().String()).
				Msg("VRF block number is already in local db")
		} else {
			vrfBlockNumbers = append(vrfBlockNumbers, block.NumberU64())
			err = bc.WriteEpochVrfBlockNums(block.Header().Epoch(), vrfBlockNumbers)
			if err != nil {
				utils.Logger().Error().
					Str("number", block.Number().String()).
					Str("epoch", block.Header().Epoch().String()).
					Msg("failed to write VRF block number to local db")
				return NonStatTy, err
			}
		}
	}

	//check non zero Vdf in header and add to local db
	if len(block.Vdf()) > 0 {
		err = bc.WriteEpochVdfBlockNum(block.Header().Epoch(), block.Number())
		if err != nil {
			utils.Logger().Error().
				Str("number", block.Number().String()).
				Str("epoch", block.Header().Epoch().String()).
				Msg("failed to write VDF block number to local db")
			return NonStatTy, err
		}
	}

	//// Shard State and Validator Update
	header := block.Header()
	if len(header.ShardState()) > 0 {
		// Write shard state for the new epoch
		epoch := new(big.Int).Add(header.Epoch(), common.Big1)
		newShardState, err := bc.WriteShardStateBytes(batch, epoch, header.ShardState())
		if err != nil {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("cannot store shard state")
			return NonStatTy, err
		}

		// Find all the active validator addresses and store them in db
		allActiveValidators := []common.Address{}
		processed := make(map[common.Address]struct{})
		for i := range newShardState.Shards {
			shard := newShardState.Shards[i]
			for j := range shard.Slots {
				slot := shard.Slots[j]
				if slot.TotalStake != nil { // For external validator
					_, ok := processed[slot.EcdsaAddress]
					if !ok {
						processed[slot.EcdsaAddress] = struct{}{}
						allActiveValidators = append(allActiveValidators, shard.Slots[j].EcdsaAddress)
					}
				}
			}
		}

		// Update active validators
		if err := bc.WriteActiveValidatorList(allActiveValidators); err != nil {
			return NonStatTy, err
		}

		// Update snapshots for all validators
		if err := bc.UpdateValidatorSnapshots(); err != nil {
			return NonStatTy, err
		}
	}

	// Do bookkeeping for new staking txns
	for _, tx := range block.StakingTransactions() {
		err = bc.UpdateStakingMetaData(tx, root)
		// keep offchain database consistency with onchain we need revert
		// but it should not happend unless local database corrupted
		if err != nil {
			utils.Logger().Debug().Msgf("oops, UpdateStakingMetaData failed, err: %+v", err)
			return NonStatTy, err
		}
	}

	// Update voting power of validators for all shards
	if block.ShardID() == 0 && len(block.Header().ShardState()) > 0 {
		shardState := new(shard.State)

		if shardState, err = shard.DecodeWrapper(block.Header().ShardState()); err == nil {
			if err = bc.UpdateValidatorVotingPower(shardState); err != nil {
				utils.Logger().Err(err).Msg("[UpdateValidatorVotingPower] Failed to update voting power")
			}
		} else {
			utils.Logger().Err(err).Msg("[UpdateValidatorVotingPower] Failed to decode shard state")
		}
	}

	// Writing validator stats (for uptime recording) for shard 0
	if block.ShardID() == 0 && bc.chainConfig.IsStaking(block.Epoch()) {
		parentHeader := bc.GetHeaderByHash(block.ParentHash())
		if parentHeader == nil {
			return NonStatTy, errors.New("no parent found for uptime accounting")
		}
		shardState, err := bc.ReadShardState(parentHeader.Epoch())
		if err == nil {
			committee := shardState.FindCommitteeByID(block.Header().ShardID())
			if committee == nil {
				return NonStatTy, errors.New("no shard found for cross-link")
			}

			members := []*bls2.PublicKey{}
			for _, slot := range committee.Slots {
				if slot.TotalStake != nil {
					pubKey := &bls2.PublicKey{}
					err := pubKey.Deserialize(slot.BlsPublicKey[:])
					if err != nil {
						return NonStatTy, err
					}
					members = append(members, pubKey)
				}
			}

			mask, _ := bls.NewMask(members, nil)
			mask.SetMask(block.Header().LastCommitBitmap())

			if err = bc.UpdateValidatorUptime(committee.Slots, mask); err != nil {
				utils.Logger().Err(err)
			}
		} else {
			return NonStatTy, errors.New("failed reading shard state for uptime accounting")
		}
	}

	//// Writing validator stats (for uptime recording) for for other shards
	if header.ShardID() == 0 && bc.chainConfig.IsStaking(block.Epoch()) && len(header.CrossLinks()) > 0 {
		header.Logger(utils.Logger()).Debug().Msg("[insertChain/crosslinks] writing crosslinks into blockchain...")
		crossLinks := &types.CrossLinks{}
		err = rlp.DecodeBytes(header.CrossLinks(), crossLinks)
		if err != nil {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("[insertChain/crosslinks] cannot parse cross links")
			return NonStatTy, err
		}
		if !crossLinks.IsSorted() {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("[insertChain/crosslinks] cross links are not sorted")
			return NonStatTy, errors.New("proposed cross links are not sorted")
		}
		for _, crossLink := range *crossLinks {
			// Writing validator stats (for uptime recording) for other shards
			if bc.chainConfig.IsStaking(crossLink.Epoch()) {
				shardState, err := bc.ReadShardState(crossLink.Epoch())
				if err == nil {
					committee := shardState.FindCommitteeByID(crossLink.ShardID())
					if committee == nil {
						return NonStatTy, errors.New("no shard found for cross-link")
					}

					members := []*bls2.PublicKey{}
					for _, slot := range committee.Slots {
						if slot.TotalStake != nil {
							pubKey := &bls2.PublicKey{}
							err := pubKey.Deserialize(slot.BlsPublicKey[:])
							if err != nil {
								return NonStatTy, err
							}
							members = append(members, pubKey)
						}
					}

					mask, _ := bls.NewMask(members, nil)
					mask.SetMask(crossLink.Bitmap())

					if err = bc.UpdateValidatorUptime(committee.Slots, mask); err != nil {
						utils.Logger().Err(err)
					}
				} else {
					return NonStatTy, errors.New("failed reading shard state for uptime accounting")
				}
			}

			// Process crosslink
			if err := bc.WriteCrossLinks(types.CrossLinks{crossLink}); err == nil {
				utils.Logger().Info().Uint64("blockNum", crossLink.BlockNum()).Uint32("shardID", crossLink.ShardID()).Msg("[insertChain/crosslinks] Cross Link Added to Beaconchain")
			}
			bc.LastContinuousCrossLink(crossLink)
		}
		//clean/update local database cache after crosslink inserted into blockchain
		num, err := bc.DeleteCommittedFromPendingCrossLinks(*crossLinks)
		utils.Logger().Debug().Msgf("DeleteCommittedFromPendingCrossLinks, crosslinks in header %d,  pending crosslinks: %d, error: %+v", len(*crossLinks), num, err)
	}

	isFirstTimeStaking := bc.chainConfig.IsStaking(new(big.Int).Add(block.Epoch(), big.NewInt(1))) &&
		len(block.Header().ShardState()) > 0 &&
		!bc.chainConfig.IsStaking(block.Epoch())

	curHeader := bc.CurrentHeader()
	if isFirstTimeStaking &&
		curHeader.ShardID() == shard.BeaconChainShardID {
		bc.WriteBlockRewardAccumulator(big.NewInt(0), curHeader.Number().Uint64())
	}

	if payout != nil &&
		curHeader.ShardID() == shard.BeaconChainShardID &&
		bc.chainConfig.IsStaking(block.Epoch()) {
		bc.WriteBlockRewardAccumulator(payout, block.Number().Uint64())
	}
	/////////////////////////// END

	// If the total difficulty is higher than our known, add it to the canonical chain
	// Second clause in the if statement reduces the vulnerability to selfish mining.
	// Please refer to http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	// TODO: Remove reorg code, it's not used in our code
	reorg := true
	if reorg {
		// Reorganise the chain if the parent is not the head block
		if block.ParentHash() != currentBlock.Hash() {
			if err := bc.reorg(currentBlock, block); err != nil {
				return NonStatTy, err
			}
		}
		// Write the positional metadata for transaction/receipt lookups and preimages
		rawdb.WriteTxLookupEntries(batch, block)
		rawdb.WritePreimages(batch, block.NumberU64(), state.Preimages())

		// write the positional metadata for CXReceipts lookups
		rawdb.WriteCxLookupEntries(batch, block)

		status = CanonStatTy
	} else {
		status = SideStatTy
	}
	if err := batch.Write(); err != nil {
		return NonStatTy, err
	}

	// Set new head.
	if status == CanonStatTy {
		bc.insert(block)
	}
	bc.futureBlocks.Remove(block.Hash())
	return status, nil
}

// InsertChain attempts to insert the given batch of blocks in to the canonical
// chain or, otherwise, create a fork. If an error is returned it will return
// the index number of the failing block as well an error describing what went
// wrong.
//
// After insertion is done, all accumulated events will be fired.
func (bc *BlockChain) InsertChain(chain types.Blocks, verifyHeaders bool) (int, error) {
	n, events, logs, err := bc.insertChain(chain, verifyHeaders)
	bc.PostChainEvents(events, logs)
	return n, err
}

// insertChain will execute the actual chain insertion and event aggregation. The
// only reason this method exists as a separate one is to make locking cleaner
// with deferred statements.
func (bc *BlockChain) insertChain(chain types.Blocks, verifyHeaders bool) (int, []interface{}, []*types.Log, error) {
	// Sanity check that we have something meaningful to import
	if len(chain) == 0 {
		return 0, nil, nil, nil
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
	// Pre-checks passed, start the full block imports
	bc.wg.Add(1)
	defer bc.wg.Done()

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

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
			err = bc.Validator().ValidateBody(block)
		}
		switch {
		case err == ErrKnownBlock:
			// Block and state both already known. However if the current block is below
			// this number we did a rollback and we should reimport it nonetheless.
			if bc.CurrentBlock().NumberU64() >= block.NumberU64() {
				stats.ignored++
				continue
			}

		case err == consensus_engine.ErrFutureBlock:
			// Allow up to MaxFuture second in the future blocks. If this limit is exceeded
			// the chain is discarded and processed at a later time if given.
			max := big.NewInt(time.Now().Unix() + maxTimeFutureBlocks)
			if block.Time().Cmp(max) > 0 {
				return i, events, coalescedLogs, fmt.Errorf("future block: %v > %v", block.Time(), max)
			}
			bc.futureBlocks.Add(block.Hash(), block)
			stats.queued++
			continue

		case err == consensus_engine.ErrUnknownAncestor && bc.futureBlocks.Contains(block.ParentHash()):
			bc.futureBlocks.Add(block.Hash(), block)
			stats.queued++
			continue

		case err == consensus_engine.ErrPrunedAncestor:
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
				bc.chainmu.Unlock()
				_, evs, logs, err := bc.insertChain(winner, true /* verifyHeaders */)
				bc.chainmu.Lock()
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
		state, err := state.New(parent.Root(), bc.stateCache)
		if err != nil {
			return i, events, coalescedLogs, err
		}

		// Process block using the parent state as reference point.
		receipts, cxReceipts, logs, usedGas, payout, err := bc.processor.Process(block, state, bc.vmConfig)
		if err != nil {
			bc.reportBlock(block, receipts, err)
			return i, events, coalescedLogs, err
		}

		// Validate the state using the default validator
		err = bc.Validator().ValidateState(block, parent, state, receipts, cxReceipts, usedGas)
		if err != nil {
			bc.reportBlock(block, receipts, err)
			return i, events, coalescedLogs, err
		}
		proctime := time.Since(bstart)

		// Write the block to the chain and get the status.
		status, err := bc.WriteBlockWithState(block, receipts, cxReceipts, payout, state)
		if err != nil {
			return i, events, coalescedLogs, err
		}
		logger := utils.Logger().With().
			Str("number", block.Number().String()).
			Str("hash", block.Hash().Hex()).
			Int("uncles", len(block.Uncles())).
			Int("txs", len(block.Transactions())).
			Uint64("gas", block.GasUsed()).
			Str("elapsed", common.PrettyDuration(time.Since(bstart)).String()).
			Logger()

		switch status {
		case CanonStatTy:
			logger.Info().Msg("Inserted new block")
			coalescedLogs = append(coalescedLogs, logs...)
			blockInsertTimer.UpdateSince(bstart)
			events = append(events, ChainEvent{block, block.Hash(), logs})
			lastCanon = block

			// Only count canonical blocks for GC processing time
			bc.gcproc += proctime
		case SideStatTy:
			logger.Debug().Msg("Inserted forked block")
			blockInsertTimer.UpdateSince(bstart)
			events = append(events, ChainSideEvent{block})
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
	queued, processed, ignored int
	usedGas                    uint64
	lastIndex                  int
	startTime                  mclock.AbsTime
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
		if st.ignored > 0 {
			context = context.Int("ignored", st.ignored)
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

// reorgs takes two blocks, an old chain and a new chain and will reconstruct the blocks and inserts them
// to be part of the new canonical chain and accumulates potential missing transactions and post an
// event about them
func (bc *BlockChain) reorg(oldBlock, newBlock *types.Block) error {
	var (
		newChain    types.Blocks
		oldChain    types.Blocks
		commonBlock *types.Block
		deletedTxs  types.Transactions
		deletedLogs []*types.Log
		// collectLogs collects the logs that were generated during the
		// processing of the block that corresponds with the given hash.
		// These logs are later announced as deleted.
		collectLogs = func(hash common.Hash) {
			// Coalesce logs and set 'Removed'.
			number := bc.hc.GetBlockNumber(hash)
			if number == nil {
				return
			}
			receipts := rawdb.ReadReceipts(bc.db, hash, *number)
			for _, receipt := range receipts {
				for _, log := range receipt.Logs {
					del := *log
					del.Removed = true
					deletedLogs = append(deletedLogs, &del)
				}
			}
		}
	)

	// first reduce whoever is higher bound
	if oldBlock.NumberU64() > newBlock.NumberU64() {
		// reduce old chain
		for ; oldBlock != nil && oldBlock.NumberU64() != newBlock.NumberU64(); oldBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1) {
			oldChain = append(oldChain, oldBlock)
			deletedTxs = append(deletedTxs, oldBlock.Transactions()...)

			collectLogs(oldBlock.Hash())
		}
	} else {
		// reduce new chain and append new chain blocks for inserting later on
		for ; newBlock != nil && newBlock.NumberU64() != oldBlock.NumberU64(); newBlock = bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1) {
			newChain = append(newChain, newBlock)
		}
	}
	if oldBlock == nil {
		return fmt.Errorf("Invalid old chain")
	}
	if newBlock == nil {
		return fmt.Errorf("Invalid new chain")
	}

	for {
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}

		oldChain = append(oldChain, oldBlock)
		newChain = append(newChain, newBlock)
		deletedTxs = append(deletedTxs, oldBlock.Transactions()...)
		collectLogs(oldBlock.Hash())

		oldBlock, newBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1), bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1)
		if oldBlock == nil {
			return fmt.Errorf("Invalid old chain")
		}
		if newBlock == nil {
			return fmt.Errorf("Invalid new chain")
		}
	}
	// Ensure the user sees large reorgs
	if len(oldChain) > 0 && len(newChain) > 0 {
		logEvent := utils.Logger().Debug()
		if len(oldChain) > 63 {
			logEvent = utils.Logger().Warn()
		}
		logEvent.
			Str("number", commonBlock.Number().String()).
			Str("hash", commonBlock.Hash().Hex()).
			Int("drop", len(oldChain)).
			Str("dropfrom", oldChain[0].Hash().Hex()).
			Int("add", len(newChain)).
			Str("addfrom", newChain[0].Hash().Hex()).
			Msg("Chain split detected")
	} else {
		utils.Logger().Error().
			Str("oldnum", oldBlock.Number().String()).
			Str("oldhash", oldBlock.Hash().Hex()).
			Str("newnum", newBlock.Number().String()).
			Str("newhash", newBlock.Hash().Hex()).
			Msg("Impossible reorg, please file an issue")
	}
	// Insert the new chain, taking care of the proper incremental order
	var addedTxs types.Transactions
	for i := len(newChain) - 1; i >= 0; i-- {
		// insert the block in the canonical way, re-writing history
		bc.insert(newChain[i])
		// write lookup entries for hash based transaction/receipt searches
		rawdb.WriteTxLookupEntries(bc.db, newChain[i])
		addedTxs = append(addedTxs, newChain[i].Transactions()...)
	}
	// calculate the difference between deleted and added transactions
	diff := types.TxDifference(deletedTxs, addedTxs)
	// When transactions get deleted from the database that means the
	// receipts that were created in the fork must also be deleted
	batch := bc.db.NewBatch()
	for _, tx := range diff {
		rawdb.DeleteTxLookupEntry(batch, tx.Hash())
	}
	batch.Write()

	if len(deletedLogs) > 0 {
		go bc.rmLogsFeed.Send(RemovedLogsEvent{deletedLogs})
	}
	if len(oldChain) > 0 {
		go func() {
			for _, block := range oldChain {
				bc.chainSideFeed.Send(ChainSideEvent{Block: block})
			}
		}()
	}

	return nil
}

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (bc *BlockChain) PostChainEvents(events []interface{}, logs []*types.Log) {
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
		}
	}
}

func (bc *BlockChain) update() {
	futureTimer := time.NewTicker(5 * time.Second)
	defer futureTimer.Stop()
	for {
		select {
		case <-futureTimer.C:
			bc.procFutureBlocks()
		case <-bc.quit:
			return
		}
	}
}

// BadBlocks returns a list of the last 'bad blocks' that the client has seen on the network
func (bc *BlockChain) BadBlocks() []*types.Block {
	blocks := make([]*types.Block, 0, bc.badBlocks.Len())
	for _, hash := range bc.badBlocks.Keys() {
		if blk, exist := bc.badBlocks.Peek(hash); exist {
			block := blk.(*types.Block)
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// addBadBlock adds a bad block to the bad-block LRU cache
func (bc *BlockChain) addBadBlock(block *types.Block) {
	bc.badBlocks.Add(block.Hash(), block)
}

// reportBlock logs a bad block error.
func (bc *BlockChain) reportBlock(block *types.Block, receipts types.Receipts, err error) {
	bc.addBadBlock(block)

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
`, bc.chainConfig, block.Number(), block.Epoch(), len(block.Transactions()), len(block.StakingTransactions()), block.Hash(), receiptString, err)
	for i, tx := range block.StakingTransactions() {
		utils.Logger().Error().Msgf("StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage())
	}
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verify nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (bc *BlockChain) InsertHeaderChain(chain []*block.Header, checkFreq int) (int, error) {
	start := time.Now()
	if i, err := bc.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

	// Make sure only one thread manipulates the chain at once
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	bc.wg.Add(1)
	defer bc.wg.Done()

	whFunc := func(header *block.Header) error {
		bc.mu.Lock()
		defer bc.mu.Unlock()

		_, err := bc.hc.WriteHeader(header)
		return err
	}

	return bc.hc.InsertHeaderChain(chain, whFunc, start)
}

// writeHeader writes a header into the local chain, given that its parent is
// already known. If the total difficulty of the newly inserted header becomes
// greater than the current known TD, the canonical chain is re-routed.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (bc *BlockChain) writeHeader(header *block.Header) error {
	bc.wg.Add(1)
	defer bc.wg.Done()

	bc.mu.Lock()
	defer bc.mu.Unlock()

	_, err := bc.hc.WriteHeader(header)
	return err
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (bc *BlockChain) CurrentHeader() *block.Header {
	return bc.hc.CurrentHeader()
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (bc *BlockChain) GetTd(hash common.Hash, number uint64) *big.Int {
	return bc.hc.GetTd(hash, number)
}

// GetTdByHash retrieves a block's total difficulty in the canonical chain from the
// database by hash, caching it if found.
func (bc *BlockChain) GetTdByHash(hash common.Hash) *big.Int {
	return bc.hc.GetTdByHash(hash)
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *block.Header {
	return bc.hc.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *block.Header {
	return bc.hc.GetHeaderByHash(hash)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (bc *BlockChain) HasHeader(hash common.Hash, number uint64) bool {
	return bc.hc.HasHeader(hash, number)
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (bc *BlockChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return bc.hc.GetBlockHashesFromHash(hash, max)
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (bc *BlockChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	return bc.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (bc *BlockChain) GetHeaderByNumber(number uint64) *block.Header {
	return bc.hc.GetHeaderByNumber(number)
}

// Config retrieves the blockchain's chain configuration.
func (bc *BlockChain) Config() *params.ChainConfig { return bc.chainConfig }

// Engine retrieves the blockchain's consensus engine.
func (bc *BlockChain) Engine() consensus_engine.Engine { return bc.engine }

// SubscribeRemovedLogsEvent registers a subscription of RemovedLogsEvent.
func (bc *BlockChain) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return bc.scope.Track(bc.rmLogsFeed.Subscribe(ch))
}

// SubscribeChainEvent registers a subscription of ChainEvent.
func (bc *BlockChain) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return bc.scope.Track(bc.chainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
func (bc *BlockChain) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	return bc.scope.Track(bc.chainSideFeed.Subscribe(ch))
}

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (bc *BlockChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}

// ReadShardState retrieves sharding state given the epoch number.
func (bc *BlockChain) ReadShardState(epoch *big.Int) (*shard.State, error) {
	cacheKey := string(epoch.Bytes())
	if cached, ok := bc.shardStateCache.Get(cacheKey); ok {
		shardState := cached.(*shard.State)
		return shardState, nil
	}
	shardState, err := rawdb.ReadShardState(bc.db, epoch)
	if err != nil {
		return nil, err
	}
	bc.shardStateCache.Add(cacheKey, shardState)
	return shardState, nil
}

// WriteShardStateBytes saves the given sharding state under the given epoch number.
func (bc *BlockChain) WriteShardStateBytes(db rawdb.DatabaseWriter,
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

// ReadLastCommits retrieves last commits.
func (bc *BlockChain) ReadLastCommits() ([]byte, error) {
	if cached, ok := bc.lastCommitsCache.Get("lastCommits"); ok {
		lastCommits := cached.([]byte)
		return lastCommits, nil
	}
	lastCommits, err := rawdb.ReadLastCommits(bc.db)
	if err != nil {
		return nil, err
	}
	return lastCommits, nil
}

// WriteLastCommits saves the commits of last block.
func (bc *BlockChain) WriteLastCommits(lastCommits []byte) error {
	err := rawdb.WriteLastCommits(bc.db, lastCommits)
	if err != nil {
		return err
	}
	bc.lastCommitsCache.Add("lastCommits", lastCommits)
	return nil
}

// GetVdfByNumber retrieves the rand seed given the block number, return 0 if not exist
func (bc *BlockChain) GetVdfByNumber(number uint64) []byte {
	header := bc.GetHeaderByNumber(number)
	if header == nil {
		return []byte{}
	}

	return header.Vdf()
}

// GetVrfByNumber retrieves the randomness preimage given the block number, return 0 if not exist
func (bc *BlockChain) GetVrfByNumber(number uint64) []byte {
	header := bc.GetHeaderByNumber(number)
	if header == nil {
		return []byte{}
	}
	return header.Vrf()
}

// ChainDb returns the database
func (bc *BlockChain) ChainDb() ethdb.Database { return bc.db }

// GetEpochBlockNumber returns the first block number of the given epoch.
func (bc *BlockChain) GetEpochBlockNumber(epoch *big.Int) (*big.Int, error) {
	// Try cache first
	cacheKey := string(epoch.Bytes())
	if cachedValue, ok := bc.epochCache.Get(cacheKey); ok {
		return (&big.Int{}).SetBytes([]byte(cachedValue.(string))), nil
	}
	blockNum, err := rawdb.ReadEpochBlockNumber(bc.db, epoch)
	if err != nil {
		return nil, ctxerror.New("cannot read epoch block number from database",
			"epoch", epoch,
		).WithCause(err)
	}
	cachedValue := []byte(blockNum.Bytes())
	bc.epochCache.Add(cacheKey, cachedValue)
	return blockNum, nil
}

// StoreEpochBlockNumber stores the given epoch-first block number.
func (bc *BlockChain) StoreEpochBlockNumber(
	epoch *big.Int, blockNum *big.Int,
) error {
	cacheKey := string(epoch.Bytes())
	cachedValue := []byte(blockNum.Bytes())
	bc.epochCache.Add(cacheKey, cachedValue)
	if err := rawdb.WriteEpochBlockNumber(bc.db, epoch, blockNum); err != nil {
		return ctxerror.New("cannot write epoch block number to database",
			"epoch", epoch,
			"epochBlockNum", blockNum,
		).WithCause(err)
	}
	return nil
}

// ReadEpochVrfBlockNums retrieves block numbers with valid VRF for the specified epoch
func (bc *BlockChain) ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error) {
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

// WriteEpochVrfBlockNums saves block numbers with valid VRF for the specified epoch
func (bc *BlockChain) WriteEpochVrfBlockNums(epoch *big.Int, vrfNumbers []uint64) error {
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

// ReadEpochVdfBlockNum retrieves block number with valid VDF for the specified epoch
func (bc *BlockChain) ReadEpochVdfBlockNum(epoch *big.Int) (*big.Int, error) {
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

// WriteEpochVdfBlockNum saves block number with valid VDF for the specified epoch
func (bc *BlockChain) WriteEpochVdfBlockNum(epoch *big.Int, blockNum *big.Int) error {
	err := rawdb.WriteEpochVdfBlockNum(bc.db, epoch, blockNum.Bytes())
	if err != nil {
		return err
	}

	bc.randomnessCache.Add("vdf-"+string(epoch.Bytes()), blockNum.Bytes())
	return nil
}

// WriteCrossLinks saves the hashes of crosslinks by shardID and blockNum combination key
func (bc *BlockChain) WriteCrossLinks(cls []types.CrossLink) error {
	var err error
	for i := 0; i < len(cls); i++ {
		cl := cls[i]
		err = rawdb.WriteCrossLinkShardBlock(bc.db, cl.ShardID(), cl.BlockNum(), cl.Serialize())
	}
	return err
}

// DeleteCrossLinks removes the hashes of crosslinks by shardID and blockNum combination key
func (bc *BlockChain) DeleteCrossLinks(cls []types.CrossLink) error {
	var err error
	for i := 0; i < len(cls); i++ {
		cl := cls[i]
		err = rawdb.DeleteCrossLinkShardBlock(bc.db, cl.ShardID(), cl.BlockNum())
	}
	return err
}

// ReadCrossLink retrieves crosslink given shardID and blockNum.
func (bc *BlockChain) ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error) {
	bytes, err := rawdb.ReadCrossLinkShardBlock(bc.db, shardID, blockNum)
	if err != nil {
		return nil, err
	}
	crossLink, err := types.DeserializeCrossLink(bytes)

	return crossLink, err
}

// LastContinuousCrossLink saves the last crosslink of a shard
// This function will update the latest crosslink in the sense that
// any previous block's crosslink is received up to this point
// there is no missing hole between genesis to this crosslink of given shardID
func (bc *BlockChain) LastContinuousCrossLink(cl types.CrossLink) error {
	if !bc.Config().IsCrossLink(cl.Epoch()) {
		return errors.New("Trying to write last continuous cross link with epoch before cross link starting epoch")
	}

	cl0, err := bc.ReadShardLastCrossLink(cl.ShardID())
	if cl0 == nil {
		rawdb.WriteShardLastCrossLink(bc.db, cl.ShardID(), cl.Serialize())
		return nil
	}
	if err != nil {
		return err
	}
	newCheckpoint := uint64(0)
	for i := cl0.BlockNum() + 1; i < cl.BlockNum(); i++ {
		tmp, err := bc.ReadCrossLink(cl.ShardID(), i)
		if err != nil || tmp == nil {
			newCheckpoint = i - 1
			break
		}
		if i == cl.BlockNum()-1 {
			newCheckpoint = cl.BlockNum()
		}
	}

	if newCheckpoint > 0 {
		utils.Logger().Debug().Msgf("LastContinuousCrossLink: latest checkpoint blockNum %d", newCheckpoint)
		cln, err := bc.ReadCrossLink(cl.ShardID(), newCheckpoint)
		if err != nil {
			return err
		}
		return rawdb.WriteShardLastCrossLink(bc.db, cln.ShardID(), cln.Serialize())
	}
	return nil
}

// ReadShardLastCrossLink retrieves the last crosslink of a shard.
func (bc *BlockChain) ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error) {
	bytes, err := rawdb.ReadShardLastCrossLink(bc.db, shardID)
	if err != nil {
		return nil, err
	}
	crossLink, err := types.DeserializeCrossLink(bytes)

	return crossLink, err
}

// ReadPendingCrossLinks retrieves pending crosslinks
func (bc *BlockChain) ReadPendingCrossLinks() ([]types.CrossLink, error) {
	bytes := []byte{}
	if cached, ok := bc.pendingCrossLinksCache.Get("pendingCLs"); ok {
		bytes = cached.([]byte)
	} else {
		bytes, err := rawdb.ReadPendingCrossLinks(bc.db)
		if err != nil || len(bytes) == 0 {
			utils.Logger().Info().Err(err).Int("dataLen", len(bytes)).Msg("ReadPendingCrossLinks")
			return nil, err
		}
	}
	cls := []types.CrossLink{}
	if err := rlp.DecodeBytes(bytes, &cls); err != nil {
		utils.Logger().Error().Err(err).Msg("Invalid pending crosslink RLP decoding")
		return nil, err
	}
	return cls, nil
}

// WritePendingCrossLinks saves the pending crosslinks
func (bc *BlockChain) WritePendingCrossLinks(crossLinks []types.CrossLink) error {
	// deduplicate crosslinks if any
	m := map[uint32]map[uint64](types.CrossLink){}
	for _, cl := range crossLinks {
		if _, ok := m[cl.ShardID()]; !ok {
			m[cl.ShardID()] = map[uint64](types.CrossLink){}
		}
		m[cl.ShardID()][cl.BlockNum()] = cl
	}

	cls := []types.CrossLink{}
	for _, m1 := range m {
		for _, cl := range m1 {
			cls = append(cls, cl)
		}
	}
	utils.Logger().Debug().Msgf("[WritePendingCrossLinks] Before Dedup has %d cls, after Dedup has %d cls", len(crossLinks), len(cls))

	bytes, err := rlp.EncodeToBytes(cls)
	if err != nil {
		utils.Logger().Error().Msg("[WritePendingCrossLinks] Failed to encode pending crosslinks")
		return err
	}
	if err := rawdb.WritePendingCrossLinks(bc.db, bytes); err != nil {
		return err
	}
	by, err := rlp.EncodeToBytes(cls)
	if err == nil {
		bc.pendingCrossLinksCache.Add("pendingCLs", by)
	}
	return nil
}

// AddPendingCrossLinks appends pending crosslinks
func (bc *BlockChain) AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error) {
	cls, err := bc.ReadPendingCrossLinks()
	if err != nil || len(cls) == 0 {
		err := bc.WritePendingCrossLinks(pendingCLs)
		return len(pendingCLs), err
	}
	cls = append(cls, pendingCLs...)
	err = bc.WritePendingCrossLinks(cls)
	return len(cls), err
}

// DeleteCommittedFromPendingCrossLinks delete pending crosslinks that already committed (i.e. passed in the params)
func (bc *BlockChain) DeleteCommittedFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error) {
	cls, err := bc.ReadPendingCrossLinks()
	if err != nil || len(cls) == 0 {
		return 0, err
	}

	m := map[uint32]map[uint64](struct{}){}
	for _, cl := range crossLinks {
		if _, ok := m[cl.ShardID()]; !ok {
			m[cl.ShardID()] = map[uint64](struct{}){}
		}
		m[cl.ShardID()][cl.BlockNum()] = struct{}{}
	}

	pendingCLs := []types.CrossLink{}
Loop:
	for _, cl := range cls {
		if _, ok := m[cl.ShardID()]; ok {
			if _, ok1 := m[cl.ShardID()][cl.BlockNum()]; ok1 {
				continue Loop
			}
		}
		pendingCLs = append(pendingCLs, cl)
	}
	err = bc.WritePendingCrossLinks(pendingCLs)
	return len(pendingCLs), err
}

// IsSameLeaderAsPreviousBlock retrieves a block from the database by number, caching it
func (bc *BlockChain) IsSameLeaderAsPreviousBlock(block *types.Block) bool {
	if IsEpochBlock(block) {
		return false
	}

	previousHeader := bc.GetHeaderByNumber(block.NumberU64() - 1)
	return block.Coinbase() == previousHeader.Coinbase()
}

// ChainDB ...
// TODO(ricl): in eth, this is not exposed. I expose it here because I need it in Harmony object.
// In eth, chainDB is initialized within Ethereum object
func (bc *BlockChain) ChainDB() ethdb.Database {
	return bc.db
}

// GetVMConfig returns the block chain VM config.
func (bc *BlockChain) GetVMConfig() *vm.Config {
	return &bc.vmConfig
}

// GetToShardReceipts filters the cross shard receipts with given destination shardID
func GetToShardReceipts(cxReceipts types.CXReceipts, shardID uint32) types.CXReceipts {
	cxs := types.CXReceipts{}
	for i := range cxReceipts {
		cx := cxReceipts[i]
		if cx.ToShardID == shardID {
			cxs = append(cxs, cx)
		}
	}
	return cxs
}

// ReadCXReceipts retrieves the cross shard transaction receipts of a given shard
func (bc *BlockChain) ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error) {
	cxs, err := rawdb.ReadCXReceipts(bc.db, shardID, blockNum, blockHash)
	if err != nil || len(cxs) == 0 {
		return nil, err
	}
	return cxs, nil
}

// WriteCXReceipts saves the cross shard transaction receipts of a given shard
func (bc *BlockChain) WriteCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash, receipts types.CXReceipts) error {
	return rawdb.WriteCXReceipts(bc.db, shardID, blockNum, blockHash, receipts)
}

// CXMerkleProof calculates the cross shard transaction merkle proof of a given destination shard
func (bc *BlockChain) CXMerkleProof(shardID uint32, block *types.Block) (*types.CXMerkleProof, error) {
	proof := &types.CXMerkleProof{BlockNum: block.Number(), BlockHash: block.Hash(), ShardID: block.ShardID(), CXReceiptHash: block.Header().OutgoingReceiptHash(), CXShardHashes: []common.Hash{}, ShardIDs: []uint32{}}
	cxs, err := rawdb.ReadCXReceipts(bc.db, shardID, block.NumberU64(), block.Hash())

	if err != nil || cxs == nil {
		return nil, err
	}

	epoch := block.Header().Epoch()
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

// LatestCXReceiptsCheckpoint returns the latest checkpoint
func (bc *BlockChain) LatestCXReceiptsCheckpoint(shardID uint32) uint64 {
	blockNum, _ := rawdb.ReadCXReceiptsProofUnspentCheckpoint(bc.db, shardID)
	return blockNum
}

// NextCXReceiptsCheckpoint returns the next checkpoint blockNum
func (bc *BlockChain) NextCXReceiptsCheckpoint(currentNum uint64, shardID uint32) uint64 {
	lastCheckpoint, _ := rawdb.ReadCXReceiptsProofUnspentCheckpoint(bc.db, shardID)
	newCheckpoint := lastCheckpoint

	// the new checkpoint will not exceed currentNum+1
	for num := lastCheckpoint; num <= currentNum+1; num++ {
		newCheckpoint = num
		by, _ := rawdb.ReadCXReceiptsProofSpent(bc.db, shardID, num)
		if by == rawdb.NAByte {
			// TODO chao: check if there is IncompingReceiptsHash in crosslink header
			// if the rootHash is non-empty, it means incomingReceipts are not delivered
			// otherwise, it means there is no cross-shard transactions for this block
			continue
		}
		if by == rawdb.SpentByte {
			continue
		}
		// the first unspent blockHash found, break the loop
		break
	}
	return newCheckpoint
}

// updateCXReceiptsCheckpoints will update the checkpoint and clean spent receipts upto checkpoint
func (bc *BlockChain) updateCXReceiptsCheckpoints(shardID uint32, currentNum uint64) {
	lastCheckpoint, err := rawdb.ReadCXReceiptsProofUnspentCheckpoint(bc.db, shardID)
	if err != nil {
		utils.Logger().Warn().Msg("[updateCXReceiptsCheckpoints] Cannot get lastCheckpoint")
	}
	newCheckpoint := bc.NextCXReceiptsCheckpoint(currentNum, shardID)
	if lastCheckpoint == newCheckpoint {
		return
	}
	utils.Logger().Debug().Uint64("lastCheckpoint", lastCheckpoint).Uint64("newCheckpont", newCheckpoint).Msg("[updateCXReceiptsCheckpoints]")
	for num := lastCheckpoint; num < newCheckpoint; num++ {
		rawdb.DeleteCXReceiptsProofSpent(bc.db, shardID, num)
	}
	rawdb.WriteCXReceiptsProofUnspentCheckpoint(bc.db, shardID, newCheckpoint)
}

// WriteCXReceiptsProofSpent mark the CXReceiptsProof list with given unspent status
// true: unspent, false: spent
func (bc *BlockChain) WriteCXReceiptsProofSpent(cxps []*types.CXReceiptsProof) {
	for _, cxp := range cxps {
		rawdb.WriteCXReceiptsProofSpent(bc.db, cxp)
	}
}

// IsSpent checks whether a CXReceiptsProof is unspent
func (bc *BlockChain) IsSpent(cxp *types.CXReceiptsProof) bool {
	shardID := cxp.MerkleProof.ShardID
	blockNum := cxp.MerkleProof.BlockNum.Uint64()
	by, _ := rawdb.ReadCXReceiptsProofSpent(bc.db, shardID, blockNum)
	if by == rawdb.SpentByte || cxp.MerkleProof.BlockNum.Uint64() < bc.LatestCXReceiptsCheckpoint(cxp.MerkleProof.ShardID) {
		return true
	}
	return false
}

// UpdateCXReceiptsCheckpointsByBlock cleans checkpoints and update latest checkpoint based on incomingReceipts of the given block
func (bc *BlockChain) UpdateCXReceiptsCheckpointsByBlock(block *types.Block) {
	m := make(map[uint32]uint64)
	for _, cxp := range block.IncomingReceipts() {
		shardID := cxp.MerkleProof.ShardID
		blockNum := cxp.MerkleProof.BlockNum.Uint64()
		if _, ok := m[shardID]; !ok {
			m[shardID] = blockNum
		} else if m[shardID] < blockNum {
			m[shardID] = blockNum
		}
	}

	for k, v := range m {
		utils.Logger().Debug().Uint32("shardID", k).Uint64("blockNum", v).Msg("[CleanCXReceiptsCheckpoints] Cleaning CXReceiptsProof upto")
		bc.updateCXReceiptsCheckpoints(k, v)
	}
}

// ReadTxLookupEntry returns where the given transaction resides in the chain,
// as a (block hash, block number, index in transaction list) triple.
// returns 0, 0 if not found
func (bc *BlockChain) ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64) {
	return rawdb.ReadTxLookupEntry(bc.db, txID)
}

// ReadValidatorInformationAt reads staking information of given validatorWrapper at a specific state root
func (bc *BlockChain) ReadValidatorInformationAt(addr common.Address, root common.Hash) (*staking.ValidatorWrapper, error) {
	state, err := bc.StateAt(root)
	if err != nil || state == nil {
		return nil, err
	}
	wrapper := state.GetStakingInfo(addr)
	if wrapper == nil {
		return nil, fmt.Errorf("ValidatorData not found: %v", addr)
	}
	return wrapper, nil
}

// ReadValidatorInformation reads staking information of given validator address
func (bc *BlockChain) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	return bc.ReadValidatorInformationAt(addr, bc.CurrentBlock().Root())
}

// ReadValidatorSnapshot reads the snapshot staking information of given validator address
func (bc *BlockChain) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorWrapper, error) {
	if cached, ok := bc.validatorCache.Get("validator-snapshot-" + string(addr.Bytes())); ok {
		by := cached.([]byte)
		v := staking.ValidatorWrapper{}
		if err := rlp.DecodeBytes(by, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	return rawdb.ReadValidatorSnapshot(bc.db, addr)
}

// writeValidatorSnapshots writes the snapshot of provided list of validators
func (bc *BlockChain) writeValidatorSnapshots(addrs []common.Address) error {
	// Read all validator's current data
	validators := []*staking.ValidatorWrapper{}
	for _, addr := range addrs {
		validator, err := bc.ReadValidatorInformation(addr)
		if err != nil {
			return err
		}
		validators = append(validators, validator)
	}

	// Batch write the current data as snapshot
	batch := bc.db.NewBatch()
	for i := range validators {
		err := rawdb.WriteValidatorSnapshot(batch, validators[i])
		if err != nil {
			return err
		}
	}
	if err := batch.Write(); err != nil {
		return err
	}

	// Update cache
	for i := range validators {
		by, err := rlp.EncodeToBytes(validators[i])
		if err == nil {
			bc.validatorCache.Add("validator-snapshot-"+string(validators[i].Address.Bytes()), by)
		}
	}
	return nil
}

// ReadValidatorStats reads the stats of a validator
func (bc *BlockChain) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	return rawdb.ReadValidatorStats(bc.db, addr)
}

// UpdateValidatorUptime writes the stats for the committee
func (bc *BlockChain) UpdateValidatorUptime(slots shard.SlotList, mask *bls.Mask) error {
	blsToAddress := make(map[shard.BlsPublicKey]common.Address)
	for _, slot := range slots {
		blsToAddress[slot.BlsPublicKey] = slot.EcdsaAddress
	}

	batch := bc.db.NewBatch()
	blsKeyBytes := shard.BlsPublicKey{}
	for i, blsPubKey := range mask.Publics {
		err := blsKeyBytes.FromLibBLSPublicKey(blsPubKey)
		if err != nil {
			return err
		}
		if addr, ok := blsToAddress[blsKeyBytes]; ok {
			// Retrieve the stats and add new counts
			stats, err := rawdb.ReadValidatorStats(bc.db, addr)
			if stats == nil {
				stats = &staking.ValidatorStats{big.NewInt(0), big.NewInt(0), big.NewInt(0), numeric.NewDec(0), numeric.NewDec(0)}
			}
			stats.NumBlocksToSign.Add(stats.NumBlocksToSign, big.NewInt(1))

			enabled, err := mask.IndexEnabled(i)
			if err != nil {
				return err
			}

			if enabled {
				stats.NumBlocksSigned.Add(stats.NumBlocksSigned, big.NewInt(1))
			}
			// TODO: record time being jailed.

			err = rawdb.WriteValidatorStats(batch, addr, stats)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("Bls public key not found in committee: %x", blsKeyBytes)
		}
	}
	if err := batch.Write(); err != nil {
		return err
	}
	// TODO: Update cache
	return nil
}

// UpdateValidatorVotingPower writes the voting power for the committees
func (bc *BlockChain) UpdateValidatorVotingPower(state *shard.State) error {
	if state == nil {
		return errors.New("[UpdateValidatorVotingPower] Nil shard state")
	}

	totalEffectiveStake := make(map[uint32]numeric.Dec)
	addrToEffectiveStakes := make(map[common.Address]map[uint32]numeric.Dec)

	for _, committee := range state.Shards {
		for _, slot := range committee.Slots {
			if slot.TotalStake != nil {
				if _, ok := addrToEffectiveStakes[slot.EcdsaAddress]; !ok {
					addrToEffectiveStakes[slot.EcdsaAddress] = map[uint32]numeric.Dec{}
				}
				if _, ok := addrToEffectiveStakes[slot.EcdsaAddress][committee.ShardID]; !ok {
					addrToEffectiveStakes[slot.EcdsaAddress][committee.ShardID] = numeric.NewDec(0)
				}
				addrToEffectiveStakes[slot.EcdsaAddress][committee.ShardID] = addrToEffectiveStakes[slot.EcdsaAddress][committee.ShardID].Add(*slot.TotalStake)

				if _, ok := totalEffectiveStake[committee.ShardID]; !ok {
					totalEffectiveStake[committee.ShardID] = numeric.NewDec(0)
				}
				totalEffectiveStake[committee.ShardID] = totalEffectiveStake[committee.ShardID].Add(*slot.TotalStake)
			}
		}
	}

	batch := bc.db.NewBatch()
	for addr, votingPowers := range addrToEffectiveStakes {
		addrTotalVotingPower := numeric.NewDec(0) // Total voting power is the average voting power across all shards
		addrTotalEffectiveStake := numeric.NewDec(0)
		for shardID, eStake := range votingPowers {
			addrTotalVotingPower = addrTotalVotingPower.Add(eStake.Quo(totalEffectiveStake[shardID]))
			addrTotalEffectiveStake = addrTotalEffectiveStake.Add(eStake)
		}
		// Retrieve the stats and update
		stats, err := rawdb.ReadValidatorStats(bc.db, addr)
		if stats == nil {
			stats = &staking.ValidatorStats{big.NewInt(0), big.NewInt(0), big.NewInt(0), numeric.NewDec(0), numeric.NewDec(0)}
		}

		stats.AvgVotingPower = addrTotalVotingPower.Quo(numeric.NewDec(int64(len(state.Shards))))
		stats.TotalEffectiveStake = addrTotalEffectiveStake

		err = rawdb.WriteValidatorStats(batch, addr, stats)
		if err != nil {
			return err
		}
	}
	if err := batch.Write(); err != nil {
		return err
	}
	// TODO: Update cache
	return nil
}

// deleteValidatorSnapshots deletes the snapshot staking information of given validator address
func (bc *BlockChain) deleteValidatorSnapshots(addrs []common.Address) error {
	batch := bc.db.NewBatch()
	for i := range addrs {
		rawdb.DeleteValidatorSnapshot(batch, addrs[i])
	}
	if err := batch.Write(); err != nil {
		return err
	}
	for i := range addrs {
		bc.validatorCache.Remove("validator-snapshot-" + string(addrs[i].Bytes()))
	}
	return nil
}

// UpdateValidatorSnapshots updates the content snapshot of all validators
// Note: this should only be called within the blockchain insertBlock process.
func (bc *BlockChain) UpdateValidatorSnapshots() error {
	allValidators, err := bc.ReadValidatorList()
	if err != nil {
		return err
	}

	// TODO: enable this once we allow validator to delete itself.
	//err = bc.deleteValidatorSnapshots(allValidators)
	//if err != nil {
	//	return err
	//}

	if err := bc.writeValidatorSnapshots(allValidators); err != nil {
		return err
	}
	return nil
}

// ReadValidatorList reads the addresses of current all validators
func (bc *BlockChain) ReadValidatorList() ([]common.Address, error) {
	if cached, ok := bc.validatorListCache.Get("validatorList"); ok {
		by := cached.([]byte)
		m := []common.Address{}
		if err := rlp.DecodeBytes(by, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return rawdb.ReadValidatorList(bc.db, false)
}

// WriteValidatorList writes the list of validator addresses to database
// Note: this should only be called within the blockchain insertBlock process.
func (bc *BlockChain) WriteValidatorList(addrs []common.Address) error {
	err := rawdb.WriteValidatorList(bc.db, addrs, false)
	if err != nil {
		return err
	}
	bytes, err := rlp.EncodeToBytes(addrs)
	if err == nil {
		bc.validatorListCache.Add("validatorList", bytes)
	}
	return nil
}

// ReadActiveValidatorList reads the addresses of active validators
func (bc *BlockChain) ReadActiveValidatorList() ([]common.Address, error) {
	if cached, ok := bc.validatorListCache.Get("activeValidatorList"); ok {
		by := cached.([]byte)
		m := []common.Address{}
		if err := rlp.DecodeBytes(by, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return rawdb.ReadValidatorList(bc.db, true)
}

// WriteActiveValidatorList writes the list of active validator addresses to database
// Note: this should only be called within the blockchain insertBlock process.
func (bc *BlockChain) WriteActiveValidatorList(addrs []common.Address) error {
	err := rawdb.WriteValidatorList(bc.db, addrs, true)
	if err != nil {
		return err
	}
	bytes, err := rlp.EncodeToBytes(addrs)
	if err == nil {
		bc.validatorListCache.Add("activeValidatorList", bytes)
	}
	return nil
}

// ReadDelegationsByDelegator reads the addresses of validators delegated by a delegator
func (bc *BlockChain) ReadDelegationsByDelegator(delegator common.Address) ([]staking.DelegationIndex, error) {
	if cached, ok := bc.validatorListByDelegatorCache.Get(string(delegator.Bytes())); ok {
		by := cached.([]byte)
		m := []staking.DelegationIndex{}
		if err := rlp.DecodeBytes(by, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return rawdb.ReadDelegationsByDelegator(bc.db, delegator)
}

// writeDelegationsByDelegator writes the list of validator addresses to database
func (bc *BlockChain) writeDelegationsByDelegator(delegator common.Address, indices []staking.DelegationIndex) error {
	err := rawdb.WriteDelegationsByDelegator(bc.db, delegator, indices)
	if err != nil {
		return err
	}
	bytes, err := rlp.EncodeToBytes(indices)
	if err == nil {
		bc.validatorListByDelegatorCache.Add(string(delegator.Bytes()), bytes)
	}
	return nil
}

// UpdateStakingMetaData updates the validator's and the delegator's meta data according to staking transaction
// Note: this should only be called within the blockchain insertBlock process.
func (bc *BlockChain) UpdateStakingMetaData(tx *staking.StakingTransaction, root common.Hash) error {
	// TODO: simply the logic here in staking/types/transaction.go
	payload, err := tx.RLPEncodeStakeMsg()
	if err != nil {
		return err
	}
	decodePayload, err := staking.RLPDecodeStakeMsg(payload, tx.StakingType())
	if err != nil {
		return err
	}
	switch tx.StakingType() {
	case staking.DirectiveCreateValidator:
		createValidator := decodePayload.(*staking.CreateValidator)
		// TODO: batch add validator list instead of one by one
		list, err := bc.ReadValidatorList()
		if err != nil {
			return err
		}
		if list == nil {
			list = []common.Address{}
		}
		beforeLen := len(list)
		list = utils.AppendIfMissing(list, createValidator.ValidatorAddress)

		if len(list) > beforeLen {
			if err = bc.WriteValidatorList(list); err != nil {
				return err
			}
		}

		// Update validator snapshot with the new validator
		validator, err := bc.ReadValidatorInformationAt(createValidator.ValidatorAddress, root)
		if err != nil {
			return err
		}
		err = rawdb.WriteValidatorSnapshot(bc.db, validator)
		if err != nil {
			return err
		}

		// Add self delegation into the index
		return bc.addDelegationIndex(createValidator.ValidatorAddress, createValidator.ValidatorAddress, root)
	case staking.DirectiveEditValidator:
	case staking.DirectiveDelegate:
		delegate := decodePayload.(*staking.Delegate)

		return bc.addDelegationIndex(delegate.DelegatorAddress, delegate.ValidatorAddress, root)
	case staking.DirectiveUndelegate:
	case staking.DirectiveCollectRewards:
	default:
	}
	return nil
}

// ReadBlockRewardAccumulator ..
func (bc *BlockChain) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	return rawdb.ReadBlockRewardAccumulator(bc.db, number)
}

// WriteBlockRewardAccumulator directly writes the BlockRewardAccumulator value
// Note: this should only be called once during staking launch.
func (bc *BlockChain) WriteBlockRewardAccumulator(reward *big.Int, number uint64) error {
	return rawdb.WriteBlockRewardAccumulator(bc.db, reward, number)
}

// Note this should read from the state of current block in concern (root == newBlock.root)
func (bc *BlockChain) addDelegationIndex(delegatorAddress, validatorAddress common.Address, root common.Hash) error {
	// Get existing delegations
	delegations, err := bc.ReadDelegationsByDelegator(delegatorAddress)
	if err != nil {
		return err
	}

	// If there is an existing delegation, just return
	validatorAddressBytes := validatorAddress.Bytes()
	for _, delegation := range delegations {
		if bytes.Compare(delegation.ValidatorAddress.Bytes(), validatorAddressBytes) == 0 {
			return nil
		}
	}

	// Found the delegation from state and add the delegation index
	// Note this should read from the state of current block in concern
	wrapper, err := bc.ReadValidatorInformationAt(validatorAddress, root)
	if err != nil {
		return err
	}
	for i := range wrapper.Delegations {
		if bytes.Compare(wrapper.Delegations[i].DelegatorAddress.Bytes(), delegatorAddress.Bytes()) == 0 {
			delegations = append(delegations, staking.DelegationIndex{
				validatorAddress,
				uint64(i),
			})
		}
	}
	return bc.writeDelegationsByDelegator(delegatorAddress, delegations)
}

// ValidatorCandidates returns the up to date validator candidates for next epoch
func (bc *BlockChain) ValidatorCandidates() []common.Address {
	list, err := bc.ReadValidatorList()
	if err != nil {
		return make([]common.Address, 0)
	}
	return list
}

// DelegatorsInformation returns up to date information of delegators of a given validator address
func (bc *BlockChain) DelegatorsInformation(addr common.Address) []*staking.Delegation {
	return make([]*staking.Delegation, 0)
}

// GetECDSAFromCoinbase retrieve corresponding ecdsa address from Coinbase Address
func (bc *BlockChain) GetECDSAFromCoinbase(header *block.Header) (common.Address, error) {
	// backward compatibility: before isStaking epoch, coinbase address is the ecdsa address
	isStaking := bc.Config().IsStaking(header.Epoch())
	if !isStaking {
		return header.Coinbase(), nil
	}

	shardState, err := bc.ReadShardState(header.Epoch())
	if err != nil {
		return common.Address{}, ctxerror.New("cannot read shard state",
			"epoch", header.Epoch(),
			"coinbaseAddr", header.Coinbase(),
		).WithCause(err)
	}

	committee := shardState.FindCommitteeByID(header.ShardID())
	if committee == nil {
		return common.Address{}, ctxerror.New("cannot find shard in the shard state",
			"blockNum", header.Number(),
			"shardID", header.ShardID(),
			"coinbaseAddr", header.Coinbase(),
		)
	}
	for _, member := range committee.Slots {
		// After staking the coinbase address will be the address of bls public key
		if utils.GetAddressFromBlsPubKeyBytes(member.BlsPublicKey[:]) == header.Coinbase() {
			return member.EcdsaAddress, nil
		}
	}
	return common.Address{}, ctxerror.New("cannot find corresponding ECDSA Address", "coinbaseAddr", header.Coinbase())
}
