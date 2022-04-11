package core

import (
	"bytes"

	"github.com/ethereum/go-ethereum/ethdb"
)

type kv struct {
	key   []byte
	value []byte
}

func (k kv) arr() (out [8]byte) {
	copy(out[:], k.key)
	return
}

type dbWrapper struct {
	db    ethdb.Database
	wrap  bool
	state map[[8]byte][]kv
}

func (d *dbWrapper) ValueSize() int {
	return 0
}

func (d *dbWrapper) Reset() {
}

func (d *dbWrapper) Replay(w ethdb.KeyValueWriter) error {
	return nil
}

func NewDbWrapper(db ethdb.Database) *dbWrapper {
	return &dbWrapper{
		db:    db,
		wrap:  false,
		state: make(map[[8]byte][]kv),
	}
}

func (d *dbWrapper) Wrap(wrap bool) {
	d.wrap = wrap
}

func (d *dbWrapper) Has(key []byte) (bool, error) {
	if d.wrap {
		_, ok := d.get(key)
		if ok {
			return true, nil
		}
	}
	return d.db.Has(key)
}

func (d *dbWrapper) Get(key []byte) ([]byte, error) {
	if d.wrap {
		if val, ok := d.get(key); ok {
			return val, nil
		}
	}
	return d.db.Get(key)
}

func (d *dbWrapper) get(key []byte) ([]byte, bool) {
	insert := kv{
		key: key,
	}
	state := d.state[insert.arr()]
	for _, row := range state {
		if bytes.Equal(row.key, key) {
			return row.value, true
		}
	}
	return nil, false
}

func (d *dbWrapper) HasAncient(kind string, number uint64) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Ancient(kind string, number uint64) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Ancients() (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) AncientSize(kind string) (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Put(key []byte, value []byte) error {
	if d.wrap {
		return d.put(key, value)
	}
	return d.db.Put(key, value)
}

func (d *dbWrapper) put(key []byte, value []byte) error {
	insert := kv{
		key:   key,
		value: value,
	}
	state := d.state[insert.arr()]
	if len(state) > 0 {
		for i := range state {
			if bytes.Equal(state[i].key, insert.key) {
				state[i].value = insert.value
				return nil
			}
		}
	}
	d.state[insert.arr()] = append(state, insert)
	return nil
}

func (d *dbWrapper) Delete(key []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) AppendAncient(number uint64, hash, header, body, receipt, td []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) TruncateAncients(n uint64) error {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Sync() error {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) NewBatch() ethdb.Batch {
	return d
}

func (d *dbWrapper) NewIterator() ethdb.Iterator {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) NewIteratorWithStart(start []byte) ethdb.Iterator {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) NewIteratorWithPrefix(prefix []byte) ethdb.Iterator {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Stat(property string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Compact(start []byte, limit []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Close() error {
	//TODO implement me
	panic("implement me")
}

func (d *dbWrapper) Clear() {
	d.state = make(map[[8]byte][]kv)
}

func (d *dbWrapper) Write() error {
	batch := d.db.NewBatch()
	for _, v := range d.state {
		for _, val := range v {
			err := batch.Put(val.key, val.value)
			if err != nil {
				return err
			}
		}
		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}
	}
	if batch.ValueSize() > 0 {
		return batch.Write()
	}
	return nil
}
