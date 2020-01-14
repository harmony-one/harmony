package explorer

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for storage.
const (
	BlockHeightKey  = "bh"
	BlockInfoPrefix = "bi"
	BlockPrefix     = "b"
	TXPrefix        = "tx"
	AddressPrefix   = "ad"
)

// GetBlockInfoKey ...
func GetBlockInfoKey(id int) string {
	return fmt.Sprintf("%s_%d", BlockInfoPrefix, id)
}

// GetAddressKey ...
func GetAddressKey(address string) string {
	return fmt.Sprintf("%s_%s", AddressPrefix, address)
}

// GetBlockKey ...
func GetBlockKey(id int) string {
	return fmt.Sprintf("%s_%d", BlockPrefix, id)
}

// GetTXKey ...
func GetTXKey(hash string) string {
	return fmt.Sprintf("%s_%s", TXPrefix, hash)
}

var storage *Storage
var once sync.Once

// Storage dump the block info into leveldb.
type Storage struct {
	db *ethdb.LDBDatabase
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
	dbFileName := "/tmp/explorer_storage_" + ip + "_" + port
	var err error
	if remove {
		var err = os.RemoveAll(dbFileName)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to remove existing database files")
		}
	}
	if storage.db, err = ethdb.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new database")
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *ethdb.LDBDatabase {
	return storage.db
}

// Dump extracts information from block and index them into lvdb for explorer.
func (storage *Storage) Dump(block *types.Block, height uint64) {
	//utils.Logger().Debug().Uint64("block height", height).Msg("Dumping block")
	if block == nil {
		return
	}

	batch := storage.db.NewBatch()
	// Update block height.
	if err := batch.Put([]byte(BlockHeightKey), []byte(strconv.Itoa(int(height)))); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot batch block height")
	}

	// Store block.
	blockData, err := rlp.EncodeToBytes(block)
	if err == nil {
		if err := batch.Put([]byte(GetBlockKey(int(height))), blockData); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot batch block data")
		}
	} else {
		utils.Logger().Error().Err(err).Msg("Failed to serialize block")
	}

	// Store txs
	for _, tx := range block.Transactions() {
		explorerTransaction := GetTransaction(tx, block)
		storage.UpdateTXStorage(batch, explorerTransaction, tx)
		storage.UpdateAddress(batch, explorerTransaction, tx)
	}
	if err := batch.Write(); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot write batch")
	}
}

// UpdateTXStorage ...
func (storage *Storage) UpdateTXStorage(batch ethdb.Batch, explorerTransaction *Transaction, tx *types.Transaction) {
	if data, err := rlp.EncodeToBytes(explorerTransaction); err == nil {
		key := GetTXKey(tx.Hash().Hex())
		if err := batch.Put([]byte(key), data); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot batch TX")
		}
	} else {
		utils.Logger().Error().Msg("EncodeRLP transaction error")
	}
}

// UpdateAddress ...
// TODO: deprecate this logic
func (storage *Storage) UpdateAddress(batch ethdb.Batch, explorerTransaction *Transaction, tx *types.Transaction) {
	explorerTransaction.Type = Received
	if explorerTransaction.To != "" {
		storage.UpdateAddressStorage(batch, explorerTransaction.To, explorerTransaction, tx)
	}
	explorerTransaction.Type = Sent
	storage.UpdateAddressStorage(batch, explorerTransaction.From, explorerTransaction, tx)
}

// UpdateAddressStorage updates specific addr Address.
// TODO: deprecate this logic
func (storage *Storage) UpdateAddressStorage(batch ethdb.Batch, addr string, explorerTransaction *Transaction, tx *types.Transaction) {
	key := GetAddressKey(addr)

	var address Address
	if data, err := storage.db.Get([]byte(key)); err == nil {
		if err = rlp.DecodeBytes(data, &address); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed due to error")
		}
	}
	address.ID = addr
	address.TXs = append(address.TXs, explorerTransaction)
	encoded, err := rlp.EncodeToBytes(address)
	if err == nil {
		if err := batch.Put([]byte(key), encoded); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot batch address")
		}
	} else {
		utils.Logger().Error().Err(err).Msg("cannot encode address")
	}
}
