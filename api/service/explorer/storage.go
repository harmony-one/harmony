package explorer

import (
	"fmt"
	"math/big"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Constants for storage.
const (
	AddressPrefix = "ad"
	PrefixLen     = 3
)

// GetAddressKey ...
func GetAddressKey(address string) string {
	return fmt.Sprintf("%s_%s", AddressPrefix, address)
}

var storage *Storage
var once sync.Once

// Storage dump the block info into leveldb.
type Storage struct {
	db *leveldb.DB
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
	if block == nil {
		return
	}

	batch := new(leveldb.Batch)
	// Store txs
	for _, tx := range block.Transactions() {
		explorerTransaction := GetTransaction(tx, block)
		storage.UpdateAddress(batch, explorerTransaction, tx)
	}
	// Store cross shard txs
	for _, proof := range block.IncomingReceipts() {
		for _, receipt := range proof.Receipts {
			var err error
			if receipt.ShardID == receipt.ToShardID {
				continue
			}
			to := ""
			if receipt.To != nil {
				if to, err = common.AddressToBech32(*receipt.To); err != nil {
					continue
				}
			}
			from := ""
			if from, err = common.AddressToBech32(receipt.From); err != nil {
				continue
			}
			explorerTransaction := &Transaction{
				ID:        receipt.TxHash.String(),
				Timestamp: "",
				From:      from,
				To:        to,
				Value:     receipt.Amount,
				Bytes:     "",
				Data:      "",
				GasFee:    big.NewInt(0),
				FromShard: receipt.ShardID,
				ToShard:   receipt.ToShardID,
				Type:      Cross,
			}
			storage.UpdateAddress(batch, explorerTransaction, nil)
		}
	}
	if err := storage.db.Write(batch, nil); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot write batch")
	}
}

// UpdateAddress ...
func (storage *Storage) UpdateAddress(batch *leveldb.Batch, explorerTransaction *Transaction, tx *types.Transaction) {
	if explorerTransaction.Type != Cross {
		explorerTransaction.Type = Received
	}
	if explorerTransaction.To != "" {
		storage.UpdateAddressStorage(batch, explorerTransaction.To, explorerTransaction, tx)
	}
	if explorerTransaction.Type != Cross {
		explorerTransaction.Type = Sent
	}
	storage.UpdateAddressStorage(batch, explorerTransaction.From, explorerTransaction, tx)
}

// UpdateAddressStorage updates specific addr Address.
func (storage *Storage) UpdateAddressStorage(batch *leveldb.Batch, addr string, explorerTransaction *Transaction, tx *types.Transaction) {
	var address Address
	key := GetAddressKey(addr)
	if data, err := storage.db.Get([]byte(key), nil); err == nil {
		if err = rlp.DecodeBytes(data, &address); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed due to error")
		}
	}
	address.ID = addr
	address.TXs = append(address.TXs, explorerTransaction)
	encoded, err := rlp.EncodeToBytes(address)
	if err == nil {
		batch.Put([]byte(key), encoded)
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
