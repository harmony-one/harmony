package metrics

import (
	"fmt"
	"strconv"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

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
var onceMetrics sync.Once

// Storage dump the block info into leveldb.
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

// Init initializes connections number storage.
func (Storage *Storage) Init(ip, port string, remove bool) {
	dbFileName := "/.hmy/db-metrics-" + ip + "-" + port
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
func (Storage *Storage) GetDB() *ethdb.LDBDatabase {
	return Storage.db
}

// Dump gets time and current connections number and index them into lvdb for monitoring service.
func (Storage *Storage) Dump(connectionsNumber int, currentTime int) {
	utils.Logger().Info().Int("Unix Time", currentTime).Msg("Store current connections number")

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


// ReadConnectionsNumbersFromDB returns a list of connections numbers to server connections number end-point.
func (Storage *Storage) ReadConnectionsNumbersFromDB(since, until int) []int {
	connectionsNumbers := make([]int, 0)
	for i := since; i <= until; i++ {
		key := GetConnectionsNumberKey(i)
		data, err := Storage.db.Get([]byte(key))
		if err != nil {
			continue
		}
		connectionsNumber := 0
		if rlp.DecodeBytes(data, connectionsNumber) != nil {
			utils.Logger().Error().Msg("Error on getting from db")
			os.Exit(1)
		}
		connectionsNumbers = append(connectionsNumbers, connectionsNumber)
	}
	return connectionsNumbers
}
