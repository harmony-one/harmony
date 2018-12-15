package explorer

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/db"
)

// Constants for storage.
const (
	BlockHeightKey  = "bh"
	BlockInfoPrefix = "bi"
	BlockPrefix     = "b"
	TXPrefix        = "tx"
)

// GetBlockInfoKey ...
func GetBlockInfoKey(id int) string {
	return fmt.Sprintf("%s_%d", BlockInfoPrefix, id)
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
			fmt.Println(err.Error())
		}
	}
	if storage.db, err = db.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
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
		Timestamp: string(block.Time().Int64()),
		TXCount:   string(block.Transactions().Len()),
		Size:      block.Size().String(),
	}

	if data, err := rlp.EncodeToBytes(blockInfo); err == nil {
		key := GetBlockInfoKey(int(height))
		fmt.Println("store blockinfo with key ", key)
		fmt.Println("data to store ", data)
		storage.db.Put([]byte(key), data)
	} else {
		fmt.Println("EncodeRLP blockInfo error")
		os.Exit(1)
	}

	// Store txs
	fmt.Println("# of txs ", len(block.Transactions()))
	for _, tx := range block.Transactions() {
		if data, err := rlp.EncodeToBytes(tx); err == nil {
			key := GetTXKey(tx.Hash().Hex())
			storage.db.Put([]byte(key), data)
		} else {
			fmt.Println("EncodeRLP transaction error")
			os.Exit(1)
		}
	}
}
