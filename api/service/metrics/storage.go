package metrics

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/rs/zerolog/log"
)

// Constants for storage.
const (
	ConnectionsNumberPrefix     = "cnp"
	BalancePrefix               = "bap"
	BlockHeightPrefix           = "bhp"
	BlocksPrefix                = "bp"
	ConsensusTimePrefix         = "ltp"
	BlockRewardPrefix           = "brp"
	TransactionsPrefix          = "tsp"
)

// GetKey returns key by name and pushed time momemnt.
func GetKey(name string, moment int) string {
	return fmt.Sprintf("%s_%d", name, moment)
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
	dbFileName := "/.hmy/db-metrics-" + ip + "-" + port
	var err error
	if remove {
		var err = os.RemoveAll(dbFileName)
		if err != nil {
			log.Error().Err(err).Msg("Failed to remove existing database files.")
		}
	}
	if storage.db, err = ethdb.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		log.Error().Err(err).Msg("Failed to create new database.")
	}
}

// GetDB returns the LDBDatabase of the storage.
func (storage *Storage) GetDB() *ethdb.LDBDatabase {
	return storage.db
}

// Dump data into lvdb.
func (storage *Storage) Dump(value interface{}, name string) error {
	currentTime := time.Now().Unix()
	log.Info().Msgf("Store %s %v at time %d", name, value, currentTime)

	batch := storage.db.NewBatch()
	// Update database.
	if err := batch.Put([]byte(GetKey(name, currentTime)), []byte(fmt.Sprintf("%v", value)); err != nil {
		log.Warn().Err(err).Msgf("Cannot batch %s.", name)
		return err
	}

	if err := batch.Write(); err != nil {
		log.Warn().Err(err).Msg("Cannot write batch.")
		return err
	}
	return nil
}

// Read returns data list of a particular metric by since, until, name readType.
func (storage *Storage) Read(since, until int, name string, readType reflect.Type) []interface{} {
	dataList := make([]readType, 0)
	for i := since; i <= until; i++ {
		data, err := storage.db.Get([]byte(GetKey(name, i)))
		if err != nil {
			continue
		}
		decodedData := 0
		if rlp.DecodeBytes(data, decodedData) != nil {
			log.Error().Msg("Error on getting data from db.")
			os.Exit(1)
		}
		dataList = append(dataList, decodedData)
	}
	return dataList
}
