package explorer

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/db"
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
func (storage *Storage) Dump(accountBlock []byte, height uint32) {
	fmt.Println("Dumping block ", height)
	if accountBlock == nil {
		return
	}
	// Update block height.
	storage.db.Put([]byte(BlockHeightKey), []byte(strconv.Itoa(int(height))))

	// Store block.
	block := new(types.Block)
	rlp.DecodeBytes(accountBlock, block)
	storage.db.Put([]byte(GetBlockKey(int(height))), accountBlock)

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

		explorerTransaction := Transaction{
			ID:        tx.Hash().Hex(),
			Timestamp: strconv.Itoa(int(block.Time().Int64() * 1000)),
			From:      tx.To().Hex(),
			To:        tx.To().Hex(),
			Value:     strconv.Itoa(int(tx.Value().Int64())),
			Bytes:     strconv.Itoa(int(tx.Size())),
		}

		storage.UpdateTXStorage(explorerTransaction, tx)
		storage.UpdateAddressStorage(explorerTransaction, tx)
	}
}

// UpdateTXStorage ...
func (storage *Storage) UpdateTXStorage(explorerTransaction Transaction, tx *types.Transaction) {
	if data, err := rlp.EncodeToBytes(explorerTransaction); err == nil {
		key := GetTXKey(tx.Hash().Hex())
		storage.db.Put([]byte(key), data)
	} else {
		Log.Error("EncodeRLP transaction error")
	}
}

// UpdateAddressStorage ...
func (storage *Storage) UpdateAddressStorage(explorerTransaction Transaction, tx *types.Transaction) {
	toAddress := tx.To().Hex()
	key := GetAddressKey(toAddress)

	var address Address
	if data, err := storage.db.Get([]byte(key)); err == nil {
		err = rlp.DecodeBytes(data, address)
		if err == nil {
			address.Balance.Add(address.Balance, tx.Value())
			txCount, _ := strconv.Atoi(address.TXCount)
			address.TXCount = strconv.Itoa(txCount + 1)
		}
	} else {
		address.Balance = tx.Value()
		address.TXCount = "1"
	}
	address.ID = toAddress
	address.TXs = append(address.TXs, explorerTransaction)
	if encoded, err := rlp.EncodeToBytes(address); err == nil {
		storage.db.Put([]byte(key), encoded)
	} else {
		Log.Error("Can not encode address account.")
	}
}
