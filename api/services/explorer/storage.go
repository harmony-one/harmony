package explorer

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/db"
	"github.com/harmony-one/harmony/log"
)

// Constants for storage.
const (
	BlockHeightKey  = "bh"
	BlockInfoPrefix = "bi"
	BlockPrefix     = "b"
	TXPrefix        = "tx"
	AddressPrefix   = "ad"
)

// Log is the temporary log for storage.
var Log = log.New()

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
	db *db.LDBDatabase
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
			Log.Error(err.Error())
		}
	}
	if storage.db, err = db.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		Log.Error(err.Error())
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *db.LDBDatabase {
	return storage.db
}

// Dump extracts information from block and index them into lvdb for explorer.
func (storage *Storage) Dump(block *types.Block, height uint32) {
	Log.Info("Dumping block ", "block height", height)
	if block == nil {
		return
	}
	// Update block height.
	storage.db.Put([]byte(BlockHeightKey), []byte(strconv.Itoa(int(height))))

	// Store block.
	blockData, err := rlp.EncodeToBytes(block)
	if err == nil {
		storage.db.Put([]byte(GetBlockKey(int(height))), blockData)
	} else {
		Log.Debug("Failed to serialize block ", "error", err)
	}

	// Store block info.
	blockInfo := BlockInfo{
		ID:        block.Hash().Hex(),
		Height:    string(height),
		Timestamp: strconv.Itoa(int(block.Time().Int64() * 1000)),
		TXCount:   string(block.Transactions().Len()),
		Size:      block.Size().String(),
	}

	if data, err := rlp.EncodeToBytes(blockInfo); err == nil {
		key := GetBlockInfoKey(int(height))
		storage.db.Put([]byte(key), data)
	} else {
		Log.Error("EncodeRLP blockInfo error")
	}

	// Store txs
	for _, tx := range block.Transactions() {
		if tx.To() == nil {
			continue
		}

		explorerTransaction := GetTransaction(tx, block)

		storage.UpdateTXStorage(explorerTransaction, tx)
		storage.UpdateAddress(explorerTransaction, tx)
	}
}

// UpdateTXStorage ...
func (storage *Storage) UpdateTXStorage(explorerTransaction *Transaction, tx *types.Transaction) {
	if data, err := rlp.EncodeToBytes(explorerTransaction); err == nil {
		key := GetTXKey(tx.Hash().Hex())
		storage.db.Put([]byte(key), data)
	} else {
		Log.Error("EncodeRLP transaction error")
	}
}

// UpdateAddress ...
func (storage *Storage) UpdateAddress(explorerTransaction *Transaction, tx *types.Transaction) {
	storage.UpdateAddressStorage(explorerTransaction.To, explorerTransaction, tx)
	storage.UpdateAddressStorage(explorerTransaction.From, explorerTransaction, tx)
}

// UpdateAddressStorage updates specific adr address.
func (storage *Storage) UpdateAddressStorage(adr string, explorerTransaction *Transaction, tx *types.Transaction) {
	key := GetAddressKey(adr)

	var address Address
	if data, err := storage.db.Get([]byte(key)); err == nil {
		err = rlp.DecodeBytes(data, address)
		if err == nil {
			address.Balance.Add(address.Balance, tx.Value())
		}
	} else {
		address.Balance = tx.Value()
	}
	address.ID = adr
	address.TXs = append(address.TXs, explorerTransaction)
	if encoded, err := rlp.EncodeToBytes(address); err == nil {
		storage.db.Put([]byte(key), encoded)
	} else {
		Log.Error("Can not encode address account.")
	}
}
