package explorer

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"

	"github.com/syndtr/goleveldb/leveldb"
)

var prefixTestData = []prefixTestEntry{
	{[]byte("000001"), []byte("000001")},
	{[]byte("000002"), []byte("000001")},
	{[]byte("000003"), []byte("000001")},
	{[]byte("000004"), []byte("000001")},
	{[]byte("000005"), []byte("000001")},
	{[]byte("100001"), []byte("000001")},
	{[]byte("100002"), []byte("000001")},
	{[]byte("000011"), []byte("000001")},
	{[]byte("000012"), []byte("000001")},
}

type prefixTestEntry struct {
	key, val []byte
}

func TestLevelDBPrefixIterator(t *testing.T) {
	tests := []struct {
		prefix  []byte
		expSize int
	}{
		{
			prefix:  []byte("00000"),
			expSize: 5,
		},
		{
			prefix:  []byte("0000"),
			expSize: 7,
		},
		{
			prefix:  []byte(""),
			expSize: 9,
		},
		{
			prefix:  []byte("2"),
			expSize: 0,
		},
	}
	for i, test := range tests {
		db := newTestLevelDB(t, i)
		if err := prepareTestLvlDB(db); err != nil {
			t.Error(err)
		}
		it := db.NewPrefixIterator(test.prefix)
		count := 0
		for it.Next() {
			count++
		}
		if count != test.expSize {
			t.Errorf("Test %v: unexpected iteration size %v / %v", i, count, test.expSize)
		}
	}
}

func newTestLevelDB(t *testing.T, i int) database {
	dbDir := tempTestDir(t, i)
	db, err := newExplorerLvlDB(dbDir)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func prepareTestLvlDB(db database) error {
	for _, entry := range prefixTestData {
		key, val := entry.key, entry.val
		if err := db.Put(key, val); err != nil {
			return err
		}
	}
	return nil
}

func tempTestDir(t *testing.T, index int) string {
	tempDir := os.TempDir()
	testDir := path.Join(tempDir, "harmony", "explorer_db", t.Name(), strconv.Itoa(index))
	os.RemoveAll(testDir)
	return testDir
}

type memDB struct {
	keyValues map[string][]byte
	lock      sync.RWMutex
}

func newMemDB() *memDB {
	return &memDB{
		keyValues: make(map[string][]byte),
	}
}

func (db *memDB) Put(key, val []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	realKey := hex.EncodeToString(key)
	db.keyValues[realKey] = val
	return nil
}

func (db *memDB) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	realKey := hex.EncodeToString(key)
	_, ok := db.keyValues[realKey]
	return ok, nil
}

func (db *memDB) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	realKey := hex.EncodeToString(key)
	val, ok := db.keyValues[realKey]
	if !ok {
		return nil, leveldb.ErrNotFound
	}
	return val, nil
}

func (db *memDB) NewBatch() batch {
	return &memBatch{
		keyValues: make(map[string][]byte),
		db:        db,
	}
}

func (db *memDB) NewPrefixIterator(prefix []byte) iterator {
	db.lock.Lock()
	defer db.lock.Unlock()

	var (
		pr     = hex.EncodeToString(prefix)
		keys   = make([]string, 0, len(db.keyValues))
		values = make([][]byte, 0, len(db.keyValues))
	)
	for key := range db.keyValues {
		if strings.HasPrefix(key, pr) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		values = append(values, db.keyValues[key])
	}
	return &memPrefixIterator{
		keys:   keys,
		values: values,
		index:  -1,
	}
}

// TODO: implement this and verify
func (db *memDB) NewSizedIterator(start []byte, size int) iterator {
	return nil
}

type memBatch struct {
	keyValues map[string][]byte
	db        *memDB
	valueSize int
}

func (b *memBatch) Put(key, val []byte) error {
	b.keyValues[hex.EncodeToString(key)] = val
	b.valueSize += len(val)
	return nil
}

func (b *memBatch) Write() error {
	for k, v := range b.keyValues {
		b.db.keyValues[k] = v
	}
	b.valueSize = 0
	return nil
}

func (b *memBatch) ValueSize() int {
	return b.valueSize
}

type memPrefixIterator struct {
	keys   []string
	values [][]byte
	index  int
}

func (it *memPrefixIterator) Key() []byte {
	b, err := hex.DecodeString(it.keys[it.index])
	if err != nil {
		panic(err)
	}
	return b
}

func (it *memPrefixIterator) Value() []byte {
	return it.values[it.index]
}

func (it *memPrefixIterator) Release() {}

func (it *memPrefixIterator) Next() bool {
	if it.index >= len(it.keys)-1 {
		return false
	}
	it.index++
	return true
}

func (it *memPrefixIterator) Error() error {
	return nil
}

func makeTestTxHash(index int) common.Hash {
	var h common.Hash
	binary.BigEndian.PutUint64(h[:], uint64(index))
	return h
}

func makeOneAddress(index int) oneAddress {
	var raw common.Address
	binary.LittleEndian.PutUint64(raw[:], uint64(index))
	oneAddr, err := common2.AddressToBech32(raw)
	if err != nil {
		panic(err)
	}
	return oneAddress(oneAddr)
}
