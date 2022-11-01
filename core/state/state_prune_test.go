package state

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

const (
	DataMod1  int64 = 5
	DataSize1 int64 = 1000
	DataMod2  int64 = 19
	DataSize2 int64 = 1000
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

func writeTestData(s *stateTest, data map[common.Hash]common.Hash) {
	obj := s.state.GetOrNewStateObject(common.BytesToAddress([]byte{0x01}))
	obj.SetNonce(uint64(len(data)))
	for k, v := range data {
		s.state.SetState(obj.address, k, v)
	}
	root, _ := s.state.Commit(true)
	s.state.db.TrieDB().Commit(root, false)
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

func stateNew(root common.Hash, db ethdb.Database) *stateTest {
	sdb, _ := New(root, NewDatabase(db))
	return &stateTest{db: db, state: sdb}
}

func TestDiffAndPrune(t *testing.T) {
	db1State1 := newStateTest()
	writeTestData(db1State1, generateTestData(DataSize1, DataMod1))
	db1State2 := stateNew(db1State1.state.trie.Hash(), db1State1.db)
	writeTestData(db1State2, generateTestData(DataSize2, DataMod2))
	db2State := newStateTest()
	writeTestData(db2State, generateTestData(DataSize2, DataMod2))
	prune := func(old *stateTest, new *stateTest) {
		batch := old.db.NewBatch()
		DiffAndPrune(old.state, new.state, batch)
		batch.Write()
	}
	prune(db1State1, db1State2)
	if !equal(db1State2.db, db2State.db) {
		t.Error("db state not equal")
	}

}
