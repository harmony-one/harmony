package explorer

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// Constants for storage.
const (
	BlockHeightKey  = "bh"
	BlockInfoPrefix = "bi"
	BlockPrefix     = "b"
	TXPrefix        = "tx"
	AccountPrefix   = "ad"
	CommitteePrefix = "cp"
)

// GetBlockInfoKey ...
func GetBlockInfoKey(id int) string {
	return fmt.Sprintf("%s_%d", BlockInfoPrefix, id)
}

// GetAccountKey ...
func GetAccountKey(address string) string {
	return fmt.Sprintf("%s_%s", AccountPrefix, address)
}

// GetBlockKey ...
func GetBlockKey(id int) string {
	return fmt.Sprintf("%s_%d", BlockPrefix, id)
}

// GetTXKey ...
func GetTXKey(hash string) string {
	return fmt.Sprintf("%s_%s", TXPrefix, hash)
}

// GetCommitteeKey ...
func GetCommitteeKey(shardID uint32, epoch uint64) string {
	return fmt.Sprintf("%s_%d_%d", CommitteePrefix, shardID, epoch)
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
	utils.Logger().Info().Uint64("block height", height).Msg("Dumping block")
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
		if tx.To() == nil {
			continue
		}

		explorerTransaction := GetTransaction(tx, block)
		storage.UpdateTXStorage(batch, explorerTransaction, tx)
		storage.UpdateAccount(batch, explorerTransaction, tx)
	}
	if err := batch.Write(); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "cannot write batch")
	}
}

// DumpCommittee commits validators for shardNum and epoch.
func (storage *Storage) DumpCommittee(shardID uint32, epoch uint64, committee shard.Committee) error {
	batch := storage.db.NewBatch()
	// Store committees.
	committeeData, err := rlp.EncodeToBytes(committee)
	if err != nil {
		return err
	}
	if err := batch.Put([]byte(GetCommitteeKey(shardID, epoch)), committeeData); err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return err
	}
	return nil
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

// UpdateAccount ...
func (storage *Storage) UpdateAccount(batch ethdb.Batch, explorerTransaction *Transaction, tx *types.Transaction) {
	explorerTransaction.Type = Received
	storage.UpdateAccountStorage(batch, explorerTransaction.To, explorerTransaction, tx)
	explorerTransaction.Type = Sent
	storage.UpdateAccountStorage(batch, explorerTransaction.From, explorerTransaction, tx)
}

// UpdateAccountStorage updates specific addr account.
func (storage *Storage) UpdateAccountStorage(batch ethdb.Batch, addr string, explorerTransaction *Transaction, tx *types.Transaction) {
	key := GetAccountKey(addr)

	var account Account
	if data, err := storage.db.Get([]byte(key)); err == nil {
		err = rlp.DecodeBytes(data, &account)
		if err == nil {
			account.Balance.Add(account.Balance, tx.Value())
		} else {
			utils.Logger().Error().Err(err).Msg("Failed to error")
		}
	} else {
		account.Balance = tx.Value()
	}
	account.ID = addr
	account.TXs = append(account.TXs, explorerTransaction)
	encoded, err := rlp.EncodeToBytes(account)
	if err == nil {
		if err := batch.Put([]byte(key), encoded); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot batch account")
		}
	} else {
		utils.Logger().Error().Err(err).Msg("cannot encode address account")
	}
}
