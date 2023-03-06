package prefix

import (
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/tikv/byte_alloc"
	"github.com/harmony-one/harmony/internal/tikv/common"
)

// PrefixDatabase is a wrapper to split the storage with prefix
type PrefixDatabase struct {
	prefix   []byte
	db       common.TiKVStore
	keysPool *byte_alloc.Allocator
}

func NewPrefixDatabase(prefix []byte, db common.TiKVStore) *PrefixDatabase {
	return &PrefixDatabase{
		prefix:   prefix,
		db:       db,
		keysPool: byte_alloc.NewAllocator(),
	}
}

func (p *PrefixDatabase) AncientDatadir() (string, error) {
	return "", nil
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (p *PrefixDatabase) NewBatchWithSize(size int) ethdb.Batch {
	return nil
}

// makeKey use to create a key with prefix, keysPool can reduce gc pressure
func (p *PrefixDatabase) makeKey(keys []byte) []byte {
	prefixLen := len(p.prefix)
	byt := p.keysPool.Get(len(keys) + prefixLen)
	copy(byt, p.prefix)
	copy(byt[prefixLen:], keys)

	return byt
}

// Has retrieves if a key is present in the key-value data store.
func (p *PrefixDatabase) Has(key []byte) (bool, error) {
	return p.db.Has(p.makeKey(key))
}

// Get retrieves the given key if it's present in the key-value data store.
func (p *PrefixDatabase) Get(key []byte) ([]byte, error) {
	return p.db.Get(p.makeKey(key))
}

// Put inserts the given value into the key-value data store.
func (p *PrefixDatabase) Put(key []byte, value []byte) error {
	return p.db.Put(p.makeKey(key), value)
}

// Delete removes the key from the key-value data store.
func (p *PrefixDatabase) Delete(key []byte) error {
	return p.db.Delete(p.makeKey(key))
}

// NewBatch creates a write-only database that buffers changes to its host db
// until a final write is called.
func (p *PrefixDatabase) NewBatch() ethdb.Batch {
	return newPrefixBatch(p.prefix, p.db.NewBatch())
}

// buildLimitUsePrefix build the limit byte from start byte, useful generating from prefix works
func (p *PrefixDatabase) buildLimitUsePrefix() []byte {
	var limit []byte
	for i := len(p.prefix) - 1; i >= 0; i-- {
		c := p.prefix[i]
		if c < 0xff {
			limit = make([]byte, i+1)
			copy(limit, p.prefix)
			limit[i] = c + 1
			break
		}
	}

	return limit
}

// NewIterator creates a binary-alphabetical iterator over the start to end keyspace
// contained within the key-value database.
func (p *PrefixDatabase) NewIterator(start, end []byte) ethdb.Iterator {
	start = append(p.prefix, start...)

	if len(end) == 0 {
		end = p.buildLimitUsePrefix()
	} else {
		end = append(p.prefix, end...)
	}

	return newPrefixIterator(p.prefix, p.db.NewIterator(start, end))
}

// Stat returns a particular internal stat of the database.
func (p *PrefixDatabase) Stat(property string) (string, error) {
	return p.db.Stat(property)
}

// Compact flattens the underlying data store for the given key range. In essence,
// deleted and overwritten versions are discarded, and the data is rearranged to
// reduce the cost of operations needed to access them.
//
// A nil start is treated as a key before all keys in the data store; a nil limit
// is treated as a key after all keys in the data store. If both is nil then it
// will compact entire data store.
func (p *PrefixDatabase) Compact(start []byte, limit []byte) error {
	return p.db.Compact(start, limit)
}

// Close the storage
func (p *PrefixDatabase) Close() error {
	return p.db.Close()
}
