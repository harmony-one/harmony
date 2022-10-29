package state

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

func generateTestData(base int64) map[common.Hash]common.Hash {
	data := make(map[common.Hash]common.Hash)
	for i := int64(0); i < 1000; i++ {
		key := common.BigToHash(big.NewInt(i))
		val := common.BigToHash(big.NewInt(i + base))
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

func compare(db1, db2 ethdb.Database) bool {
	it1 := db1.NewIterator()
	it2 := db2.NewIterator()
	for it1.Next() {
		if !it2.Next() {
			return false
		}
		key1, val1 := it1.Key(), it1.Value()
		key2, val2 := it2.Key(), it2.Value()
		if !bytes.Equal(key1, key2) {
			return false
		}
		if !bytes.Equal(val1, val2) {
			return false
		}
	}
	return !it2.Next()
}

func newState(root common.Hash, db ethdb.Database) *stateTest {
	sdb, _ := New(root, NewDatabase(db))
	return &stateTest{db: db, state: sdb}
}

func TestDiffAndPrune2(t *testing.T) {
	db1State1 := newStateTest()
	writeTestData(db1State1, generateTestData(0))
	db1State2 := newState(db1State1.state.trie.Hash(), db1State1.db)
	writeTestData(db1State2, generateTestData(10000))
	db2State := newStateTest()
	writeTestData(db2State, generateTestData(10000))
	prune := func(old *stateTest, new *stateTest) {
		batch := old.db.NewBatch()
		DiffAndPrune(old.state, new.state, batch)
		batch.Write()
	}
	prune(db1State1, db1State2)
	if !compare(db1State2.db, db2State.db) {
		t.Error("db state not equal")
	}

}
