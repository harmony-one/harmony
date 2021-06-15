package explorer

import (
	"fmt"
	"math/big"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Constants for storage.
const (
	AddressPrefix        = "ad"
	CheckpointPrefix     = "dc"
	PrefixLen            = 3
	addrIndexCacheSize   = 1000
	flushNumberThreshold = 100              // flush every 100 blocks
	flushTimeThreshold   = 90 * time.Second // flush every 90 seconds
	newBlockCBuffer      = 100
	catchupBlockCBuffer  = 1
)

type oneAddress string

// GetAddressKey ...
func GetAddressKey(address oneAddress) []byte {
	return []byte(fmt.Sprintf("%s_%s", AddressPrefix, address))
}

// GetCheckpointKey ...
func GetCheckpointKey(blockNum *big.Int) []byte {
	return []byte(fmt.Sprintf("%s_%x", CheckpointPrefix, blockNum))
}

var storage *Storage
var once sync.Once

// Storage dump the block info into leveldb.
type Storage struct {
	db       *leveldb.DB
	clean    *lru.Cache
	dirty    map[oneAddress]*Address
	dirtyBNs map[uint64]struct{}

	lock          sync.RWMutex
	newBlockC     chan *types.Block
	catchupBlockC chan *types.Block
	closeC        chan struct{}
}

// GetStorageInstance returns attack model by using singleton pattern.
func GetStorageInstance(ip, port string) *Storage {
	once.Do(func() {
		var err error
		storage, err = newStorageInstance(ip, port)
		// TODO: refactor this
		if err != nil {
			fmt.Println("failed to open explorer db:", err.Error())
			os.Exit(1)
		}
	})
	return storage
}

func newStorageInstance(ip, port string) (*Storage, error) {
	db, err := newStorageDB(ip, port)
	if err != nil {
		return nil, err
	}
	clean, _ := lru.New(addrIndexCacheSize)
	dirty := make(map[oneAddress]*Address)
	return &Storage{
		db:            db,
		clean:         clean,
		dirty:         dirty,
		dirtyBNs:      make(map[uint64]struct{}),
		newBlockC:     make(chan *types.Block, newBlockCBuffer),
		catchupBlockC: make(chan *types.Block, catchupBlockCBuffer),
		closeC:        make(chan struct{}),
	}, nil
}

func newStorageDB(ip, port string) (*leveldb.DB, error) {
	dbFileName := path.Join(nodeconfig.GetDefaultConfig().DBDir, "explorer_storage_"+ip+"_"+port)
	utils.Logger().Info().Msg("explorer storage folder: " + dbFileName)
	// https://github.com/ethereum/go-ethereum/blob/master/ethdb/leveldb/leveldb.go#L98 options.
	// We had 0 for handles and cache params before, so set 0s for all of them. Filter opt is the same.
	options := &opt.Options{
		OpenFilesCacheCapacity: 0,
		BlockCacheCapacity:     0,
		WriteBuffer:            0,
		Filter:                 filter.NewBloomFilter(10),
	}
	db, err := leveldb.OpenFile(dbFileName, options)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new database")
		return nil, err
	}
	return db, nil
}

func (storage *Storage) Start() {
	go storage.loop()
}

func (storage *Storage) Close() {
	storage.closeC <- struct{}{}
	<-storage.closeC
}

func (storage *Storage) loop() {
	defer func() {
		if err := storage.flushLocked(); err != nil {
			utils.Logger().Error().Err(err).Msg("[explorer] failed to flush at close")
		}
		close(storage.closeC)
	}()

	var lastFlush = time.Now()

	for {
		// flush when reached threshold
		storage.lock.RLock()
		numDirty := len(storage.dirtyBNs)
		storage.lock.RUnlock()
		if numDirty >= flushNumberThreshold || time.Since(lastFlush) >= flushTimeThreshold {
			lastFlush = time.Now()
			if err := storage.flushLocked(); err != nil {
				utils.Logger().Error().Err(err).Msg("[explorer] failed to flush")
			}
		}
		// prioritize processing new blocks and close C
		select {
		case b := <-storage.newBlockC:
			if err := storage.handleBlock(b); err != nil {
				utils.Logger().Error().Err(err).Msg("[explorer] handle new block result failed")
			}
			continue
		case <-storage.closeC:
			return
		default:
		}
		// handles catchupBlock
		select {
		case b := <-storage.newBlockC:
			if err := storage.handleBlock(b); err != nil {
				utils.Logger().Error().Err(err).Msg("[explorer] handle new block result failed")
			}
		case b := <-storage.catchupBlockC:
			if err := storage.handleBlock(b); err != nil {
				utils.Logger().Error().Err(err).Msg("[explorer] handle catch up block failed")
			}
		case <-storage.closeC:
			return
		}
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *leveldb.DB {
	return storage.db
}

// DumpNewBlock extracts information from block and index them into leveldb for explorer.
func (storage *Storage) DumpNewBlock(block *types.Block) {
	if block == nil {
		return
	}
	storage.lock.RLock()
	if storage.isBlockComputed(block.NumberU64()) {
		storage.lock.RUnlock()
		return
	}
	storage.lock.RUnlock()

	storage.newBlockC <- block // blocking if channel is full
}

func (storage *Storage) DumpCatchupBlock(block *types.Block) {
	if block == nil {
		return
	}
	storage.lock.RLock()
	if storage.isBlockComputed(block.NumberU64()) {
		storage.lock.RUnlock()
		return
	}
	storage.lock.RUnlock()

	storage.catchupBlockC <- block
}

func (storage *Storage) isBlockComputed(bn uint64) bool {
	if _, isDirty := storage.dirtyBNs[bn]; isDirty {
		return true
	}
	blockCheckpoint := GetCheckpointKey(new(big.Int).SetUint64(bn))
	if _, err := storage.GetDB().Get(blockCheckpoint, nil); err == nil {
		return true
	}
	return false
}

func (storage *Storage) handleBlock(b *types.Block) error {
	txs, stks := computeAccountsTransactionsMapForBlock(b)
	return storage.handleBlockResultLocked(b.NumberU64(), txs, stks)
}

func (storage *Storage) handleBlockResultLocked(bn uint64, txs map[oneAddress]TxRecords, stkTxs map[oneAddress]TxRecords) error {
	storage.lock.Lock()
	defer storage.lock.Unlock()

	if storage.isBlockComputed(bn) {
		return nil
	}
	for address, txRecords := range txs {
		if err := storage.UpdateTxAddressStorage(address, txRecords, false /* isStaking */); err != nil {
			utils.Logger().Error().Err(err).Str("address", string(address)).
				Msg("[explorer] failed to update address")
			return err
		}
	}
	for address, txRecords := range stkTxs {
		if err := storage.UpdateTxAddressStorage(address, txRecords, true /* isStaking */); err != nil {
			utils.Logger().Error().Err(err).Str("address", string(address)).
				Msg("[explorer] failed to update address")
			return err
		}
	}
	storage.dirtyBNs[bn] = struct{}{}
	return nil
}

// UpdateTxAddressStorage updates specific addr tx Address.
func (storage *Storage) UpdateTxAddressStorage(addr oneAddress, txRecords TxRecords, isStaking bool) error {
	address, err := storage.getAddressInfo(addr)
	if err != nil {
		return err
	}

	address.ID = string(addr)
	if isStaking {
		address.StakingTXs = append(address.StakingTXs, txRecords...)
	} else {
		address.TXs = append(address.TXs, txRecords...)
	}
	storage.dirty[addr] = address
	return nil
}

func (storage *Storage) flushLocked() error {
	storage.lock.RLock()
	batch := new(leveldb.Batch)
	for addr, addressInfo := range storage.dirty {
		key := GetAddressKey(addr)
		encoded, err := rlp.EncodeToBytes(addressInfo)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("error when flushing explorer")
			return err
		}
		batch.Put(key, encoded)
	}
	for bn := range storage.dirtyBNs {
		key := GetCheckpointKey(new(big.Int).SetUint64(bn))
		batch.Put(key, []byte{})
	}
	storage.lock.RUnlock()

	// Hack: during db write, dirty is read-only. We can release the lock for now.
	if err := storage.db.Write(batch, nil); err != nil {
		return errors.Wrap(err, "failed to write explorer data")
	}
	// clean up
	storage.lock.Lock()
	defer storage.lock.Unlock()
	for addr, addressInfo := range storage.dirty {
		storage.clean.Add(addr, addressInfo)
	}
	storage.dirty = make(map[oneAddress]*Address)
	storage.dirtyBNs = make(map[uint64]struct{})
	return nil
}

// GetAddressInfo get the address info of the given address
func (storage *Storage) GetAddressInfo(addr string) (*Address, error) {
	storage.lock.RLock()
	defer storage.lock.RUnlock()

	return storage.getAddressInfo(oneAddress(addr))
}

func (storage *Storage) getAddressInfo(addr oneAddress) (*Address, error) {
	if addrInfo, ok := storage.dirty[addr]; ok {
		return addrInfo, nil
	}
	if x, ok := storage.clean.Get(addr); ok {
		return x.(*Address), nil
	}
	addrInfo, err := storage.readAddressInfoFromDB(addr)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return &Address{
				ID: string(addr),
			}, nil
		}
		return nil, err
	}
	storage.clean.Add(addr, addrInfo)
	return addrInfo, nil
}

func (storage *Storage) readAddressInfoFromDB(addr oneAddress) (*Address, error) {
	key := GetAddressKey(addr)
	val, err := storage.GetDB().Get(key, nil)
	if err != nil {
		return nil, err
	}
	var addrInfo *Address
	if err := rlp.DecodeBytes(val, &addrInfo); err != nil {
		return nil, err
	}
	return addrInfo, nil
}

// GetAddresses returns size of addresses from address with prefix.
func (storage *Storage) GetAddresses(size int, prefix oneAddress) ([]string, error) {
	db := storage.GetDB()
	key := GetAddressKey(prefix)
	iterator := db.NewIterator(&util.Range{Start: []byte(key)}, nil)
	addresses := make([]string, 0)
	read := 0
	for iterator.Next() && read < size {
		address := string(iterator.Key())
		read++
		if len(address) < PrefixLen {
			utils.Logger().Info().Msgf("address len < 3 %s", address)
			continue
		}
		addresses = append(addresses, address[PrefixLen:])
	}
	iterator.Release()
	if err := iterator.Error(); err != nil {
		utils.Logger().Error().Err(err).Msg("iterator error")
		return nil, err
	}
	return addresses, nil
}

func computeAccountsTransactionsMapForBlock(
	block *types.Block,
) (map[oneAddress]TxRecords, map[oneAddress]TxRecords) {
	// mapping from account address to TxRecords for txns in the block
	var acntsTxns = make(map[oneAddress]TxRecords)
	// mapping from account address to TxRecords for staking txns in the block
	var acntsStakingTxns = make(map[oneAddress]TxRecords)

	// Store txs
	for _, tx := range block.Transactions() {
		explorerTransaction, err := GetTransaction(tx, block)
		if err != nil {
			utils.Logger().Error().Err(err).Str("txHash", tx.HashByType().String()).
				Msg("[Explorer Storage] Failed to get GetTransaction mapping")
			continue
		}
		from := oneAddress(explorerTransaction.From)
		to := oneAddress(explorerTransaction.To)

		// store as sent transaction with from address
		txRecord := &TxRecord{
			Hash:      explorerTransaction.ID,
			Type:      Sent,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntTxns, ok := acntsTxns[from]
		if !ok {
			acntTxns = make(TxRecords, 0)
		}
		acntTxns = append(acntTxns, txRecord)
		acntsTxns[from] = acntTxns

		// store as received transaction with to address
		txRecord = &TxRecord{
			Hash:      explorerTransaction.ID,
			Type:      Received,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntTxns, ok = acntsTxns[to]
		if !ok {
			acntTxns = make(TxRecords, 0)
		}
		acntTxns = append(acntTxns, txRecord)
		acntsTxns[to] = acntTxns
	}

	// Store staking txns
	for _, tx := range block.StakingTransactions() {
		explorerTransaction, err := GetStakingTransaction(tx, block)
		if err != nil {
			utils.Logger().Error().Err(err).Str("txHash", tx.Hash().String()).
				Msg("[Explorer Storage] Failed to get StakingTransaction mapping")
			continue
		}
		from := oneAddress(explorerTransaction.From)
		to := oneAddress(explorerTransaction.To)

		// store as sent staking transaction with from address
		txRecord := &TxRecord{
			Hash:      explorerTransaction.ID,
			Type:      Sent,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntStakingTxns, ok := acntsStakingTxns[from]
		if !ok {
			acntStakingTxns = make(TxRecords, 0)
		}
		acntStakingTxns = append(acntStakingTxns, txRecord)
		acntsStakingTxns[from] = acntStakingTxns

		// For delegate/undelegate, also store as received staking transaction with to address
		txRecord = &TxRecord{
			Hash:      explorerTransaction.ID,
			Type:      Received,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntStakingTxns, ok = acntsStakingTxns[to]
		if !ok {
			acntStakingTxns = make(TxRecords, 0)
		}
		acntStakingTxns = append(acntStakingTxns, txRecord)
		acntsStakingTxns[to] = acntStakingTxns
	}

	return acntsTxns, acntsStakingTxns
}
