package explorer

import (
	"fmt"
	"math/big"
	"os"
	"path"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Constants for storage.
const (
	AddressPrefix    = "ad"
	CheckpointPrefix = "dc"
	PrefixLen        = 3
)

// GetAddressKey ...
func GetAddressKey(address string) string {
	return fmt.Sprintf("%s_%s", AddressPrefix, address)
}

// GetCheckpointKey ...
func GetCheckpointKey(blockNum *big.Int) string {
	return fmt.Sprintf("%s_%x", CheckpointPrefix, blockNum)
}

var storage *Storage
var once sync.Once

// Storage dump the block info into leveldb.
type Storage struct {
	db   *leveldb.DB
	lock sync.Mutex
}

// GetStorageInstance returns attack model by using singleton pattern.
func GetStorageInstance(ip, port string, remove bool) *Storage {
	once.Do(func() {
		storage = &Storage{}
		storage.Init(ip, port, remove)
	})
	return storage
}

// Init initializes the block update.
func (storage *Storage) Init(ip, port string, remove bool) {
	dbFileName := path.Join(nodeconfig.GetDefaultConfig().DBDir, "explorer_storage_"+ip+"_"+port)
	utils.Logger().Info().Msg("explorer storage folder: " + dbFileName)
	var err error
	if remove {
		var err = os.RemoveAll(dbFileName)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to remove existing database files")
		}
	}
	// https://github.com/ethereum/go-ethereum/blob/master/ethdb/leveldb/leveldb.go#L98 options.
	// We had 0 for handles and cache params before, so set 0s for all of them. Filter opt is the same.
	options := &opt.Options{
		OpenFilesCacheCapacity: 0,
		BlockCacheCapacity:     0,
		WriteBuffer:            0,
		Filter:                 filter.NewBloomFilter(10),
	}
	if storage.db, err = leveldb.OpenFile(dbFileName, options); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new database")
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *leveldb.DB {
	return storage.db
}

// Dump extracts information from block and index them into lvdb for explorer.
func (storage *Storage) Dump(block *types.Block, height uint64) {
	storage.lock.Lock()
	defer storage.lock.Unlock()
	if block == nil {
		return
	}

	// Skip dump for redundant blocks with lower block number than the checkpoint block number
	blockCheckpoint := GetCheckpointKey(block.Header().Number())
	if _, err := storage.GetDB().Get([]byte(blockCheckpoint), nil); err == nil {
		return
	}

	// Store txs
	for _, tx := range block.Transactions() {
		explorerTransaction, err := GetTransaction(tx, block)
		if err != nil {
			utils.Logger().Error().Err(err).Str("txHash", tx.Hash().String()).
				Msg("[Explorer Storage] Failed to get GetTransaction mapping")
			continue
		}
		storage.UpdateTxAddress(explorerTransaction, tx)
	}
	// Store staking txns
	for _, tx := range block.StakingTransactions() {
		explorerTransaction, err := GetStakingTransaction(tx, block)
		if err != nil {
			utils.Logger().Error().Err(err).Str("txHash", tx.Hash().String()).
				Msg("[Explorer Storage] Failed to get StakingTransaction mapping")
			continue
		}
		storage.UpdateStakingTxAddress(explorerTransaction, tx)
	}

	// save checkpoint of block dumped
	storage.GetDB().Put([]byte(blockCheckpoint), []byte{}, nil)
}

// UpdateTxAddress ...
func (storage *Storage) UpdateTxAddress(explorerTransaction *Transaction, tx *types.Transaction) {
	explorerTransaction.Type = Received
	if explorerTransaction.To != "" {
		storage.UpdateTxAddressStorage(explorerTransaction.To, explorerTransaction, tx)
	}
	explorerTransaction.Type = Sent
	storage.UpdateTxAddressStorage(explorerTransaction.From, explorerTransaction, tx)
}

// UpdateStakingTxAddress ...
func (storage *Storage) UpdateStakingTxAddress(explorerTransaction *StakingTransaction, tx *staking.StakingTransaction) {
	explorerTransaction.Type = Received
	if explorerTransaction.To != "" {
		storage.UpdateStakingTxAddressStorage(explorerTransaction.To, explorerTransaction, tx)
	}
	explorerTransaction.Type = Sent
	storage.UpdateStakingTxAddressStorage(explorerTransaction.From, explorerTransaction, tx)
}

// UpdateTxAddressStorage updates specific addr tx Address.
func (storage *Storage) UpdateTxAddressStorage(addr string, explorerTransaction *Transaction, tx *types.Transaction) {
	var address Address
	key := GetAddressKey(addr)
	if data, err := storage.GetDB().Get([]byte(key), nil); err == nil {
		if err = rlp.DecodeBytes(data, &address); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed due to error")
		}
	}
	address.ID = addr
	address.TXs = append(address.TXs, explorerTransaction)
	encoded, err := rlp.EncodeToBytes(address)
	if err == nil {
		storage.GetDB().Put([]byte(key), encoded, nil)
	} else {
		utils.Logger().Error().Err(err).Msg("cannot encode address")
	}
}

// UpdateStakingTxAddressStorage updates specific addr staking tx Address.
func (storage *Storage) UpdateStakingTxAddressStorage(addr string, explorerTransaction *StakingTransaction, tx *staking.StakingTransaction) {
	var address Address
	key := GetAddressKey(addr)
	if data, err := storage.GetDB().Get([]byte(key), nil); err == nil {
		if err = rlp.DecodeBytes(data, &address); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed due to error")
		}
	}
	address.ID = addr
	address.StakingTXs = append(address.StakingTXs, explorerTransaction)
	encoded, err := rlp.EncodeToBytes(address)
	if err == nil {
		storage.GetDB().Put([]byte(key), encoded, nil)
	} else {
		utils.Logger().Error().Err(err).Msg("cannot encode address")
	}
}

// GetAddresses returns size of addresses from address with prefix.
func (storage *Storage) GetAddresses(size int, prefix string) ([]string, error) {
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
