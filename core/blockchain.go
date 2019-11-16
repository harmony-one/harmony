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
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/rand"
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
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
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
	shardCacheLimit                    = 2
	commitsCacheLimit                  = 10
	epochCacheLimit                    = 10
	randomnessCacheLimit               = 10
	stakingCacheLimit                  = 256
	validatorListCacheLimit            = 2
	validatorListByDelegatorCacheLimit = 256

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
	stakingCache                  *lru.Cache // Cache for staking validator
	validatorListCache            *lru.Cache // Cache of validator list
	validatorListByDelegatorCache *lru.Cache // Cache of validator list by delegator

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
	stakingCache, _ := lru.New(stakingCacheLimit)
	validatorListCache, _ := lru.New(validatorListCacheLimit)
	validatorListByDelegatorCache, _ := lru.New(validatorListByDelegatorCacheLimit)

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
		stakingCache:                  stakingCache,
		validatorListCache:            validatorListCache,
		validatorListByDelegatorCache: validatorListByDelegatorCache,
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
	receipts, cxReceipts, _, usedGas, err := bc.processor.Process(block, state, bc.vmConfig)
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
			bc.hc.SetCurrentHeader(bc.GetHeader(currentHeader.ParentHash(), currentHeader.Number().Uint64()-1))
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
func (bc *BlockChain) WriteBlockWithState(block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, state *state.DB) (status WriteStatus, err error) {
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

	epoch := block.Header().Epoch()
	if bc.chainConfig.IsCrossTx(block.Epoch()) {
		shardingConfig := shard.Schedule.InstanceForEpoch(epoch)
		shardNum := int(shardingConfig.NumShards())
		for i := 0; i < shardNum; i++ {
			if i == int(block.ShardID()) {
				continue
			}
			shardReceipts := GetToShardReceipts(cxReceipts, uint32(i))
			err := rawdb.WriteCXReceipts(batch, uint32(i), block.NumberU64(), block.Hash(), shardReceipts, false)
			if err != nil {
				utils.Logger().Debug().Err(err).Interface("shardReceipts", shardReceipts).Int("toShardID", i).Msg("WriteCXReceipts cannot write into database")
				return NonStatTy, err
			}
		}
		// Mark incomingReceipts in the block as spent
		bc.WriteCXReceiptsProofSpent(block.IncomingReceipts())
	}

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

	header := block.Header()
	if header.ShardStateHash() != (common.Hash{}) {
		epoch := new(big.Int).Add(header.Epoch(), common.Big1)
		err = bc.WriteShardStateBytes(batch, epoch, header.ShardState())
		if err != nil {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("cannot store shard state")
			return NonStatTy, err
		}
	}

	if len(header.CrossLinks()) > 0 {
		crossLinks := &types.CrossLinks{}
		err = rlp.DecodeBytes(header.CrossLinks(), crossLinks)
		if err != nil {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("[insertChain] cannot parse cross links")
			return NonStatTy, err
		}
		if !crossLinks.IsSorted() {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("[insertChain] cross links are not sorted")
			return NonStatTy, errors.New("proposed cross links are not sorted")
		}
		for _, crossLink := range *crossLinks {
			if err := bc.WriteCrossLinks(types.CrossLinks{crossLink}, false); err == nil {
				utils.Logger().Info().Uint64("blockNum", crossLink.BlockNum().Uint64()).Uint32("shardID", crossLink.ShardID()).Msg("[InsertChain] Cross Link Added to Beaconchain")
			}
			bc.DeleteCrossLinks(types.CrossLinks{crossLink}, true)
			bc.WriteShardLastCrossLink(crossLink.ShardID(), crossLink)
		}
	}

	if bc.chainConfig.IsStaking(block.Epoch()) {
		for _, tx := range block.StakingTransactions() {
			err = bc.UpdateStakingMetaData(tx)
			// keep offchain database consistency with onchain we need revert
			// but it should not happend unless local database corrupted
			if err != nil {
				utils.Logger().Debug().Msgf("oops, UpdateStakingMetaData failed, err: %+v", err)
				return NonStatTy, err
			}
		}
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
		receipts, cxReceipts, logs, usedGas, err := bc.processor.Process(block, state, bc.vmConfig)
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
		status, err := bc.WriteBlockWithState(block, receipts, cxReceipts, state)
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
Hash: 0x%x
%v

Error: %v
##############################
`, bc.chainConfig, block.Number(), block.Hash(), receiptString, err)
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
func (bc *BlockChain) ReadShardState(epoch *big.Int) (shard.State, error) {
	cacheKey := string(epoch.Bytes())
	if cached, ok := bc.shardStateCache.Get(cacheKey); ok {
		shardState := cached.(shard.State)
		return shardState, nil
	}
	shardState, err := rawdb.ReadShardState(bc.db, epoch)
	if err != nil {
		return nil, err
	}
	bc.shardStateCache.Add(cacheKey, shardState)
	return shardState, nil
}

// WriteShardState saves the given sharding state under the given epoch number.
func (bc *BlockChain) WriteShardState(
	epoch *big.Int, shardState shard.State,
) error {
	shardState = shardState.DeepCopy()
	err := rawdb.WriteShardState(bc.db, epoch, shardState)
	if err != nil {
		return err
	}
	cacheKey := string(epoch.Bytes())
	bc.shardStateCache.Add(cacheKey, shardState)
	return nil
}

// WriteShardStateBytes saves the given sharding state under the given epoch number.
func (bc *BlockChain) WriteShardStateBytes(db rawdb.DatabaseWriter,
	epoch *big.Int, shardState []byte,
) error {
	decodeShardState := shard.State{}
	if err := rlp.DecodeBytes(shardState, &decodeShardState); err != nil {
		return err
	}
	err := rawdb.WriteShardStateBytes(db, epoch, shardState)
	if err != nil {
		return err
	}
	cacheKey := string(epoch.Bytes())
	bc.shardStateCache.Add(cacheKey, decodeShardState)
	return nil
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

// GetShardState returns the shard state for the given epoch,
// creating one if needed.
func (bc *BlockChain) GetShardState(epoch *big.Int) (shard.State, error) {
	shardState, err := bc.ReadShardState(epoch)
	if err == nil { // TODO ek – distinguish ErrNotFound
		return shardState, err
	}

	if epoch.Cmp(big.NewInt(GenesisEpoch)) == 0 {
		shardState, err = committee.WithStakingEnabled.Compute(
			big.NewInt(GenesisEpoch), *bc.Config(), nil,
		)
	} else {
		prevEpoch := new(big.Int).Sub(epoch, common.Big1)
		shardState, err = committee.WithStakingEnabled.ReadFromDB(
			prevEpoch, bc,
		)
	}

	if err != nil {
		return nil, err
	}
	err = bc.WriteShardState(epoch, shardState)
	if err != nil {
		return nil, err
	}
	utils.Logger().Debug().Str("epoch", epoch.String()).Msg("saved new shard state")
	return shardState, nil
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
// temp=true is to write the just received cross link that's not committed into blockchain with consensus
func (bc *BlockChain) WriteCrossLinks(cls []types.CrossLink, temp bool) error {
	var err error
	for i := 0; i < len(cls); i++ {
		cl := cls[i]
		err = rawdb.WriteCrossLinkShardBlock(bc.db, cl.ShardID(), cl.BlockNum().Uint64(), cl.Serialize(), temp)
	}
	return err
}

// DeleteCrossLinks removes the hashes of crosslinks by shardID and blockNum combination key
// temp=true is to write the just received cross link that's not committed into blockchain with consensus
func (bc *BlockChain) DeleteCrossLinks(cls []types.CrossLink, temp bool) error {
	var err error
	for i := 0; i < len(cls); i++ {
		cl := cls[i]
		err = rawdb.DeleteCrossLinkShardBlock(bc.db, cl.ShardID(), cl.BlockNum().Uint64(), temp)
	}
	return err
}

// ReadCrossLink retrieves crosslink given shardID and blockNum.
// temp=true is to retrieve the just received cross link that's not committed into blockchain with consensus
func (bc *BlockChain) ReadCrossLink(shardID uint32, blockNum uint64, temp bool) (*types.CrossLink, error) {
	bytes, err := rawdb.ReadCrossLinkShardBlock(bc.db, shardID, blockNum, temp)
	if err != nil {
		return nil, err
	}
	crossLink, err := types.DeserializeCrossLink(bytes)

	return crossLink, err
}

// WriteShardLastCrossLink saves the last crosslink of a shard
func (bc *BlockChain) WriteShardLastCrossLink(shardID uint32, cl types.CrossLink) error {
	return rawdb.WriteShardLastCrossLink(bc.db, cl.ShardID(), cl.Serialize())
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
// temp=true is to retrieve the just received receipts that's not committed into blockchain with consensus
func (bc *BlockChain) ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash, temp bool) (types.CXReceipts, error) {
	cxs, err := rawdb.ReadCXReceipts(bc.db, shardID, blockNum, blockHash, temp)
	if err != nil || len(cxs) == 0 {
		return nil, err
	}
	return cxs, nil
}

// WriteCXReceipts saves the cross shard transaction receipts of a given shard
// temp=true is to store the just received receipts that's not committed into blockchain with consensus
func (bc *BlockChain) WriteCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash, receipts types.CXReceipts, temp bool) error {
	return rawdb.WriteCXReceipts(bc.db, shardID, blockNum, blockHash, receipts, temp)
}

// CXMerkleProof calculates the cross shard transaction merkle proof of a given destination shard
func (bc *BlockChain) CXMerkleProof(shardID uint32, block *types.Block) (*types.CXMerkleProof, error) {
	proof := &types.CXMerkleProof{BlockNum: block.Number(), BlockHash: block.Hash(), ShardID: block.ShardID(), CXReceiptHash: block.Header().OutgoingReceiptHash(), CXShardHashes: []common.Hash{}, ShardIDs: []uint32{}}
	cxs, err := rawdb.ReadCXReceipts(bc.db, shardID, block.NumberU64(), block.Hash(), false)

	if err != nil || cxs == nil {
		return nil, err
	}

	epoch := block.Header().Epoch()
	shardingConfig := shard.Schedule.InstanceForEpoch(epoch)
	shardNum := int(shardingConfig.NumShards())

	for i := 0; i < shardNum; i++ {
		receipts, err := bc.ReadCXReceipts(uint32(i), block.NumberU64(), block.Hash(), false)
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

// ReadStakingValidator reads staking information of given validatorWrapper
func (bc *BlockChain) ReadStakingValidator(addr common.Address) (*staking.ValidatorWrapper, error) {
	if cached, ok := bc.stakingCache.Get("staking-" + string(addr.Bytes())); ok {
		by := cached.([]byte)
		v := staking.ValidatorWrapper{}
		if err := rlp.DecodeBytes(by, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	return rawdb.ReadStakingValidator(bc.db, addr)
}

// WriteStakingValidator reads staking information of given validatorWrapper
func (bc *BlockChain) WriteStakingValidator(v *staking.ValidatorWrapper) error {
	err := rawdb.WriteStakingValidator(bc.db, v)
	if err != nil {
		return err
	}
	by, err := rlp.EncodeToBytes(v)
	if err != nil {
		return err
	}
	bc.stakingCache.Add("staking-"+string(v.Address.Bytes()), by)
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
	return rawdb.ReadValidatorList(bc.db)
}

// WriteValidatorList writes the list of validator addresses to database
func (bc *BlockChain) WriteValidatorList(addrs []common.Address) error {
	err := rawdb.WriteValidatorList(bc.db, addrs)
	if err != nil {
		return err
	}
	bytes, err := rlp.EncodeToBytes(addrs)
	if err == nil {
		bc.validatorListCache.Add("validatorList", bytes)
	}
	return nil
}

// ReadValidatorListByDelegator reads the addresses of validators delegated by a delegator
func (bc *BlockChain) ReadValidatorListByDelegator(delegator common.Address) ([]common.Address, error) {
	if cached, ok := bc.validatorListByDelegatorCache.Get(delegator.Bytes()); ok {
		by := cached.([]byte)
		m := []common.Address{}
		if err := rlp.DecodeBytes(by, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return rawdb.ReadValidatorListByDelegator(bc.db, delegator)
}

// WriteValidatorListByDelegator writes the list of validator addresses to database
func (bc *BlockChain) WriteValidatorListByDelegator(delegator common.Address, addrs []common.Address) error {
	err := rawdb.WriteValidatorListByDelegator(bc.db, delegator, addrs)
	if err != nil {
		return err
	}
	bytes, err := rlp.EncodeToBytes(addrs)
	if err == nil {
		bc.validatorListByDelegatorCache.Add(delegator.Bytes(), bytes)
	}
	return nil
}

// UpdateStakingMetaData updates the validator's and the delegator's meta data according to staking transaction
func (bc *BlockChain) UpdateStakingMetaData(tx *staking.StakingTransaction) error {
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
			err = bc.WriteValidatorList(list)
		}
		return err

	// following cases are placeholder for now
	case staking.DirectiveEditValidator:
	case staking.DirectiveDelegate:
		delegate := decodePayload.(*staking.Delegate)
		validators, err := bc.ReadValidatorListByDelegator(delegate.DelegatorAddress)
		if err != nil {
			return err
		}
		found := false
		for _, validator := range validators {
			if bytes.Compare(validator.Bytes(), delegate.ValidatorAddress.Bytes()) == 0 {
				found = true
				break
			}
		}
		if !found {
			validators = append(validators, delegate.ValidatorAddress)
		}
		err = bc.WriteValidatorListByDelegator(delegate.DelegatorAddress, validators)
		return err
	case staking.DirectiveUndelegate:
	case staking.DirectiveCollectRewards:
		// TODO: Check whether the delegation reward can be cleared after reward is collected
	default:
	}
	return nil
}

// CurrentValidatorAddresses returns the address of active validators for current epoch
func (bc *BlockChain) CurrentValidatorAddresses() []common.Address {
	list, err := bc.ReadValidatorList()
	if err != nil {
		return make([]common.Address, 0)
	}

	currentEpoch := bc.CurrentBlock().Epoch()

	filtered := []common.Address{}
	for _, addr := range list {
		val, err := bc.ValidatorInformation(addr)
		if err != nil {
			continue
		}
		epoch := shard.Schedule.CalcEpochNumber(val.CreationHeight.Uint64())
		if epoch.Cmp(currentEpoch) >= 0 {
			// wait for next epoch
			continue
		}
		filtered = append(filtered, addr)
	}
	return filtered
}

const (
	// Pick big number for interesting looking one addresses
	fixedRandomGen       = 98765654323123134
	fixedRandomGenStakeL = 40
	fixedRandomGenStakeH = 150
)

var (
	// By fixing the source, we have predicable sequence
	sequenceL        = rand.New(rand.NewSource(42))
	sequenceH        = rand.New(rand.NewSource(84))
	accountGenerator = rand.New(rand.NewSource(1337))
	blsKeyGen        = rand.New(rand.NewSource(4040))
	blsSlotsGen      = rand.New(rand.NewSource(8080))
)

type TempTest struct {
	Public  string `json:"public-key"`
	Private string `json:"private-key"`
}

var (
	tempBank map[common.Address]*staking.Validator = map[common.Address]*staking.Validator{}
	addrs    []common.Address

	NewKeys map[string]TempTest = map[string]TempTest{
		// Shard0
		"9012": TempTest{"e2c13c84c8f2396cf7180d5026e048e59ec770a2851003f1f2ab764a79c07463681ed7ddfe62bc4d440526a270891c86", "19e9adb6f6a10a160b951c33fee81e53beb7e41ba1b500626a1082215faba010"},
		"9010": TempTest{"1480fca328daaddd3487195c5500969ecccbb806b6bf464734e0e3ad18c64badfae8578d76e2e9281b6a3645d056960a", "8e7874184b1fe76d23b6a3e20b7178928123e7e4e63ae642235c5a955983c26c"},
		"9108": TempTest{"9d657b1854d6477dba8bca9606f6ac884df308558f2b3b545fc76a9ef02abc87d3518cf1134d21acf03036cea2820f02", "f2b677a9cc5b2325843d493dbc1a970d40c7edc032ff848a6116fcfe9a465015"},
		// Shard1
		"9011": TempTest{"72ccee6c5767a21d9b659a510482acca9b2303055d8efe09c2b55bd7d8910de4521036274c8a3a30564f0e9378b05189", "df8c569d2ed038f34408dcf980895de448bd25866192cc994aeb094982d7f769"},
		"9013": TempTest{"249976f984f30306f800ef42fb45272b391cfdd17f966e093a9f711e30f66f77ecda6c367bf79afc9fa31a1789e9ee8e", "ba742bf554447d0a6bb8da47f9134817c3f8a5c3717ed50e6c3fe6bc488fbc67"},
		"9107": TempTest{"ec143fa8ca3febd4b8b8d160b214a7135ab48f13d5f37ffce22607959e5da5b5852c8a8403ba0836fe64645fc1d92f97", "8ac153f6efc02cb4eb7c566a5fe89dd9bbe6da3df0c68b3e5dd5f4887b09725f"},
	}
)

var (
	accounts = [...][]string{
		{"14cab59f60837090b428a4fe6bca8f82c0bd995322feec4684c444d8f6e7c6c0a6c3f1705b0d285fe0257b266626a316", "1c733e5866e2f615e4d299e493f6593f4f282d10e35ca0362dfc4fba526be863"},
		{"d3c9c227f945c8a7dc0857884710a9904e29b9aeb8ffe1460e978568385cf3a1b6d2a73b47af0c2bb4851aff684ed383", "20667b58e47025b8b4a5524e5fd5fcd094f0126926098d8cba91cf8c57828237"},
		{"ea50cd0a7263d5530b633c5056d183388ac6fc5dfa6e7a8b59ed2c0ca9025d27d54d78e45a817bc3821a5d96d5ae7c97", "353a96f7484637fc5c806b14ed409c89c884f97aec14152be2dc02a95c02a256"},
		{"56b1e97e79fe37e19a5876c1f0e24e12e4252a086088221f73ad43174f477c02b2083e301216791092a8aa2633428883", "255f3197ee303e20a42223ea9dbe1cd2791c6c91d50ff7d2f406d480e133db22"},
		{"b166d48dec6934dbf6c446742de9adee53c7a6872f63e62611634382b163b7a1e0400996674db8c8d6910c4d8d935889", "083d9eb3915564ecaa4997ad4011e42a660287bf0175bd46cbfc5ec766b97d54"},
		{"9cf1e8c8cc7e32cdbdb762d464032a6fa5b74e7dbf916909ca22a57ff0c01856e350e2a7c2b1c74d8871d0c9af70640e", "a506f6895829956c8c514868f7283fcd6833e7cadc4c62e57365678f9417622a"},
		{"e43ac882bdac2bfd79f0645d8a97263ba6aede152400c946ff6670eee1da6154dade87eb43c5fc13316ec3cffbbc8b8f", "2d5ed7ebf50eb0a3586800d691d28b4ba4ff701afd3f1246dfa216b89cc9131a"},
		{"40b33fbbb15a002693bd5a4a7da86e45941f66c82c8c41cbaecadcf3d1956dd4a19208d69908663bcbc606a4c9f67002", "40c57ce993f53c298434a4be3e78dfc35c45c336671e2e8c0cfdb56eda36dc20"},
		{"2b6d32cd1a32f813bb98991002d42703cdb00be6d5f4f319204d2a539da58a14c5a9b3d8bb74a16c054c55339a53ae08", "b52394fc0fc094e71a1a64ef25897db58abc6cd1bb172672e4c0382d5b4a7826"},
		{"444ac7f6f16a4f023c18d7e7acf2cf8ebb96a2473e02a403f2252839b0309851df3abb1d24f7ae4819c7e9283e818897", "3043bbfddc4d950ef65f45e364c5e28b416abc5d5643dd14d59d19410f25f65d"},
		{"dcdc15f99ecd08d6e8e7a8746fc781d0113855ef6866c9d03f445f5b8bf4bde005148b28b75a3c7c3a0f37ac49e77986", "96aedec3f2b3d48c8464f04dcaf4dbf4c27b7108af836d0581d347e569d29344"},
		{"1c4b625ac8424d7e45df5aca7179113af2edeea36457418ae84a1294e54f4b740309fc1b4e4b981b6f51ac0a68dd5791", "15a1b5aa27fe98682db715ad177e8e3c661ad917405a135b18acdfb73a71881e"},
		{"7e72cbeb6152d45903cb476cef4c2e65eb6a5e0fa8d90d46e0bba4ff16853969c73bbaee66d0c3f47a556c502f87a319", "c4cdb2f30ccbad6200fee58cc622c8849139484599cb43b65e52aac55d888a4e"},
		{"1fdf24b10e748798be733d4be7d8c5d18fde8800e3828cf6e7ec66beb87c55aae345bfb5b6321ff059089dcaef67e696", "be1e345b4f85583750c6ed3a912d613d9a47ae06e79492d8a7a63362ed02dd06"},
		{"c29b37816e1ddf0b7736af7ed22f4ddcab26dbb4bdd7a5347474aeffb9d933e81a14b66e98018c4dc125b2b0c1c1f605", "65e2a6aad957d23917ef958e9465b10bf0271122820682bd9360225e0b17c667"},
		{"7990b4f5975e399398b82bdf87eba6852dfff49ce25bcc88a360861a660c1fcee3a1f994a4879f482f467fc53aa54e13", "12deb9915b4b28ea7b822467030910878bfcc1cc27d3f0147dd0cd9213b77718"},
		{"27877ce5cbed77848fa7f894c2b4a7f8fd11d0e31e0c49f88682ef063c44da291a728797e6e5852d0bcec659bf05dd85", "7a84694b1084c2f79d0f61cb46e7ff724a31f3774194e535eedc51df3e89e214"},
		{"73a0feab8fc245419d8459e3d50e5133a835a69050c417e501f826873149c645cf1290619a054d1b4485b9175c69a092", "27ff614de69f7da721c0ec240af78c928ee60418940869b9dbeba6e939f9df70"},
		{"dca05a3e1d080b69496beb131b2c18dfbccf9c5fb470f16bb4a39b5c3aff19fb6aa37b115009ca7724038059dbfa2d02", "ae84b73e496fcf5f5661773ba92125b3d19f53f9539e0ef5f8cabd7c5a739321"},
		{"0a9221c81cfb6340e4e40a46403970d6f929c090bc4169540bbcbe02fb0ad6b06941dcb192677b9e2204a5ef27a24487", "a18c0fbea616989fc31e919c1f8090dc42cf02f410808ab1573d0b1618f3ec00"},
		{"e03d395369fee3c9bc7261bbb17bfb5eaf09e0b88978cb70a6c18f9860c7b68f231db64dd69d04035683762866febe00", "e7ac0c215d3fc055426dfdb4a05b4c48d41c7e850d6c5d7ecb24a6a5061e750c"},
		{"6a58a3ba54421cfd0bf2c8b15b92d1f9015ffc4d6e76852c5609b092a14a5ed2fa49ba1183414640e3c2c43e41b55286", "7d09db6b6fd3f9fa03ef6f788f59c12292250950ebe37e0def2db46c1af8325f"},
		{"adda915c4ca55d78c7dc595bc26452a3cd1a8cc117fbd30bfdd35ab234519566b4ad0aa581ae6ac64812990eabdd1204", "4fd18f56ad25de60b60caedc2c3b14eb8b2cc30e2a258b3f1348fa337b25ae59"},
		{"af1ba6ca3e3ba7436e3054d4b5cb3e19b7b1e0403bfbb76a4e25a1ed0fb75258d9fdf53f609394e002e865bfe525988b", "9cbaa24d59024b19a14f399a8183f4ab8a2558714eb01c3a8ff35890e6750908"},
		{"dcbfe47fdfee29df070c078609be69c412e44b5f7dd10b1ef58ed368b7a8c422f1c3b70debed3441c0cf15ad7d03df05", "82d910e82dd49e79f13829ed74f8562760df0956a7ae7e60ed1cfec9ef2f9727"},
		{"a61d2dfea77bd324d2188181487a99e8afa12b09ebe41e5bd031f24a07eef785fb0ef80f3425ed44ab1023a5a41fac81", "015e5b8a897f4bad686f29b8c1fcc81c1654dff616fd21b1664194369d0c5e62"},
		{"a82bdbd2c017fe51aee694151138a7a1b9df848c7998ad7a9fb1df2003f0d52cc99099d5fc12a3211f91d411c3553b11", "a49de622d95e586b1bf55b53d18364ff46bab263e73e2b03f00fe5d69923d248"},
		{"390851593559be7658ac6c342422f4a447a8291dc517cb1809e150e6b5fd538ab60e1b9bdbe6941ff78fde71ca68f989", "2f5ba8ef8e25999004a5c319cc5a5a4c62ac5c6a0bfc8d25419bfeac91c07f73"},
		{"c04d3fd26431a4d0ebe1ed385b46ad975f6a663365b891dd1b25a731cb7b4295f88b920451337152480571006eafa180", "e8538488afc01b044e01f6eaad582949c792160906fcb7d6047d174ecf2e7b39"},
		{"02e8a378a863080f06f4714161a4b592550e6889e15f296bdb33a156675a37af13a03522973e7329ba0bf8c79dfb4d06", "8b57d172579dbe19c21dfcf00d47ac5eced33a531c298df13038350c5aea391b"},
		{"a39b6cff47d06e9857c94da5229f814ebeaf041a704dcbff8d64224b5fa996e923e015e1c36b9e48031f48948d816b86", "c6bb4e835e4c8949117606d6bdb9610a0ebaf7eafe66ff93e97e8f6d054b8a72"},
		{"226c9493a85f82fd2efd3d933cf0a90436e0d16ad86d06619f01b75d2b89371e51b9c8b9417e3737538f09382a657717", "ae0e6194d4b86bbbfd6a7d557f0b520a37c1d58350801c6b2fe691c8e8b7a617"},
		{"8b0d5f40e3d655f9b8e6e4cef1c005c02ccbee539d7a35e5b148233e2cba8d050a838e69a156c29e416746961513f819", "8539c03e72eb85ff3321593fbb7f8a86806f93756c52b611ccdc0fdc3849ac11"},
		{"72506fb7d6467a9be5dc91051d3639e8eeebf2cf9ef81b3d8a5e95a47c35c2636c74f21ac22ae8bb9e9f43cda4edbf02", "563cce730dab68d1d9a57d8ad092d48b9589b4fb0926a6e2a3d92b110cfc9b6e"},
		{"27614c62a76a38b79dddee0fd9142b1d18b1b2502a411b86a2e43eba3a01b6a87f11192dbc02492647173c77c2cd290d", "e31ad7aed3c46221e36f2bedf1ba30ef4c891d0dfb2ca271a695175224fe2907"},
		{"bd59cbb9514ea7e9a6d8b7d6b5d05497f66aae8d32b3b97657ffe13748963dccdc4d3ff48002dd2b6e05b59e7cd6090c", "65f1a41835f471005c8ab1a65b513372c4a40a68d4c8300313f1368a28d14064"},
		{"5f49373eba2af47b6f0ec9f9ab925b0e4d1769dddc656bdf2adc992eca654495222a1b6e89fd97e86c52b039bb7c3212", "7845b0f43a621204bfbe7d22ca91fdf9abf7eb6ba355bca2f1e650b33d69b20a"},
		{"511329bd1ccce0bb0d86751fb9dbfe7c7e3dd1176112f182145b1123a850e75b13df43c199153de8078b3d2ebbadb086", "0f800ac46d8ce7131f09532ee239ce14a8356375cd374c2d48081a02a3a1de61"},
		{"37df36cc1a203a38264442093e424ff7b56a854ec9ff07e12e316f0f5b0f0ae82262eef41418f74593daafbb3e12780b", "f6d241140bef139880c66042c2a77cd94d5183f982c83359849ded6de196a319"},
		{"6d66cc4174873e1df4c34f56c3d1d01281a2bfd25fbfbd45af64d4aaa5fcbcd889aabeeee6314a0061986074f9937b88", "d9da208d426380be14beee42db5e47f1950927af70bb1f7d30488ce829d28436"},
		{"e965403c1eecd5ed151e844b2d9bf601fb8d8d1fc86169e60346dc9bdfd4c5317312c02b9caada3c9dcecef61b21ba96", "8ff45b01787acd4b2d4a780c1bf3bea1495e6cba6328371028155d4c0fc0d773"},
		{"7bd979db856a697dce7da76feab7f6e3c036984ea766198c695dff95532f8599d4cf6e3c00e2455bfb4b5ce0b26b5192", "c83952ee8ac4b25452a3fcfc75ba07ce58c392878b7c519922fa4200d1169e40"},
		{"4c40089a3eddbef0886fd375302c5dcd7b56f83cf9ff48afa56389d9d7b8b0d6daa0f2e2241773fa7ab7f0cc77621917", "7cd8e8fac117a7e327a9d609f176dfd0445749ec823096ff3f29930047959303"},
		{"2694baa0e0bd4e24c192ae5bc4b3a096b2adb0026823298c4c96bacf062b591a1a3b557d6c81773a0bcd0e1031755b8e", "81ae0d5dd80d47acb9761f3734533fa9543a19ae1a1fd67b10555775f1dcea37"},
		{"68bc09c6277fe5a478a4b036e9ede1934b38e7ba86315360b7c75c1999b848df0bac7fe00f9d1e43ca1fa20c0a200c07", "8a4172905299d69fb005ea2650b42604d8900ce9967304648545e8c0267f6b0f"},
		{"77c4d9e0ec0f1cf6c981481f766b1395ba0912a8020a5ef1e15949ffa95c6844fba6a87cb7dc3a01774ab7e26360cb96", "09a98961b4ca28357fefb75ab183074e8e439a9109d4c8a12e06f1678fcf9c1a"},
		{"7fa504b0d97596010748564f736385fda141c12f1dee6bd1f0c3bf1793f5089a1c78db84ac0ca23dc48b764d356ea188", "26ee79bf3c2391bd9af142058f773b93c9d9b8659cc2899452ba9f3eb017151c"},
		{"ddf52e6c80dc39fa940e85265ef09ade1a44e3c83eb5bc2798555ca3ecd5ec8438db21cbfe79be0f4ccdbffb22699d08", "2350f67df6146af7aef3f36de4a815574aa33db097a22184ef11911014546a4e"},
		{"b0e476eb3d88e44fe6d654b78302009940f5f7ec80e0c236802554f223c8c4011a6732930e7425973210a6d669b92696", "6898e732291fc8fad1eeb76e42be8ac8c15abf6f028bc965bef92acfae389d3a"},
		{"ae7eff87ebc4c3ea47474367a10880139cde747de0be5278c7935f54e245c762294e1fa0094ede65e6f716a0c7ee7680", "d68df81e8b89abf32597524d52240c17d0be7e3a02bc34a2ed3d633307e72f17"},
		{"30b89a0199fb280398f0d65b200b4b45327368a45337f679993ec3e7d38969d08ba3e1387e1c85ab2c4b43ce674bc105", "8340272fa1b8c273102ffed66a3934d8320772dfd5fdffdbfb969fee97364464"},
		{"9542e021b19160efc08a5aca567f7d4cba277e25927b756f3e0ad535b74caa8b3d62216c90304625f3c4293827c3c38c", "01c590a4f2bd80c5411d744b2223f5851d2e1c46731dc3a730902ad017e67c5a"},
		{"7ed1287d91d29e1fbdf43360a494f53d450fab14e3b9cc08a63f06c868d757c32e781622603a37057c909d57f1901088", "f56915dc69dea77304cab132e8bc247e568a0b2b11672a39b84c38c9dcc49468"},
		{"514051f789312e3a1851ad56ae4f64bf798c1147d99065903f5aa5807e938ce6cafa0b4122b0e3b6636f425241b2ce09", "1bb85c34055d3a9e12482136a65a837078119927296ebb5ae6a7ddd0c0aba514"},
		{"f8727e55841bd8ae53db38e67377b58e882072e937f54cdcf70c4c7f151fe7b175110d0962b66167de26c6074b6a8c17", "58847f00751f7d5bf9594957e47a88c773b16257627cd052b7180e1c0721e950"},
		{"b4225d7cbe4a290cf880dce13d65155b8ddb2d27ee495929338780d268ad96747f4013e7455fcd589ceadeecfc138a03", "3e305f7d0941507b251469d4628294582bd9ac3c995067606321422492e3442b"},
		{"82ec676e743abf1265d81b1bab873acc911d533cf9aec5043dbbaa7b29ee1163180cc84bb22d1a567c532e3d676cc281", "da0a4974718fd08ae9cd46a6d0217a67c3fb2758e81a2419d13bb2be56eb134e"},
		{"cce64ff99850372b795fd146688c9c8f54d6aab555cf624623e665ebaffc44b3587d7759a95968b4cc874380be096993", "440bbf61908f54e4801fcf215e62256b1d2bdac6a2c0cd940a20beed04cbb844"},
		{"0b10167f5ca2979864e46125df72aca33956173aee020ca88dec3e65a5e0d41c1f317ba930d66ef9075c9ab20595f58e", "bdd7446112ddf42ce0d468a01a0c1c08e45b6b76239336878922235216002210"},
		{"502ca2a064d45970776140c190ae8ff0472f8c7c3236f43d0d706e72a430ffff4050bb8214de2fcbec7e9c3b1577f688", "184d37b6d7f04f232e5f8262b28a4aa26dc68f2208810d02665383cbfa074835"},
		{"02e95502143d16ab34f082b4368113f49a6b7a4dbe33e10c172a1376add5405a934c2992a4e0755636e3faecc11abd88", "7dbaa1c7760334a0c6b7148fe91d7bde88d212317b45115643e29d0cb8484155"},
		{"9d0892d0e8e2474fcbe225b69b9560571842cbcd8a35f0ecd0d3cb9885a079dfb2d1669f886062f28e2ea347fb7aa014", "b5a7ed451787128bee2b256be308e5b0f73ec1f3504f0400fb801c4ae373c327"},
		{"c4349aa33c18eeda0be31e0a519c5a53cfc6940b995c88b5b34b81bc22310f3eb68a7d6abce6d7e609a6a269a75eeb8d", "5c142f86bbe3fd2f6e446060682587fec7dcef6f360fb1659bd380ecfba9c656"},
		{"ba97d3b920e7590afd420458e4dfbce3328a7890dc150439bc9f9b343d4267851eddad645940a14c14f9c9dd085bab85", "966ca693d191cd6dfe003862122adee65c96d1aeca2ec85aff716c3ffef4ad35"},
		{"6caaa02b4289dff76ddc7f8441bd83821847a7d1d2739303dca02b8510a3cb6037cb2562c2ad5a4528b555ff3128a618", "96bc5bcf9421b96e63c150d2ebdb9a75a85ff3854867462efe0178a7c4855623"},
		{"1467f6aa800e4476846e721d6cd684cd899aa46d87c5b6ba674f9f9565c2fb64cbfec5b4af7b55e86fa2d8604acdb884", "1ef689c2fdcb0e93ed3830311f146f37dc1df26b7027760292a3818b4964671a"},
		{"176f9edf6b38ce85d35f2d314287c03f2939a88230c2a493476a337133f9f2c00d1a69e0d24587b89597c400236d2f10", "c06afc1859c5148fc2c3820614739cc5132d50505fa7dc40e8a914e4b65c9847"},
		{"4a6a00e30b327b8d9a01ebce8b5c765fa1a5ebdd3d7a06207590e5a293ab8b38a2a197e386e678cac86826953f2c5301", "1fda30d5de8a0e8b9d725af8805996f7e8a31f36406dee7e1d1e046265c4de04"},
		{"5c33b153ee8c281b484c20f0a8d9626a1b13aa20d386347250996032df2cc607a307055ac46796de7e65b50e5149648e", "cc1a270c065654d949c5e9d2e158acdd3750655cb94ec94810041dd7a26c9e38"},
		{"0633521bb2c3c27477e97ed506587ab4994072e97f307b87a735a0b77fb48de51975114bd24ea4938063d4edcd1da313", "f555d422832a7f8f8309f61b94cc4dd96d77041684dd08e72396bca00c52b439"},
		{"e4ad16fd499040c0315c91eb645c0fba1b1f922772d39bd894c055735c9b9d483473d8b6321638e007bfcfcc1a67988f", "ba2d5d76f3178b954ad624c6af84467d4b55e43bfdd1add98dc50e9857514c02"},
		{"ea8353454271fc95a17543728255a4c3135ec6a48b82458ff18568b155d30f3449c480882bc77d38cd553195fde5dc12", "964fac9a31467514411d96a44cacf2a5a6f0a2db985a12dcfb87adade0cecb4c"},
		{"e298500b9c972c42cdac74a11a096ba2d64405d015fd5626e07561a297dc6c4f36fb5f1cb1c4d86894aec82d5e2ef691", "08bb76af47e9385c354655e9001ada326e811ae241c4d22f81a2111ad9baba44"},
		{"04e34592e6f062a9558fc09ca379585e15b4ed6131e2766354cce6796233db689c340ed6ee0b7e9f40e85ef7b50e6a13", "efc1f64c2d8cda2d1107c0046703ddf4cf7ecfc6f38adc4631736e83846a145d"},
		{"8c4b304f3a0c7310de817b6e958588cabb4124d0cdea74eecb8113a2daea0dad1ae203bd0d9ce228e2ee89d68965868a", "6e8f06bf1128b4a148b47ba8f2678398add67e0a8b11f81aa7adef4bd7d0c22d"},
		{"02f3d61f931c09d5954878afc08799e1b8de77b1756576e3a11ef836247bdc212bad1fd287712a815228c2b13f5b128f", "468049f68a279e8be38b0d3f0c67758e45cc5bd4b5a236fc7ef3eba78b45176a"},
		{"323245a1027c2a00b9fbb908840601a9bc2fd5808cc313a93b2f14714ebfd234dabe8984bc2169b09ad5327e2a3f6584", "5a65a9fa599fd7ef2140b7b80fd78842dcdbb4084f6e069ef427fee4cb2ca240"},
		{"62a08494a9808ba4fb15431dfb431c8d993afe6a4e2d18214b8ea8ebedcbd46594894d4afc0290739fb17d4f7b71c389", "1d73c784cbd13c5e7987c78540aa21aabc395fd2cbb50a6561677945c2558d39"},
		{"fd46f1477459c863f903ac7daba941e2c91031dc000740ca8d185a86939907bc9893ffe586dacdeabb1a3459c6e8a696", "04ca4066f6f3a49e8ff5ab3680a90bb2e17da973c4fb67022a6ceb3b05b1a152"},
		{"e71d1a01831e052622264ad4d7e7a9123ede454257501c7c60d214904d124dafb023dba7d48dfcc49d50fca5ef2c5098", "de295122f5c2982ad00cda3cf81eba0fb05c087969b01862d7e75569c931b856"},
		{"ea16cc1fd08e907911f247e6837ccbb8ebefbf8986fdbf2cb1e3c29254cf030ea211a279d8e368b61a48e49d1833db17", "1b3a20dfc6f2a25b1da99b2c836b1e8707d897a16313790ad0618f3c952dee10"},
		{"dc90ec2c5c2e82793bcc45aa9ab72fe40b68d63e394e2f8b3b654f1b8bc87cf5e94a2fef0b955acac1018ef526858f84", "0973be2092613e69d8d16d251b74e6ebbb39932f9fecad384cc96f917438b811"},
		{"cb7ce449200b1bbcec236273d2fd79bd6f633cfb21ea4720571d36a4ac0dfbe6d218f74d1093a7888652c2f82ab6820b", "568363bf1acd8ab07feaaf324401eb418883b936c720088a23a1867233083b35"},
		{"277544d9b04150060bf5b53a9e53e64216613e2e5356a2113c7e4706f16a0babe6353a585444495aa175cc157aa40087", "ea0a26d7406f3ecaeb602a129cc6052db891fdec7a11e6ee852e8dbf3520a91d"},
		{"92eeb95ca74d9f4e9deb0afb2f7a1f9973e2edfcd62c0c3d13c66b61bd3cef962198b5b5a2451934ea079ed1182d3591", "5a5fe61ae2e047bc0e8c81d5c5949749f4c419ad921a858e07524f2486f0a12b"},
		{"e307e3b4bc5da252d020a755aba61f3adf35b3bb56b3ec8e0ded021674b620fa14d6071ccbb19f04820955474458bb19", "d44f0e3a91f72df286ae784ea61ec12b7340fafe647ec25b8155d630b55a3b12"},
		{"f59598f47bfde137d5023f865861832949606c6256c235c697f01cfdc41583161ee6674ee89a1766c0a75e623be22509", "f310ef9048ed84f7474802b4ce8778582d1d91c6cb6e03af9741ed3c5759af3e"},
		{"249976f984f30306f800ef42fb45272b391cfdd17f966e093a9f711e30f66f77ecda6c367bf79afc9fa31a1789e9ee8e", "ba742bf554447d0a6bb8da47f9134817c3f8a5c3717ed50e6c3fe6bc488fbc67"},
		{"72ccee6c5767a21d9b659a510482acca9b2303055d8efe09c2b55bd7d8910de4521036274c8a3a30564f0e9378b05189", "df8c569d2ed038f34408dcf980895de448bd25866192cc994aeb094982d7f769"},
		{"08a71ab590ae132ca2e1a7211e0f90498629b55a0a1e2b753b43c5f1cae8cefb4f2494e4fed603f2b725cd893cbb0c90", "14fac9e0979b69f4c9ba3de4eaefa4e26615dfb67430d459d3eda58567849e57"},
		{"a59ffa292bb9162f55a83164c6162dd9b485b156d18c9d701649299da38aa5c3b2713ab9cac64132b98a3bece2061897", "8777f023e07075346c3ea3b2fe95f765284d952b3c04548085519fd9ff09dc61"},
		{"e2c13c84c8f2396cf7180d5026e048e59ec770a2851003f1f2ab764a79c07463681ed7ddfe62bc4d440526a270891c86", "19e9adb6f6a10a160b951c33fee81e53beb7e41ba1b500626a1082215faba010"},
		{"ec143fa8ca3febd4b8b8d160b214a7135ab48f13d5f37ffce22607959e5da5b5852c8a8403ba0836fe64645fc1d92f97", "8ac153f6efc02cb4eb7c566a5fe89dd9bbe6da3df0c68b3e5dd5f4887b09725f"},
		{"dcee54914821c267328842dfa7aa8fc993488505818384169562f1fde35067b3258600e9bec801ffa91b2d4d44b04580", "3c4816103eab7b9e4f279dac808ef79a1754a526682503c9dade0c1b9413a316"},
		{"cc120db9a2d189e58128b6c2c546be5a68c173de71b7f5062c184b3909faeed2699503aea5f2685a1b150685244cfe00", "a409124667a202e4ec48ae124dd1551e2f138090ad5aad51407cbcd5c2a56b5d"},
		{"5005ee176e5040d0cbd2ed7464ea826690e9aa8c846adee42eb438db86be34cac177f50ae66db56f1f2759e06cd4ca86", "c66c665432c441e90bd14746c8b8dcfb0284e16f0214fb4b48da97dd23ba9f0e"},
		{"9d657b1854d6477dba8bca9606f6ac884df308558f2b3b545fc76a9ef02abc87d3518cf1134d21acf03036cea2820f02", "f2b677a9cc5b2325843d493dbc1a970d40c7edc032ff848a6116fcfe9a465015"},
		{"1480fca328daaddd3487195c5500969ecccbb806b6bf464734e0e3ad18c64badfae8578d76e2e9281b6a3645d056960a", "8e7874184b1fe76d23b6a3e20b7178928123e7e4e63ae642235c5a955983c26c"},
		{"5139d2fbd1af771f76ecb93444d15a1a0bdb9706ef8149db9f88d9456e2b616a812f2bc21a320185c4619248950c9582", "3a0032951ccdcca696f82f16b16178f7d239db5444b6203292573ed1109e6466"},
		{"3e15ed2b396c2076d2df067e58dec3c5fc0d5f31ff3b20cd245bfcaa58d73db8373da74ce7b852d95863c1954f1f4986", "8adbbb081aa9885854b8ce9b6d437e30136d065f2835f11ea6a0b89b74509372"},
		{"e99f29a4791a9da645539d154acacb731f15906201e194a77aa7cf2cd4827692c28e7f3cb8d57a3dd0157d79c01e0492", "a68648839687f1bfab70ae1b8d41eab44cfdb00b599f4dfb5aae31cb77c1a60e"},
		{"744d03bb36aec3680fd894d3761d30bd8f9bddabfcc0465e80bb288a43c02801073eaf441a4f99f10776788dc6bcdd8f", "be1fec362830fd7457db13e45cc5e24bf9aa15c54d347385b112bfe18044ff2a"},
		{"a3b437fee6d9c0455b34b70c984e21ee8f1d6c17fb857462c67592b1994c72d72de61afbdc9fdfe5a90d598444b10516", "319f70b9d21899f6a9db38a9262e59a37212004ee8678f143d71bfb1b8bb5b54"},
		{"02dbc59d3d2a173f2799ec5d9010238c762311beb1cf63df03937e416c9c67e491ba5fb912c2e46488c022c009257806", "2c39b8517d58e6f11d71459ad5f074cace01335bffc572741da5845e357b2d26"},
		{"f26d9fe117f382c14128352de40ea9b662168f50a89cc10ccb20a3680c1ebfa5fe1efe84a7b1090e316ab86dbd734094", "f79aea85a3673fc026e1314139cbf9c12fbe5a688623465c966c175b69472f65"},
		{"f3b034ab8643ac72cd29f3151d124348cf459aa7ad69dc0f98782120f69ebc82a21df0bd1db01ff5f9fe304f4c7b0485", "e6fb858d69eff707637c4c1a2c14a9bbc8c7f05ea58b208b119214c236c1512b"},
		{"0a7559029ae550507facd6375a30f70cbc108407bb9c4fab8381fe5b8a30c0a2d99800b21a788b2d2f0e74f3c23c8b12", "25af65b82f969650b0c50d9916fd10798dda73f34382181e238106ebb9649652"},
		{"c4b9926ffc51e45404cecc06dfe0d652c9510651470fbe2b948d6730373d472b3a9a87e748f80785f22544afbb16ec97", "8a2e1d44667d5082350de9b05536de4b3de88becc68b99d4cd922fa006d6481c"},
		{"4a10b2749f91f9d8577f612b374274f8bd90e50d1294ec1bb043e4d0de590ae8adc0e48a7c494cf6559d9ee5bfb2ec82", "f2159355c79d46d5abd239a780d584846bb7f3622306e3dc6aef5d7646f8b037"},
		{"cf03b1d8da5a8b063e278215d8e6677e9cee9a09cd0ededb32e28e623cd43ac7fa72e571e786ac925253e56941170099", "4a4939ded1d7210f01decf8905967a8814d94c7e6901923e49c0c862bda1cb0d"},
		{"eb28831cc5257b4e85f824e4e1c71bd530eb60112f21a8284dc6272e11abc711041b64afebf202086b18917d765a2301", "09cf22643028d316bec1ecedddb2881064132939a7b63f50fc94509b6c5f5433"},
		{"5220fa5b230d78234a25adfefff480a3664405f231eb87c797b33fb9338e5bdcad58e3b80780bf7cfc0705a51fd5d519", "7e985ff6ad71d8968bd7dede437467a9bb120d67cbe26866b81e9df6153e9e73"},
		{"7402dc3baa41314482037cad9ac5487d0af9b8775b82b0cd4265af078a8b6af48358b884c3047c0c46f4bacb658c5b02", "12044340eb632a6ba4e9dc81f9bbcf33fa68792c45773ba51e4d62642c5ff761"},
		{"43f08c35d400b36299cce44c74064da8ad15c2f4896dfa9c5810b09197e8f26e5d0ab11db797f2dc72f3fdd8b8fe4b8b", "00165cab4cea976f26edc80afc9e5a4b15678b20cfbe3d7f38a630fb1824c44a"},
		{"4ad485a064eebbc26fc4c016b8ec979aa6816d1b847fe52111f677e9729f8793576946d67b9aa01265f98d4e437de988", "82e9fed6646457d008201d7cf489e0e308ab54235d1ef59a5ae484ad6e93ec34"},
		{"733cab5ae15056a40cf515acac9a5e283f0dc56e5f7b7a1aed4e258d4ca0477a9804f2ddd59a6c2eef0697538fb8c491", "015db82e40041eeffc1573d93427be038b26d7600cd0651771daf3a97f747701"},
		{"2abd73dd3f286c23e7bc0650a580b6bd29ce3a948067314629898d683b9e4c1d30924c01ff0813f7e912e7e69f07b28e", "c32154e8d23412450a33f46bc61716199d1316a3c3ae00ed7e374b0bc7fb3d0d"},
		{"5ec4b7f1636b7d565addb55c6cf182b044560296dc3c82c846b3faae8e485fef77ce9d8b0e6a04f832454fedbad46a00", "63cae9af8c9adcd402aecce77cf0de3912e4c6e1574cdbf2ed3046c1605f4b30"},
		{"160004f16449ca70b454f6260d8fccfd6b399e4a099ee05c745cea26ba48047373f105bbd426a7035fb582f8e6348291", "09bcf84d83fac97648715d60b7b70eee4e3abc03c0253bc5147913b9ed405717"},
		{"23b33c8771e70d1a84dd2f24b924c50a7c518a6e6f5a073f4c223ef299876689c4164a8f29fc21a531895750166e0b85", "c7c917686f6829fc79cb5020ca7271bee4cd2c3fe8c7b31ddca5e8c1c11e7b1e"},
		{"563a4f6d39adc91bd9033573a517e5a365b181b8b5b6f086f67bce1e09f80489a6b173f0ee02201e5d333dab3b1f9e0c", "452971a484dd203669f330cc5cc7a96febddb72ff8884d63a6ff225728c0ca46"},
		{"d6be59700f6d81ea11cc22481d70b972741d52f9495f1637e7e2a478ed99898a4cbe5a5fbd6fd2648a10ad09796a5811", "52a6e2b5c5da26e75b96894f1a443d948f6235873040a511b4a33202f36cde1e"},
		{"8fce243f7d8f4bb28195c428bc54c4a233d9ac7c204e75cb45771f1d4ba9725b3b9c69c24e390dd932ad930ef4741997", "1b3aab5c04b451cd714d55273f3749aaad4a45247911292ae84f02ccb6a30d1c"},
		{"56318730454846ca6d9d7b23f2de5b3a67b4b62fa057e4556c144d6f543e2e9defc031ba3057be59b5532bb0b2f81c06", "5f434becf1c975090deeda7bf15c5705c03b767db5bd338f400368baffba9825"},
		{"5f64a969b0ca0e06b78c8d6f0f655116dc526efa48115fc33320abfcf73dc1136a5833362dc3f9147a574fc0e5cb2f0c", "98a256ff411ebaacba2d0d1519a1f952b2e143dfe323202fed5046ccb24b2224"},
		{"4ba427d9624cca3f21a76e31793debe6c7adaa871693ee3d88e8e943c4934f941896ca7e1cdac50901eea77998504f99", "39b81fcf87f18e479a6abbe2a7011fe5992fae0beca763782d911cf5dc8e5b0c"},
		{"3a24c5b01460b83f48bc10eb56acb75252d800f3a57d42a04311d195dd63a0b768979c4a7291802d09b5339c3de84a88", "832df5a953bb748109d3346c2d9f1cd2268ae5026cb7d3251cf15e9ae60aad50"},
		{"77f8d0e62982ae95a91b31b2d1ec05035784c4d63c91d213c499a4e6fac839057c53c15eaff6716c150a3888f05a4a13", "1fb813936193004802f311f7b24e034ca52d121c4e715cbe9f3ce6dd3f819356"},
		{"f094a37f1631d595b40aec0ce15822f8a66e7403aad52858dc30e1a740ee83abe4a47d574725f90e9d2149d3afca5309", "b433b444d92f22f05f59b27e150442d1731c95e002610b5da3f265b0c25a6561"},
		{"86d8c9181fc53c83d5c5a59ff71a5552422a4d5f50125c8235605bbed442250ab5eedc2a9af96e72d9df185af0b4620c", "b70c758757675dce385eb2a6019ae4b9768836a29cb8faa50e8a8016c603871d"},
		{"69327dcb847bc70a01fad93968cc746628bf1422ff92b8b32202015ff75d85f8a3edd0a8c738c1d8c6a465ab895a6d13", "cdc4d41677731c59ae715be9a250fa0a674cabfe70b200afc1c11b651f977134"},
		{"7493640a45a5cecbf761ce67b4070d1864e27972ed3ca7c55fd7c1f191be3dc3b5fca52202d4c5fae1156d1659cf4c97", "488a0d363880eaa61904ceadfcdcde361321926b532001e9b33f9e066eb76730"},
		{"441e760d4d1d6fadc9dc4d67194375e14500c7c82d2fa8eb460bc9980dba72f213a34840ff10fd7ebf17cdf8215f7807", "bca8878d4b7da98864ca552843f7b0bb35bbc62b793ef0320b810549e50d0059"},
		{"e44a73e261e0650238743e889e8ba28d77f6d760e4dddc8979b64c2b76716443801b6335700a32d8da477494cafeb301", "d35d194f207d2f1e5032ef4f138c6d732fa371b259f8a954a2c7f4b1bd0cd94e"},
		{"73b97bd2114bd822efdb31fda3bd45d32b11ea2277d88a7e5b7de25e81363aa8a4f2d069111b08e215b933cd96705902", "7760a6f816e7d345554dc4559d9514eb5947226a349d464eda3a75f1af679a26"},
		{"123f4ba326f048e9a9f650e7769db67d17b6a20198aec99fe92c74a8055fc78a9ec28171a1a09ab6d508172100974a06", "097af324bc4757ba78553480f11c0ba001c3cc96590f8537215a1514329f560e"},
		{"0b859a8b28dfe7898f4f4e1cc346cc705da88403157f17813715d5c4e73e99fd2ac0d4c7afd1c87e8268c3f9380fdd05", "313a7f8c831827383a350b6c55a9d24b4532e63f610ac187bc7a92303ec5c604"},
		{"77ce9b523ebc8339e0bce80d249954797e9749be4bf73bdc5bec030a81207be9d76051982c5b80c628de39c4c8e22598", "9d1452e022fd6a280ce7baf4ac3a7151cd71af54f3d0894289f95aa39ad4d439"},
		{"cecdd9c3b88b68ea1a84c7d9e14e6a0112e27a8e7883730ddf129526b513f7630af673f9a89f5f28341163bc3f040e80", "07165b308d2924c30ca2a3e7205f46cd6094c8cfca62e1fb46b0e9ba4290043c"},
		{"da6e4afdafbd87cebe3a7bfa165d118912e2ff180795196f49b0129bd777d671d78486b2660c5528aaee27aa2c314d03", "350fcb263739dc7b718982a9180037da35d067886d7a569854497d6dca56fc39"},
		{"80e59409d68e0caeb5105c02234a861fd8bbc6962928d50f5f244e7ce73fb5425fc0668dd70207a3bcd0135372a8820e", "eb8af67f82be8f3ac0c01f4bf438e4215603cf07e4cf154932f9f4f2e220cd47"},
		{"dfe1f5b9f22293440603171f7b0234ed76a5c3db16de169e94a2d04615e32db6b5099ea9e7607aea9f41694b3ee2a387", "427880ecb5a18008be31af70ac35feba1b7ecb774750509c46fedc6fe87a3c03"},
		{"211c5709ec1d114b427d956a4d4433e3798ec214740ccd4c8dcc8688d8f5e2b54614f1b66c8032d76e8617ada8836e14", "dd00cdc07bc1454de81e99c7bcf08d96f5487f16c6b08c8c25642bf6f671671c"},
		{"b7f66b9f9f023c98b5e4c5ca751561d55914e96d8db25aa1a2d482f4dfa9c38049a1697b432c9500adf9b0a2cc8d1390", "f9679df2366b7076068dcb62478c436d488a1234f46bc5d23a2582c30504ce39"},
		{"24fc0f43dc21bfe7252bc26fd1ecd4c108b5ca6dcd6e213b4497ee3b9f39704a31ce4e4dd4dab8ecc4a0cccbbb9d050f", "818e23447a8c4028d57e3d450c7eb78312fca20a11168df831c629468f3da93e"},
		{"3cf72ba7a2e414f3e3e42591f19b7811cc86abb09b689bcb321bac2280f59e0a052bafbf0dc05f9671ef3d48e6ea0892", "7852d030e30ef02fccddade41dd11dad0761ff3e0de2bf80a220afda46e60a04"},
		{"525cf97855946b70709d2c56313ee51376082681c9a351a1829d49a6d18f337bdca6e5cf2219948b46b733dbe2657e05", "507f1cf227ecc51b58a6cb260a2ad88d0fae52b14df33080ffe634d125420d3a"},
		{"2d43b3290b7d056e9f3d470d5c16a5018dc102b25ab5ccff98c4a7610241c956352d56652466158dd92024d65ad8300a", "dc9a9e84e89c316d7e82f17b6af209f7eecfa0592514b86ed367a0f36e91894f"},
		{"16adbe52c89f86d6746e6135224eb0268db5b82fdc9f3332505daf7a79aab69b274cfc77ab63e0f6ecca5e1bf970fe17", "4464353707dd05051ef67ab31070b44e524c1a31f47681e7a88ae97142a13955"},
		{"1432dc6b2c5c1b546acade2a96482350f2d5369ae7e6eb7cbe5e6abc4680237336ad9715552a110ce7d15c75a5440d80", "93ff03d1944a65e99cbf205094246c19c0269dfd17e75c4094fdb7ad99151b43"},
		{"4f0b413335c109ac5fb56229816bd4aa6969678922125c9ecbbc1b371ed73dab6e743b6bc5ecedfd89a636dec3cb3396", "00de6ebd1259754b4d13117ed98410e6f794de52c91e85f827b2580b6a8e8a28"},
		{"2bd7f336338b4af9ee6ffb24ebd4076b7bc3805bc496325379649356c1d98cb7e1245cd184c885f6271e8c08d765a603", "309ab0f63625e579b9294209fa00b949ed92f1c2203ef7e149d8e9713a45a551"},
		{"7d86fe374d3e108b6988503428984b5a34e5b71746e5371c0f8df0ed29cc5852d89afb395a33503d580f6cef521b9e96", "49e42da63171901d51b62abc234100ecc9c07b22e6f180a747da15da66fa280e"},
		{"95137562374a17ed96a3272a05889e52630a7c7c4c3a7d4c389d9a067805ad246b259e60949260aabc704a89beb10c05", "d0d08d77a535903dcc509f016825a12f6f49219bfef6e16fcc6d29b9b30de30f"},
	}
)

// Public key first, private key second
func init() {
	const a = 100
	addrs = make([]common.Address, a)

	for i := 0; i < a; i++ {
		addr := common.Address{}
		addr.SetBytes(
			big.NewInt(int64(accountGenerator.Int63n(fixedRandomGen))).Bytes(),
		)
		addrs[i] = addr
		someValidator := &staking.Validator{}
		someValidator.Address = addr
		low := sequenceL.Intn(fixedRandomGenStakeL)
		high := sequenceL.Intn(fixedRandomGenStakeH)
		r := sequenceL.Intn(fixedRandomGenStakeH) + 1
		modBy := high - low + 1
		if modBy <= 0 {
			modBy *= -1
			modBy++
		}
		someValidator.Stake = new(big.Int).Abs(big.NewInt(int64(
			(r % modBy) + low,
		)))
		const amt = 2
		pubKeys := make([]shard.BlsPublicKey, amt)
		for j := 0; j < amt; j++ {
			pair := accounts[i+j]
			priKey := bls.SecretKey{}
			pubKey := bls.PublicKey{}

			pubKey.DeserializeHexStr(pair[0])
			priKey.DeserializeHexStr(pair[1])

			k := shard.BlsPublicKey{}
			k.FromLibBLSPublicKey(&pubKey)
			pubKeys[j] = k
		}

		someValidator.SlotPubKeys = pubKeys
		tempBank[addr] = someValidator
	}
	// b, _ := json.Marshal(TestBank)
	// fmt.Println("keys", string(b))
}

// ValidatorCandidates returns the up to date validator candidates for next epoch
func (bc *BlockChain) ValidatorCandidates() []common.Address {
	return addrs
}

// ValidatorInformation returns the information of validator
func (bc *BlockChain) ValidatorInformation(addr common.Address) (*staking.Validator, error) {
	return tempBank[addr], nil
}

// DelegatorsInformation returns up to date information of delegators of a given validator address
func (bc *BlockChain) DelegatorsInformation(addr common.Address) []*staking.Delegation {
	return make([]*staking.Delegation, 0)
}

// ValidatorStakingWithDelegation returns the amount of staking after applying all delegated stakes
func (bc *BlockChain) ValidatorStakingWithDelegation(addr common.Address) *big.Int {
	return big.NewInt(0)
}
