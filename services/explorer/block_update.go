package explorer

import (
	"fmt"
	"os"
	"sync"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/db"
)

var blockUpdate *BlockUpdate
var once sync.Once

// BlockUpdate dump the block info into leveldb.
type BlockUpdate struct {
	db *db.LDBDatabase
}

// GetInstance returns attack model by using singleton pattern.
func GetInstance() *BlockUpdate {
	once.Do(func() {
		blockUpdate = &BlockUpdate{}
	})
	return blockUpdate
}

// Init initializes the block update.
func (blockUpdate *BlockUpdate) Init(ip, port string) {
	dbFileName := "/tmp/explorer_db_" + ip + "_" + port
	var err = os.RemoveAll(dbFileName)
	if err != nil {
		fmt.Println(err.Error())
	}
	if blockUpdate.db, err = db.NewLDBDatabase(dbFileName, 0, 0); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

// Dump extracts information from block and index them into lvdb for explorer.
func (blockUpdate *BlockUpdate) Dump(block *types.Block, height int) {
}
