package explorer

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
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
			utils.GetLogInstance().Error(err.Error())
		}
	}
	if storage.db, err = ethdb.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		utils.GetLogInstance().Error(err.Error())
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *ethdb.LDBDatabase {
	return storage.db
}

// Dump extracts information from block and index them into lvdb for explorer.
func (storage *Storage) Dump(block *types.Block, height uint32) {
	utils.GetLogInstance().Info("Dumping block ", "block height", height)
	if block == nil {
		return
	}

	batch := storage.db.NewBatch()
	// Update block height.
	batch.Put([]byte(BlockHeightKey), []byte(strconv.Itoa(int(height))))

	// Store block.
	blockData, err := rlp.EncodeToBytes(block)
	if err == nil {
		batch.Put([]byte(GetBlockKey(int(height))), blockData)
	} else {
		utils.GetLogInstance().Debug("Failed to serialize block ", "error", err)
	}

	// Debug code: will clean up after relaunching.
	// keys := map[string]int{}
	// for _, tx := range block.Transactions() {
	// 	explorerTransaction := GetTransaction(tx, block)
	// 	if explorerTransaction == nil {
	// 		continue
	// 	}
	// 	utils.GetLogInstance().Info("minh tx info before adding", "from", explorerTransaction.From, "to", explorerTransaction.To)
	// 	key := GetAddressKey(explorerTransaction.To)
	// 	var address Address
	// 	if data, err := storage.db.Get([]byte(key)); err == nil {
	// 		err = rlp.DecodeBytes(data, &address)
	// 		if err == nil {
	// 			utils.GetLogInstance().Info("existed for", "key", key, "address", address)
	// 			keys[key] = len(address.TXs)
	// 		} else {
	// 			utils.GetLogInstance().Error("error when decode bytes for address", "address", address, "err", err)
	// 		}
	// 	} else {
	// 		utils.GetLogInstance().Info("not existed for", "key", key)
	// 	}
	// 	key = GetAddressKey(explorerTransaction.From)
	// 	if data, err := storage.db.Get([]byte(key)); err == nil {
	// 		err = rlp.DecodeBytes(data, &address)
	// 		if err == nil {
	// 			utils.GetLogInstance().Info("existed for", "key", key, "address", address)
	// 			keys[key] = len(address.TXs)
	// 		} else {
	// 			utils.GetLogInstance().Error("error when decode bytes for address", "address", address, "err", err)
	// 		}
	// 	} else {
	// 		utils.GetLogInstance().Info("not existed for", "key", key)
	// 	}
	// }

	// Store txs
	for _, tx := range block.Transactions() {
		if tx.To() == nil {
			continue
		}

		explorerTransaction := GetTransaction(tx, block)
		storage.UpdateTXStorage(batch, explorerTransaction, tx)
		storage.UpdateAddress(batch, explorerTransaction, tx)
	}
	batch.Write()

	// Debug code: will clean up after relaunching.
	// utils.GetLogInstance().Info("minh test", "inserted", []byte(strconv.Itoa(int(height))))
	// res, _ := storage.db.Get([]byte(BlockHeightKey))
	// utils.GetLogInstance().Info("minh test", "after inserted", res)
	// for _, tx := range block.Transactions() {
	// 	explorerTransaction := GetTransaction(tx, block)
	// 	if explorerTransaction == nil {
	// 		continue
	// 	}
	// 	utils.GetLogInstance().Info("****minh tx info after adding***", "from", explorerTransaction.From, "to", explorerTransaction.To)

	// 	key := GetAddressKey(explorerTransaction.To)
	// 	var address Address
	// 	if data, err := storage.db.Get([]byte(key)); err == nil {
	// 		err = rlp.DecodeBytes(data, &address)
	// 		if err == nil {
	// 			utils.GetLogInstance().Info("existed for", "key", key, "address", address)
	// 			utils.GetLogInstance().Info("existed for", "key", key, "address", address, "before", keys[key], "after", len(address.TXs))
	// 		} else {
	// 			utils.GetLogInstance().Error("error when decode bytes for address", "address", address, "err", err)
	// 		}
	// 	} else {
	// 		utils.GetLogInstance().Info("not existed for", "key", key)
	// 	}

	// 	key = GetAddressKey(explorerTransaction.From)
	// 	if data, err := storage.db.Get([]byte(key)); err == nil {
	// 		err = rlp.DecodeBytes(data, &address)
	// 		if err == nil {
	// 			utils.GetLogInstance().Info("existed for", "key", key, "address", address)
	// 			utils.GetLogInstance().Info("existed for", "key", key, "address", address, "before", keys[key], "after", len(address.TXs))
	// 		} else {
	// 			utils.GetLogInstance().Error("error when decode bytes for address", "address", address, "err", err)
	// 		}
	// 	} else {
	// 		utils.GetLogInstance().Info("not existed for", "key", key)
	// 	}
	// }
}

// UpdateTXStorage ...
func (storage *Storage) UpdateTXStorage(batch ethdb.Batch, explorerTransaction *Transaction, tx *types.Transaction) {
	if data, err := rlp.EncodeToBytes(explorerTransaction); err == nil {
		key := GetTXKey(tx.Hash().Hex())
		batch.Put([]byte(key), data)
	} else {
		utils.GetLogInstance().Error("EncodeRLP transaction error")
	}
}

// UpdateAddress ...
func (storage *Storage) UpdateAddress(batch ethdb.Batch, explorerTransaction *Transaction, tx *types.Transaction) {
	storage.UpdateAddressStorage(batch, explorerTransaction.To, explorerTransaction, tx)
	storage.UpdateAddressStorage(batch, explorerTransaction.From, explorerTransaction, tx)
}

// UpdateAddressStorage updates specific adr address.
func (storage *Storage) UpdateAddressStorage(batch ethdb.Batch, adr string, explorerTransaction *Transaction, tx *types.Transaction) {
	key := GetAddressKey(adr)

	var address Address
	if data, err := storage.db.Get([]byte(key)); err == nil {
		err = rlp.DecodeBytes(data, &address)
		if err == nil {
			address.Balance.Add(address.Balance, tx.Value())
		} else {
			utils.GetLogInstance().Error("Failed to error", "err", err)
		}
	} else {
		address.Balance = tx.Value()
	}
	address.ID = adr
	// Debug code. clean it up after relaunching.
	// utils.GetLogInstance().Info("minh UpdateAddressStorage:", "address", adr, "length:", len(address.TXs))
	address.TXs = append(address.TXs, explorerTransaction)
	// utils.GetLogInstance().Info("minh UpdateAddressStorage after append:", "address", adr, "length:", len(address.TXs))
	if encoded, err := rlp.EncodeToBytes(address); err == nil {
		batch.Put([]byte(key), encoded)
	} else {
		utils.GetLogInstance().Error("Can not encode address account.")
	}
}
