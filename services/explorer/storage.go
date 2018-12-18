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
		Timestamp: strconv.Itoa(int(block.Time().Int64() * 1000)),
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

		storage.UpdateTxStorage(explorerTransaction, tx)
		storage.UpdateAddressStorage(explorerTransaction, tx)
	}
}

// UpdateTxStorage ...
func (storage *Storage) UpdateTxStorage(explorerTransaction Transaction, tx *types.Transaction) {
	if data, err := rlp.EncodeToBytes(explorerTransaction); err == nil {
		key := GetTXKey(tx.Hash().Hex())
		storage.db.Put([]byte(key), data)
	} else {
		fmt.Println("EncodeRLP transaction error")
		os.Exit(1)
	}
}

// UpdateAddressStorage ...
func (storage *Storage) UpdateAddressStorage(explorerTransaction Transaction, tx *types.Transaction) {
	toAddress := tx.To().Hex()
	key := GetAddressKey(toAddress)

	fmt.Println("dumping address", toAddress, key)

	var addressAccount Address
	if data, err := storage.db.Get([]byte(key)); err == nil {
		fmt.Println("the key existed")
		err = rlp.DecodeBytes(data, addressAccount)
		if err == nil {
			addressAccount.Balance.Add(addressAccount.Balance, tx.Value())
			txCount, _ := strconv.Atoi(addressAccount.TXCount)
			addressAccount.TXCount = strconv.Itoa(txCount + 1)
		}
	} else {
		fmt.Println("the key not existed")
		addressAccount.Balance = tx.Value()
		addressAccount.TXCount = "1"
	}
	addressAccount.ID = toAddress
	addressAccount.TXs = append(addressAccount.TXs, explorerTransaction)
	fmt.Println("trying to encode it")
	if encoded, err := rlp.EncodeToBytes(addressAccount); err == nil {
		fmt.Println("store addressAccount with length ", len(encoded))
		storage.db.Put([]byte(key), encoded)
	} else {
		fmt.Println("err when encoding ", err)
	}
}
