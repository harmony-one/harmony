package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/spf13/cobra"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/cli"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
)

var batchFlag = cli.IntFlag{
	Name:      "batch",
	Shorthand: "b",
	Usage:     "batch size limit in MB",
	DefValue:  512,
}

var dumpDBCmd = &cobra.Command{
	Use:     "dumpdb srcdb destdb [startKey [endKey [firstStateStartKey [firstStateEndKey]",
	Short:   "dump a snapshot db.",
	Long:    "dump a snapshot db.",
	Example: "harmony dumpdb /srcDir/harmony_db_0 /destDir/harmony_db_0",
	Args:    cobra.RangeArgs(2, 6),
	Run: func(cmd *cobra.Command, args []string) {
		srcDBDir, destDBDir := args[0], args[1]
		var batchLimitMB int
		var startKey []byte
		var endKey []byte
		var firstStateStartKey []byte
		var firstStateEndKey []byte
		if len(args) > 2 {
			_startKey, err := hexutil.Decode(args[2])
			if err != nil {
				fmt.Println("invalid startKey:", err)
				os.Exit(-1)
			}
			startKey = _startKey
		}
		if len(args) > 3 {
			_endKey, err := hexutil.Decode(args[3])
			if err != nil {
				fmt.Println("invalid endKey:", err)
				os.Exit(-1)
			}
			endKey = _endKey
		}
		if len(args) > 4 {
			_startKey, err := hexutil.Decode(args[4])
			if err != nil {
				fmt.Println("invalid stateStartKey:", err)
				os.Exit(-1)
			}
			firstStateStartKey = _startKey
		}
		if len(args) > 5 {
			_endKey, err := hexutil.Decode(args[5])
			if err != nil {
				fmt.Println("invalid stateEndKey:", err)
				os.Exit(-1)
			}
			firstStateEndKey = _endKey
		}
		batchLimitMB = cli.GetIntFlagValue(cmd, batchFlag)
		networkType := getNetworkType(cmd)
		shardSchedule = getShardSchedule(networkType)
		if shardSchedule == nil {
			fmt.Println("unsupported network type")
			os.Exit(-1)
		}
		fmt.Println(srcDBDir, destDBDir, batchLimitMB, hexutil.Encode(startKey), hexutil.Encode(endKey), hexutil.Encode(firstStateStartKey), hexutil.Encode(firstStateEndKey))
		dumpMain(srcDBDir, destDBDir, batchLimitMB*MB, startKey, endKey, firstStateStartKey, firstStateEndKey)
		os.Exit(0)
	},
}

func getShardSchedule(networkType nodeconfig.NetworkType) shardingconfig.Schedule {
	switch networkType {
	case nodeconfig.Mainnet:
		return shardingconfig.MainnetSchedule
	case nodeconfig.Testnet:
		return shardingconfig.TestnetSchedule
	case nodeconfig.Pangaea:
		return shardingconfig.PangaeaSchedule
	case nodeconfig.Localnet:
		return shardingconfig.LocalnetSchedule
	case nodeconfig.Partner:
		return shardingconfig.PartnerSchedule
	case nodeconfig.Stressnet:
		return shardingconfig.StressNetSchedule
	}
	return nil
}

func registerDumpDBFlags() error {
	return cli.RegisterFlags(dumpDBCmd, []cli.Flag{batchFlag, networkTypeFlag})
}

type KakashiDB struct {
	ethdb.Database
	toDB       ethdb.Database
	toDBBatch  ethdb.Batch
	batchLimit int
	cache      *lru.Cache
}

const (
	MB                 = 1024 * 1024
	BLOCKS_DUMP        = 512 // must >= 256
	EPOCHS_DUMP        = 10
	STATEDB_CACHE_SIZE = 64 // size in MB
	LEVELDB_CACHE_SIZE = 256
	LEVELDB_HANDLES    = 1024
	LRU_CACHE_SIZE     = 64 * 1024 * 1024
)

const (
	NONE = iota
	ON_ACCOUNT_START
	ON_ACCOUNT_STATE
	ON_ACCOUNT_END
)

var (
	totalSize    = 0 // current dump size
	printSize    = 0 // last print dump size
	flushSize    = 0 // size flushed into db
	accountCount = 0 // number of accounts
	lastAccount  = state.DumpAccount{
		Address: &common.Address{},
	}
	savedStateKey hexutil.Bytes
	accountState  = NONE
	emptyHash     = common.Hash{}
	shardSchedule shardingconfig.Schedule // init by cli flag
)

func dumpPrint(prefix string, showAccount bool) {
	if totalSize-printSize > MB || showAccount {
		now := time.Now().Unix()
		fmt.Println(now, prefix, accountCount, totalSize, printSize/MB, flushSize/MB)
		if showAccount {
			fmt.Println("account:", lastAccount.Address.Hex(), lastAccount.Balance, len(lastAccount.Code), accountState, lastAccount.SecureKey.String(), savedStateKey.String())
		}
		printSize = totalSize
	}
}

func (db *KakashiDB) Get(key []byte) ([]byte, error) {
	value, err := db.Database.Get(key)
	if exist, _ := db.cache.ContainsOrAdd(string(key), nil); !exist {
		db.copyKV(key, value)
	}
	return value, err
}
func (db *KakashiDB) Put(key []byte, value []byte) error {
	return nil
}

// Delete removes the key from the key-value data store.
func (db *KakashiDB) Delete(key []byte) error {
	return nil
}

// copy key,value to toDB
func (db *KakashiDB) copyKV(key, value []byte) {
	db.toDBBatch.Put(key, value)
	totalSize += len(key) + len(value)
	dumpPrint("copyKV", false)
}

func (db *KakashiDB) flush() {
	dumpPrint("KakashiDB batch writhing", true)
	db.toDBBatch.Write()
	db.toDBBatch.Reset()
	flushSize = totalSize
	dumpPrint("KakashiDB flushed", false)
}

func (db *KakashiDB) Close() error {
	db.toDBBatch.Reset() // drop dirty cache
	fmt.Println("KakashiDB Close")
	db.toDB.Close()
	return db.Database.Close()
}

func (db *KakashiDB) OnRoot(common.Hash) {}

// OnAccount implements DumpCollector interface
func (db *KakashiDB) OnAccountStart(addr common.Address, acc state.DumpAccount) {
	accountState = ON_ACCOUNT_START
	lastAccount = acc
	lastAccount.Address = &addr
}

// OnAccount implements DumpCollector interface
func (db *KakashiDB) OnAccountState(addr common.Address, StateSecureKey hexutil.Bytes, key, value []byte) {
	accountState = ON_ACCOUNT_STATE
	if totalSize-flushSize > int(db.batchLimit) {
		savedStateKey = StateSecureKey
		db.flush()
	}
}

// OnAccount implements DumpCollector interface
func (db *KakashiDB) OnAccountEnd(addr common.Address, acc state.DumpAccount) {
	accountCount++
	accountState = ON_ACCOUNT_END
	if totalSize-flushSize > int(db.batchLimit) {
		db.flush()
	}
}

func (db *KakashiDB) getHashByNumber(number uint64) common.Hash {
	hash := rawdb.ReadCanonicalHash(db, number)
	return hash
}
func (db *KakashiDB) GetHeaderByNumber(number uint64) *block.Header {
	hash := db.getHashByNumber(number)
	if hash == (common.Hash{}) {
		return nil
	}
	return db.GetHeader(hash, number)
}

func (db *KakashiDB) GetHeader(hash common.Hash, number uint64) *block.Header {
	header := rawdb.ReadHeader(db, hash, number)
	return header
}

func (db *KakashiDB) GetHeaderByHash(hash common.Hash) *block.Header {
	number := rawdb.ReadHeaderNumber(db, hash)
	return rawdb.ReadHeader(db, hash, *number)
}

// GetBlock retrieves a block from the database by hash and number
func (db *KakashiDB) GetBlock(hash common.Hash, number uint64) *types.Block {
	block := rawdb.ReadBlock(db, hash, number)
	return block
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the database
func (db *KakashiDB) GetBlockNumber(hash common.Hash) *uint64 {
	return rawdb.ReadHeaderNumber(db, hash)
}

// GetBlockByHash retrieves a block from the database by hash
func (db *KakashiDB) GetBlockByHash(hash common.Hash) *types.Block {
	number := db.GetBlockNumber(hash)
	return db.GetBlock(hash, *number)
}

// GetBlockByNumber retrieves a block from the database by number
func (db *KakashiDB) GetBlockByNumber(number uint64) *types.Block {
	hash := rawdb.ReadCanonicalHash(db, number)
	return db.GetBlock(hash, number)
}

func (db *KakashiDB) indexerDataDump(block *types.Block) {
	fmt.Println("indexerDataDump:")
	bloomIndexer := hmy.NewBloomIndexer(db, params.BloomBitsBlocks, params.BloomConfirms)
	bloomIndexer.Close() // just stop event loop
	section, blkno, blkhash := bloomIndexer.Sections()
	bloomIndexer.AddCheckpoint(section-1, blkhash)
	for i := blkno; i <= block.NumberU64(); i++ {
		db.GetHeaderByNumber(i)
	}
	db.flush()
}

func (db *KakashiDB) offchainDataDump(block *types.Block) {
	fmt.Println("offchainDataDump:")
	rawdb.WriteHeadBlockHash(db.toDBBatch, block.Hash())
	rawdb.WriteHeadHeaderHash(db.toDBBatch, block.Hash())
	db.GetHeaderByNumber(0)
	db.GetBlockByNumber(0)
	db.GetHeaderByHash(block.Hash())
	// EVM may access the last 256 block hash
	for i := 0; i <= BLOCKS_DUMP; i++ {
		if block.NumberU64() < uint64(i) {
			break
		}
		latestNumber := block.NumberU64() - uint64(i)
		latestBlock := db.GetBlockByNumber(latestNumber)
		db.GetBlockByHash(latestBlock.Hash())
		db.GetHeaderByHash(latestBlock.Hash())
		db.GetBlockByHash(latestBlock.Hash())
		rawdb.ReadBlockRewardAccumulator(db, latestNumber)
		rawdb.ReadBlockCommitSig(db, latestNumber)
		epoch := block.Epoch()
		epochInstance := shardSchedule.InstanceForEpoch(epoch)
		for shard := 0; shard < int(epochInstance.NumShards()); shard++ {
			rawdb.ReadCrossLinkShardBlock(db, uint32(shard), latestNumber)
		}
	}
	headEpoch := block.Epoch()
	epochInstance := shardSchedule.InstanceForEpoch(headEpoch)
	for shard := 0; shard < int(epochInstance.NumShards()); shard++ {
		rawdb.ReadShardLastCrossLink(db, uint32(shard))
	}

	rawdb.IteratorValidatorStats(db, func(it ethdb.Iterator, addr common.Address) bool {
		db.copyKV(it.Key(), it.Value())
		return true
	})
	rawdb.ReadPendingCrossLinks(db)

	rawdb.IteratorDelegatorDelegations(db, func(it ethdb.Iterator, delegator common.Address) bool {
		db.copyKV(it.Key(), it.Value())
		return true
	})
	for i := 0; i < EPOCHS_DUMP; i++ {
		epoch := new(big.Int).Sub(headEpoch, big.NewInt(int64(i)))
		if epoch.Sign() < 0 {
			break
		}
		rawdb.ReadShardState(db, epoch)
		rawdb.ReadEpochBlockNumber(db, epoch)
		rawdb.ReadEpochVrfBlockNums(db, epoch)
		rawdb.ReadEpochVdfBlockNum(db, epoch)
		var validators []common.Address
		rawdb.IteratorValidatorSnapshot(db, func(addr common.Address, _epoch *big.Int) bool {
			if _epoch.Cmp(epoch) == 0 {
				validator, err := rawdb.ReadValidatorSnapshot(db, addr, epoch)
				if err != nil {
					panic(err)
				}
				validators = append(validators, validator.Validator.Address)
			}
			return true
		})
		if i == 0 {
			rawdb.ReadValidatorList(db)
		}
	}

	rawdb.IteratorCXReceiptsProofSpent(db, func(it ethdb.Iterator, shardID uint32, number uint64) bool {
		db.copyKV(it.Key(), it.Value())
		return true
	})
	db.flush()
}

func (db *KakashiDB) stateDataDump(block *types.Block, startKey, endKey, firstStateStartKey, firstStateEndKey []byte) {
	fmt.Println("stateDataDump:")
	stateDB0 := state.NewDatabaseWithCache(db, STATEDB_CACHE_SIZE)
	rootHash := block.Root()
	stateDB, err := state.New(rootHash, stateDB0)
	if err != nil {
		panic(err)
	}
	config := new(state.DumpConfig)
	config.Start = startKey
	config.End = endKey
	config.StateStart = firstStateStartKey
	config.StateEnd = firstStateEndKey
	stateDB.DumpToCollector(db, config)
	db.flush()
}

func dumpMain(srcDBDir, destDBDir string, batchLimit int, startKey, endKey, firstStateStartKey, firstStateEndKey []byte) {
	fmt.Println("===dumpMain===")
	srcDB, err := ethRawDB.NewLevelDBDatabase(srcDBDir, LEVELDB_CACHE_SIZE, LEVELDB_HANDLES, "")
	if err != nil {
		fmt.Println("open src db error:", err)
		os.Exit(-1)
	}
	destDB, err := ethRawDB.NewLevelDBDatabase(destDBDir, LEVELDB_CACHE_SIZE, LEVELDB_HANDLES, "")
	if err != nil {
		fmt.Println("open dest db error:", err)
		os.Exit(-1)
	}

	headHash := rawdb.ReadHeadBlockHash(srcDB)
	headNumber := *rawdb.ReadHeaderNumber(srcDB, headHash)
	fmt.Println("head-block:", headNumber, headHash.Hex())

	if headHash == emptyHash {
		fmt.Println("empty head block hash")
		os.Exit(-1)
	}
	block := rawdb.ReadBlock(srcDB, headHash, headNumber)
	if block == nil {
		fmt.Println("ReadBlock error:")
		os.Exit(-1)
	}
	fmt.Println("start copying...")
	cache, _ := lru.New(LRU_CACHE_SIZE)
	copier := &KakashiDB{
		Database:   srcDB,
		toDB:       destDB,
		toDBBatch:  destDB.NewBatch(),
		batchLimit: batchLimit,
		cache:      cache,
	}
	defer copier.Close()
	copier.offchainDataDump(block)
	copier.indexerDataDump(block)
	copier.stateDataDump(block, startKey, endKey, firstStateStartKey, firstStateEndKey)
}
