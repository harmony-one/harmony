package state

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

const (
	DataSize int64 = 1000
	DataMod1 int64 = 5
	DataMod2 int64 = 19
)

func generateTestData(dataSize, mod int64) map[common.Hash]common.Hash {
	data := make(map[common.Hash]common.Hash)
	for i := int64(0); i < dataSize; i++ {
		key := common.BigToHash(big.NewInt(i))
		val := common.BigToHash(big.NewInt(i % mod))
		data[key] = val
	}
	return data
}

func writeTestData(stateDB *DB, data map[common.Hash]common.Hash) {
	obj := stateDB.GetOrNewStateObject(common.BytesToAddress([]byte{0x01}))
	obj.SetNonce(uint64(len(data)))
	for k, v := range data {
		stateDB.SetState(obj.address, k, v)
	}
	root, _ := stateDB.Commit(true)
	stateDB.db.TrieDB().Commit(root, false)
}

func equal(db1, db2 ethdb.Database) bool {
	it1 := db1.NewIterator()
	it2 := db2.NewIterator()
	for it1.Next() {
		if !it2.Next() {
			return false
		}
		if !bytes.Equal(it1.Key(), it2.Key()) {
			return false
		}
		if !bytes.Equal(it1.Value(), it2.Value()) {
			return false
		}
	}
	return !it2.Next()
}

func newStateDB(root common.Hash, db ethdb.Database) *DB {
	sdb, err := New(root, NewDatabase(db))
	if err != nil {
		panic("invalid state root")
	}
	return sdb
}

func writeAndPrune(db ethdb.Database, root common.Hash, data map[common.Hash]common.Hash) common.Hash {
	oldStateDB := newStateDB(root, db)
	newStateDB := oldStateDB.Copy()
	writeTestData(newStateDB, data)
	batch := db.NewBatch()
	DiffAndPruneSync(oldStateDB, newStateDB, batch, nil)
	batch.Write()
	return newStateDB.trie.Hash()
}

func TestDiffAndPrune(t *testing.T) {
	writeAndCheck := func(db ethdb.Database, root common.Hash, data map[common.Hash]common.Hash) common.Hash {
		newRoot := writeAndPrune(db, root, data)
		pureDB := rawdb.NewMemoryDatabase()
		pureDBRoot := writeAndPrune(pureDB, common.Hash{}, data)
		if newRoot != pureDBRoot {
			t.Fatal("root hash not equal")
		}
		if !equal(db, pureDB) {
			t.Fatal("databases are not equal")
		}
		return newRoot
	}

	dataset1 := generateTestData(DataSize, DataMod1)
	dataset2 := generateTestData(DataSize, DataMod2)
	datasets := []map[common.Hash]common.Hash{
		dataset1,
		dataset2,
		dataset1,
	}
	db := rawdb.NewMemoryDatabase()
	root := common.Hash{}
	for _, dataset := range datasets {
		root = writeAndCheck(db, root, dataset)
	}
}

func writeAndPruneAbort(db ethdb.Database, root common.Hash, data map[common.Hash]common.Hash) (error, common.Hash) {
	oldStateDB := newStateDB(root, db)
	newStateDB := oldStateDB.Copy()
	writeTestData(newStateDB, data)
	batch := db.NewBatch()
	abortCh := make(chan struct{}, 1)
	abortCh <- struct{}{}
	if _, err := DiffAndPruneSync(oldStateDB, newStateDB, batch, abortCh); err != nil {
		return err, root
	}
	batch.Write()
	return nil, newStateDB.trie.Hash()
}

func TestDiffAndPruneAbort(t *testing.T) {
	dataset1 := generateTestData(DataSize, DataMod1)
	dataset2 := generateTestData(DataSize, DataMod2)
	datasets := []map[common.Hash]common.Hash{
		dataset1,
		dataset2,
		dataset1,
	}
	db := rawdb.NewMemoryDatabase()
	root := common.Hash{}
	for _, dataset := range datasets {
		err, root := writeAndPruneAbort(db, root, dataset)
		if err == nil || (root != common.Hash{}) {
			t.Fatal("pruning should be aborted")
		}
	}
}
