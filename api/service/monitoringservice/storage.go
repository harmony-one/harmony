package monitoringservice

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
)

// Constants for storage.
const (
	CurrentConnectionsNumberKey = "cnk"
	ConnectionsNumberPrefix = "cnp"
)

// GetCurrentConnectionsNumberKey ...
func GetCurrentConnectionsNumberKey(currentTime int) string {
	return fmt.Sprintf("%s_%d", CurrentConnectionsNumberKey, currentTime)
}


// GetConnectionsNumberKey ...
func GetConnectionsNumberKey(moment int) string {
	return fmt.Sprintf("%s_%d", ConnectionsNumberPrefix, moment)
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

// Init initializes connections number storage.
func (storage *Storage) Init(ip, port string, remove bool) {
	dbFileName := "/tmp/monitoring_service_storage_" + ip + "_" + port
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

// Dump get time and current connections number and index them into lvdb for monitoring service.
func (storage *Storage) Dump(connectionsNumber int, currentTime int) {
	utils.Logger().Info().Int("Unix Time", currentTime).Msg("Store current connections number")
	if block == nil {
		return
	}

	batch := storage.db.NewBatch()
	// Update current connections number.
	if err := batch.Put([]byte(CurrentConnectionsNumberKey), []byte(strconv.Itoa(connectionsNumber))); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot batch current connections number")
	}

	// Store connections number for current time.
	connectionsNumberData, err := rlp.EncodeToBytes(connectionsNumber)
	if err == nil {
		if err := batch.Put([]byte(GetConnectionsNumberKey(currentTime)), connectionsNumberData); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot batch connections number")
		}
	} else {
		utils.Logger().Error().Err(err).Msg("Failed to serialize connections number")
	}

	if err := batch.Write(); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "cannot write batch")
	}
}