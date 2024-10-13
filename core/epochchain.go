package core

import (
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
)

type SideEffect func()

type EpochChain struct {
	chainConfig   *params.ChainConfig
	genesisBlock  *types.Block
	db            ethdb.Database // Low level persistent database to store final content in.
	mu            chan struct{}
	currentHeader atomic.Value // Current head of the blockchain.
	engine        consensus_engine.Engine
	vmConfig      *vm.Config

	headerCache     *lru.Cache // Cache for the most recent block headers
	numberCache     *lru.Cache // Cache for the most recent block numbers
	canonicalCache  *lru.Cache // number -> Hash
	shardStateCache *lru.Cache // Cache for shard states where key is epoch number.

	Stub
}

func cache(size int) *lru.Cache {
	cache, _ := lru.New(size)
	return cache
}

func NewEpochChain(db ethdb.Database, chainConfig *params.ChainConfig,
	engine consensus_engine.Engine, vmConfig vm.Config) (*EpochChain, error) {

	hash := rawdb.ReadCanonicalHash(db, 0)
	genesisBlock := rawdb.ReadBlock(db, hash, 0)
	if genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	bc := &EpochChain{
		chainConfig:   chainConfig,
		genesisBlock:  genesisBlock,
		db:            db,
		mu:            make(chan struct{}, 1),
		currentHeader: atomic.Value{},
		engine:        engine,
		vmConfig:      &vmConfig,

		headerCache:     cache(headerCacheLimit),
		numberCache:     cache(numberCacheLimit),
		canonicalCache:  cache(canonicalCacheLimit),
		shardStateCache: cache(shardCacheLimit),

		Stub: Stub{Name: "EpochChain"},
	}

	head := rawdb.ReadHeadHeaderHash(db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		utils.Logger().Warn().Msg("Empty database, resetting chain")
		err := bc.resetWithGenesisBlock(bc.genesisBlock)
		if err != nil {
			return nil, err
		}
	} else {
		header := bc.GetHeaderByHash(head)
		if header == nil {
			return nil, errors.New("failed to initialize: missing header")
		}

		bc.currentHeader.Store(header)
	}
	return bc, nil
}

func (bc *EpochChain) resetWithGenesisBlock(genesis *types.Block) error {
	// Prepare the genesis block and reinitialise the chain
	if err := rawdb.WriteHeader(bc.db, genesis.Header()); err != nil {
		return err
	}
	bc.currentHeader.Store(bc.genesisBlock.Header())
	return rawdb.WriteHeadBlockHash(bc.db, bc.genesisBlock.Hash())
}

func (bc *EpochChain) ShardID() uint32 {
	return 0
}

func (bc *EpochChain) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(bc.currentHeader.Load().(*block.Header))
}

func (bc *EpochChain) Stop() {
	bc.mu <- struct{}{}
	time.AfterFunc(1*time.Second, func() {
		<-bc.mu
	})
}

func (bc *EpochChain) InsertChain(blocks types.Blocks, _ bool) (int, error) {
	if len(blocks) == 0 {
		return 0, nil
	}
	bc.mu <- struct{}{}
	defer func() {
		<-bc.mu
	}()
	for i, block := range blocks {
		if !block.IsLastBlockInEpoch() {
			return i, ErrNotLastBlockInEpoch
		}
		sig, bitmap, err := chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
		if err != nil {
			return i, errors.Wrap(err, "parse commitSigAndBitmap")
		}

		// Signature validation.
		err = bc.Engine().VerifyHeaderSignature(bc, block.Header(), sig, bitmap)
		if err != nil {
			return i, errors.Wrap(err, "failed signature validation")
		}

		ep := block.Header().Epoch()
		next := ep.Add(ep, common.Big1)
		batch := bc.db.NewBatch()
		_, se1, err := bc.writeShardStateBytes(batch, next, block.Header().ShardState())
		if err != nil {
			return i, err
		}
		se2, err := bc.writeHeadBlock(batch, block)
		if err != nil {
			return i, err
		}
		err = rawdb.WriteHeadHeaderHash(batch, block.Hash())
		if err != nil {
			return i, err
		}
		err = rawdb.WriteHeaderNumber(batch, block.Hash(), block.NumberU64())
		if err != nil {
			return i, err
		}
		err = rawdb.WriteHeader(batch, block.Header())
		if err != nil {
			return i, err
		}
		if err := batch.Write(); err != nil {
			return i, err
		}
		se1()
		se2()
		utils.Logger().Info().
			Msgf("[EPOCHSYNC] Added block %d, epoch %d, %s", block.NumberU64(), block.Epoch().Uint64(), block.Hash().Hex())

	}
	return 0, nil
}

func (bc *EpochChain) CurrentHeader() *block.Header {
	return bc.currentHeader.Load().(*block.Header)
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (bc *EpochChain) GetBlockNumber(hash common.Hash) *uint64 {
	if cached, ok := bc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}
	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number != nil {
		bc.numberCache.Add(hash, *number)
	}
	return number
}

func (bc *EpochChain) GetHeaderByHash(hash common.Hash) *block.Header {
	number := bc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetHeader(hash, *number)
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (bc *EpochChain) GetHeader(hash common.Hash, number uint64) *block.Header {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := bc.headerCache.Get(hash); ok {
		return header.(*block.Header)
	}
	header := rawdb.ReadHeader(bc.db, hash, number)
	if header == nil {
		return nil
	}
	// Cache the found header for next time and return
	bc.headerCache.Add(hash, header)
	return header
}

func (bc *EpochChain) GetCanonicalHash(number uint64) common.Hash {
	return rawdb.ReadCanonicalHash(bc.db, number)
}

func (bc *EpochChain) GetHeaderByNumber(number uint64) *block.Header {
	hash := bc.getHashByNumber(number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetHeader(hash, number)
}

func (bc *EpochChain) getHashByNumber(number uint64) common.Hash {
	// Since canonical chain is immutable, it's safe to read header
	// hash by number from cache.
	if hash, ok := bc.canonicalCache.Get(number); ok {
		return hash.(common.Hash)
	}
	hash := rawdb.ReadCanonicalHash(bc.db, number)
	if hash != (common.Hash{}) {
		bc.canonicalCache.Add(number, hash)
	}
	return hash
}

func (bc *EpochChain) Config() *params.ChainConfig {
	return bc.chainConfig
}

func (bc *EpochChain) Engine() engine.Engine {
	return bc.engine
}

func (bc *EpochChain) ReadShardState(epoch *big.Int) (*shard.State, error) {
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

// pure function
func (bc *EpochChain) writeShardStateBytes(db rawdb.DatabaseWriter,
	epoch *big.Int, shardState []byte,
) (*shard.State, SideEffect, error) {
	decodeShardState, err := shard.DecodeWrapper(shardState)
	if err != nil {
		return nil, nil, err
	}
	err = rawdb.WriteShardStateBytes(db, epoch, shardState)
	if err != nil {
		return nil, nil, err
	}
	cacheKey := string(epoch.Bytes())
	se := func() {
		bc.shardStateCache.Add(cacheKey, decodeShardState)
	}
	return decodeShardState, se, nil
}

// WriteHeadBlock writes a new head block.
func (bc *EpochChain) WriteHeadBlock(block *types.Block) error {
	batch := bc.db.NewBatch()
	se, err := bc.writeHeadBlock(batch, block)
	if err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return err
	}
	se()
	return nil
}

// pure function
func (bc *EpochChain) writeHeadBlock(db rawdb.DatabaseWriter, block *types.Block) (SideEffect, error) {
	// Add the block to the canonical chain number scheme and mark as the head
	if err := rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64()); err != nil {
		return nil, err
	}
	if err := rawdb.WriteHeadBlockHash(db, block.Hash()); err != nil {
		return nil, err
	}

	return func() {
		bc.currentHeader.Store(block.Header())
	}, nil
}

func (bc *EpochChain) IsSameLeaderAsPreviousBlock(block *types.Block) bool {
	if IsEpochBlock(block) {
		return false
	}

	previousHeader := bc.GetHeaderByNumber(block.NumberU64() - 1)
	if previousHeader == nil {
		return false
	}
	return block.Coinbase() == previousHeader.Coinbase()
}

func (bc *EpochChain) GetVMConfig() *vm.Config {
	return bc.vmConfig
}

func (bc *EpochChain) CommitPreimages() error {
	// epoch chain just has last block, which does not have any txs
	// so no pre-images here
	return nil
}
