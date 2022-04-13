package ethdb_locker

import (
	"github.com/ethereum/go-ethereum/ethdb"
)

var _ ethdb.Database = &RWLockWrapper{}

type RWLocker interface {
	Lock()
	Unlock()
	RUnlock()
	RLock()
}

type RWLockWrapper struct {
	db   ethdb.Database
	lock RWLocker
}

func (d *RWLockWrapper) Has(key []byte) (bool, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.Has(key)
}

func (d *RWLockWrapper) Get(key []byte) ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.Get(key)
}

func (d *RWLockWrapper) HasAncient(kind string, number uint64) (bool, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.HasAncient(kind, number)
}

func (d *RWLockWrapper) Ancient(kind string, number uint64) ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.Ancient(kind, number)
}

func (d *RWLockWrapper) Ancients() (uint64, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.Ancients()
}

func (d *RWLockWrapper) AncientSize(kind string) (uint64, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.AncientSize(kind)
}

func (d *RWLockWrapper) Put(key []byte, value []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.Put(key, value)
}

func (d *RWLockWrapper) Delete(key []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.Delete(key)
}

func (d *RWLockWrapper) AppendAncient(number uint64, hash, header, body, receipt, td []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.AppendAncient(number, hash, header, body, receipt, td)
}

func (d *RWLockWrapper) TruncateAncients(n uint64) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.TruncateAncients(n)
}

func (d *RWLockWrapper) Sync() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.Sync()
}

func (d *RWLockWrapper) NewBatch() ethdb.Batch {
	return NewRWLockBatchWrapper(d.db.NewBatch(), d.lock)
}

func (d *RWLockWrapper) NewIterator() ethdb.Iterator {
	panic("NewIterator method is not implemented for ethdb lock wrapper")
}

func (d *RWLockWrapper) NewIteratorWithStart(start []byte) ethdb.Iterator {
	panic("NewIteratorWithStart method is not implemented for ethdb lock wrapper")
}

func (d *RWLockWrapper) NewIteratorWithPrefix(prefix []byte) ethdb.Iterator {
	panic("NewIteratorWithPrefix method is not implemented for ethdb lock wrapper")
}

func (d *RWLockWrapper) Stat(property string) (string, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.db.Stat(property)
}

func (d *RWLockWrapper) Compact(start []byte, limit []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.Compact(start, limit)
}

func (d *RWLockWrapper) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.db.Close()
}

func NewEthWrapperWithLocks(db ethdb.Database, mu RWLocker) *RWLockWrapper {
	return &RWLockWrapper{
		db:   db,
		lock: mu,
	}
}
