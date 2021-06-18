package explorer

import (
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
)

// Constants for storage.
const (
	LegAddressPrefixLen  = 3
	addrIndexCacheSize   = 1000
	flushNumberThreshold = 100              // flush every 100 blocks
	flushTimeThreshold   = 90 * time.Second // flush every 90 seconds
)

//var storage *Storage
var once sync.Once

// Storage dump the block info into leveldb.
type LegStorage struct {
	db       database
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
	dbPath := defaultDBPath(ip, port)
	utils.Logger().Info().Msg("explorer storage folder: " + dbPath)
	db, err := newLvlDB(dbPath)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new database")
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

func defaultDBPath(ip, port string) string {
	return path.Join(nodeconfig.GetDefaultConfig().DBDir, "explorer_storage_"+ip+"_"+port)
}

func (storage *Storage) Start() {
	//go storage.loop()
}

func (storage *Storage) Close() {
	storage.closeC <- struct{}{}
	<-storage.closeC
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
	is, err := isBlockComputedInDB(storage.db, bn)
	if err != nil {
		return false
	}
	return is
}

//
//func (storage *Storage) handleBlock(b *types.Block) error {
//	txs, stks := computeAccountsTransactionsMapForBlock(b)
//	return storage.handleBlockResultLocked(b.NumberU64(), txs, stks)
//}

func (storage *Storage) handleBlockResultLocked(bn uint64, txs map[oneAddress]LegTxRecords, stkTxs map[oneAddress]LegTxRecords) error {
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
func (storage *Storage) UpdateTxAddressStorage(addr oneAddress, txRecords LegTxRecords, isStaking bool) error {
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
	addrInfo, err := readAddressInfoFromDB(storage.db, addr)
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

// GetAddresses returns size of addresses from address with prefix.
func (storage *Storage) GetAddresses(size int, prefix oneAddress) ([]string, error) {
	db := storage.GetDB()
	key := LegGetAddressKey(prefix)
	iterator := db.NewPrefixIterator(key)
	addresses := make([]string, 0)
	read := 0
	for iterator.Next() && read < size {
		address := string(iterator.Key())
		read++
		if len(address) < LegAddressPrefixLen {
			utils.Logger().Info().Msgf("address len < 3 %s", address)
			continue
		}
		addresses = append(addresses, address[LegAddressPrefixLen:])
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
) (map[oneAddress]LegTxRecords, map[oneAddress]LegTxRecords) {
	// mapping from account address to LegTxRecords for txns in the block
	var acntsTxns = make(map[oneAddress]LegTxRecords)
	// mapping from account address to LegTxRecords for staking txns in the block
	var acntsStakingTxns = make(map[oneAddress]LegTxRecords)

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
		txRecord := &LegTxRecord{
			Hash:      explorerTransaction.ID,
			Type:      LegSent,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntTxns, ok := acntsTxns[from]
		if !ok {
			acntTxns = make(LegTxRecords, 0)
		}
		acntTxns = append(acntTxns, txRecord)
		acntsTxns[from] = acntTxns

		// store as received transaction with to address
		txRecord = &LegTxRecord{
			Hash:      explorerTransaction.ID,
			Type:      LegReceived,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntTxns, ok = acntsTxns[to]
		if !ok {
			acntTxns = make(LegTxRecords, 0)
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
		txRecord := &LegTxRecord{
			Hash:      explorerTransaction.ID,
			Type:      LegSent,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntStakingTxns, ok := acntsStakingTxns[from]
		if !ok {
			acntStakingTxns = make(LegTxRecords, 0)
		}
		acntStakingTxns = append(acntStakingTxns, txRecord)
		acntsStakingTxns[from] = acntStakingTxns

		// For delegate/undelegate, also store as received staking transaction with to address
		txRecord = &LegTxRecord{
			Hash:      explorerTransaction.ID,
			Type:      LegReceived,
			Timestamp: explorerTransaction.Timestamp,
		}
		acntStakingTxns, ok = acntsStakingTxns[to]
		if !ok {
			acntStakingTxns = make(LegTxRecords, 0)
		}
		acntStakingTxns = append(acntStakingTxns, txRecord)
		acntsStakingTxns[to] = acntStakingTxns
	}

	return acntsTxns, acntsStakingTxns
}
