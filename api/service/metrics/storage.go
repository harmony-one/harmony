package metrics

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for storage.
const (
	BalancePrefix           = "bap"
	BlockHeightPrefix       = "bhp"
	BlocksPrefix            = "bp"
	BlockRewardPrefix       = "brp"
	ConnectionsNumberPrefix = "cnp"
	ConsensusTimePrefix     = "ltp"
	IsLeaderPrefix          = "ilp"
	TxPoolPrefix            = "tpp"
)

// GetKey returns key by prefix and pushed time momemnt.
func GetKey(prefix string, moment int64) string {
	return fmt.Sprintf("%s_%d", prefix, moment)
}

// storage instance
var storage *Storage
var onceMetrics sync.Once

// Storage storage dump the block info into leveldb.
type Storage struct {
	db *ethdb.LDBDatabase
}

// GetStorageInstance returns attack model by using singleton pattern.
func GetStorageInstance(ip, port string, remove bool) *Storage {
	onceMetrics.Do(func() {
		storage = &Storage{}
		storage.Init(ip, port, remove)
	})
	return storage
}

// Init initializes storage.
func (storage *Storage) Init(ip, port string, remove bool) {
	dbFileName := "/tmp/db_metrics_" + ip + "_" + port
	var err error
	if remove {
		var err = os.RemoveAll(dbFileName)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to remove existing database files.")
		}
	}
	if storage.db, err = ethdb.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new database.")
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *ethdb.LDBDatabase {
	return storage.db
}

// Dump data into lvdb by value and prefix.
func (storage *Storage) Dump(value interface{}, prefix string) error {
	currentTime := time.Now().UnixNano()
	utils.Logger().Info().Msgf("Store %s %v at time %d", prefix, value, currentTime)
	batch := storage.db.NewBatch()
	// Update database.
	if err := batch.Put([]byte(GetKey(prefix, currentTime)), []byte(fmt.Sprintf("%v", value.(interface{})))); err != nil {
		utils.Logger().Warn().Err(err).Msgf("Cannot batch %s.", prefix)
		return err
	}
	if err := batch.Write(); err != nil {
		utils.Logger().Warn().Err(err).Msg("Cannot write batch.")
		return err
	}
	return nil
}

// Read returns data list of a particular metric by since, until, prefix, interface.
func (storage *Storage) Read(since, until int64, prefix string, varType interface{}) []interface{} {
	dataList := make([]interface{}, 0)
	for i := since; i <= until; i++ {
		data, err := storage.db.Get([]byte(GetKey(prefix, i)))
		if err != nil {
			continue
		}
		decodedData := varType
		if rlp.DecodeBytes(data, decodedData) != nil {
			utils.Logger().Error().Msg("Error on getting data from db.")
			os.Exit(1)
		}
		dataList = append(dataList, decodedData)
	}
	return dataList
}
