package prefix

import (
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/tikv/byte_alloc"
	"github.com/harmony-one/harmony/internal/tikv/common"
)

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

func (p *PrefixDatabase) makeKey(keys []byte) []byte {
	prefixLen := len(p.prefix)
	byt := p.keysPool.Get(len(keys) + prefixLen)
	copy(byt, p.prefix)
	copy(byt[prefixLen:], keys)

	return byt
}

func (p *PrefixDatabase) Has(key []byte) (bool, error) {
	return p.db.Has(p.makeKey(key))
}

func (p *PrefixDatabase) Get(key []byte) ([]byte, error) {
	return p.db.Get(p.makeKey(key))
}

func (p *PrefixDatabase) Put(key []byte, value []byte) error {
	return p.db.Put(p.makeKey(key), value)
}

func (p *PrefixDatabase) Delete(key []byte) error {
	return p.db.Delete(p.makeKey(key))
}

func (p *PrefixDatabase) NewBatch() ethdb.Batch {
	return newPrefixBatch(p.prefix, p.db.NewBatch())
}

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

func (p *PrefixDatabase) NewIterator(start, end []byte) ethdb.Iterator {
	start = append(p.prefix, start...)

	if len(end) == 0 {
		end = p.buildLimitUsePrefix()
	} else {
		end = append(p.prefix, end...)
	}

	return newPrefixIterator(p.prefix, p.db.NewIterator(start, end))
}

func (p *PrefixDatabase) Stat(property string) (string, error) {
	return p.db.Stat(property)
}

func (p *PrefixDatabase) Compact(start []byte, limit []byte) error {
	return p.db.Compact(start, limit)
}

func (p *PrefixDatabase) Close() error {
	return p.db.Close()
}
