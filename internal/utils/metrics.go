package utils


import (
	"fmt"
	"strconv"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/ctxerror"
)

// Constants for storage.
const (
	CurrentConnectionsNumberKey = "cnk"
	ConnectionsNumberPrefix = "cnp"
	BalancePrefix = "bp"
	BlocksSuccessPrefix = "bsp"
	BlocksProcessedPrefix = "bpp"
	TransactionsSuccessPrefix = "tsp"
	TrnascationsProcessedPrefix = "tpp"
	NodeCPUPrefix = "ncp"
	NodeTrafficPrefix = "ntp"
	LeaderTimePrefix = "ltp"
	ConsensusFramePrefix = "cfp"
)

// BToMb ...
func BToMb(b uint64) uint64 {
	return b / 1024 / 1024
}


// GetCurrentConnectionsNumberKey ...
func GetCurrentConnectionsNumberKey(currentTime int) string {
	return fmt.Sprintf("%s_%d", CurrentConnectionsNumberKey, currentTime)
}


// GetConnectionsNumberKey ...
func GetConnectionsNumberKey(moment int) string {
	return fmt.Sprintf("%s_%d", ConnectionsNumberPrefix, moment)
}


var storage *MetricsStorage
var onceMetrics sync.Once

// Storage dump the block info into leveldb.
type MetricsStorage struct {
	db *ethdb.LDBDatabase
}

// GetStorageInstance returns attack model by using singleton pattern.
func GetMetricsStorageInstance(ip, port string, remove bool) *MetricsStorage {
	onceMetrics.Do(func() {
		storage = &MetricsStorage{}
		storage.Init(ip, port, remove)
	})
	return storage
}

// Init initializes connections number storage.
func (storage *MetricsStorage) Init(ip, port string, remove bool) {
	dbFileName := "/.hmy/db-metrics-" + ip + "-" + port
	var err error
	if remove {
		var err = os.RemoveAll(dbFileName)
		if err != nil {
			Logger().Error().Err(err).Msg("Failed to remove existing database files")
		}
	}
	if storage.db, err = ethdb.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		Logger().Error().Err(err).Msg("Failed to create new database")
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *MetricsStorage) GetDB() *ethdb.LDBDatabase {
	return storage.db
}

// Dump get time and current connections number and index them into lvdb for monitoring service.
func (storage *MetricsStorage) Dump(connectionsNumber int, currentTime int) {
	Logger().Info().Int("Unix Time", currentTime).Msg("Store current connections number")

	batch := storage.db.NewBatch()
	// Update current connections number.
	if err := batch.Put([]byte(CurrentConnectionsNumberKey), []byte(strconv.Itoa(connectionsNumber))); err != nil {
		Logger().Warn().Err(err).Msg("cannot batch current connections number")
	}

	// Store connections number for current time.
	connectionsNumberData, err := rlp.EncodeToBytes(connectionsNumber)
	if err == nil {
		if err := batch.Put([]byte(GetConnectionsNumberKey(currentTime)), connectionsNumberData); err != nil {
			Logger().Warn().Err(err).Msg("cannot batch connections number")
		}
	} else {
		Logger().Error().Err(err).Msg("Failed to serialize connections number")
	}

	if err := batch.Write(); err != nil {
		ctxerror.Warn(GetLogger(), err, "cannot write batch")
	}
}

// ReadConnectionsNumberFromDB returns a list of connections numbers to server connections number end-point.
func (storage *MetricsStorage) ReadConnectionsNumbersFromDB(since, until int) []int {
	connectionsNumbers := make([]int, 0)
	for i := since; i <= until; i++ {
		key := GetConnectionsNumberKey(i)
		data, err := storage.db.Get([]byte(key))
		if err != nil {
			continue
		}
		connectionsNumber := 0
		if rlp.DecodeBytes(data, connectionsNumber) != nil {
			Logger().Error().Msg("Error on getting from db")
			os.Exit(1)
		}
		connectionsNumbers = append(connectionsNumbers, connectionsNumber)
	}
	return connectionsNumbers
}
