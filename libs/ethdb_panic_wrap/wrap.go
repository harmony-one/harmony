package ethdb_panic_wrap

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
)

var _ ethdb.Database = &Wrap{}

var locked uint32 = 0

func Set(v uint32) {
	atomic.StoreUint32(&locked, v)
}

func Get() uint32 {
	return atomic.LoadUint32(&locked)
}

type Wrap struct {
	db ethdb.Database
}

func New(db ethdb.Database) *Wrap {
	return &Wrap{
		db: db,
	}
}

func (w *Wrap) Has(key []byte) (bool, error) {
	return w.db.Has(key)
}

func (w *Wrap) Get(key []byte) ([]byte, error) {
	return w.db.Get(key)
}

func (w *Wrap) HasAncient(kind string, number uint64) (bool, error) {
	return w.db.HasAncient(kind, number)
}

func (w *Wrap) Ancient(kind string, number uint64) ([]byte, error) {
	return w.db.Ancient(kind, number)
}

func (w *Wrap) Ancients() (uint64, error) {
	return w.db.Ancients()
}

func (w *Wrap) AncientSize(kind string) (uint64, error) {
	return w.db.AncientSize(kind)
}

func (w *Wrap) Put(key []byte, value []byte) error {
	if Get() == 1 {
		panic("write is locked")
	}
	return w.db.Put(key, value)
}

func (w *Wrap) Delete(key []byte) error {
	if Get() == 1 {
		panic("write is locked")
	}
	return w.db.Delete(key)
}

func (w *Wrap) AppendAncient(number uint64, hash, header, body, receipt, td []byte) error {
	if Get() == 1 {
		panic("write is locked")
	}
	return w.db.AppendAncient(number, hash, header, body, receipt, td)
}

func (w *Wrap) TruncateAncients(n uint64) error {
	if Get() == 1 {
		panic("write is locked")
	}
	return w.db.TruncateAncients(n)
}

func (w *Wrap) Sync() error {
	if Get() == 1 {
		panic("write is locked")
	}
	return w.db.Sync()
}

func (w *Wrap) NewBatch() ethdb.Batch {
	return w.db.NewBatch()
}

func (w *Wrap) NewIterator() ethdb.Iterator {
	return w.db.NewIterator()
}

func (w *Wrap) NewIteratorWithStart(start []byte) ethdb.Iterator {
	return w.db.NewIteratorWithStart(start)
}

func (w *Wrap) NewIteratorWithPrefix(prefix []byte) ethdb.Iterator {
	return w.db.NewIteratorWithPrefix(prefix)
}

func (w *Wrap) Stat(property string) (string, error) {
	return w.db.Stat(property)
}

func (w *Wrap) Compact(start []byte, limit []byte) error {
	if Get() == 1 {
		panic("write is locked")
	}
	return w.db.Compact(start, limit)
}

func (w *Wrap) Close() error {
	return w.db.Close()
}
