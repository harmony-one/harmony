package ethdb_memwrap

import (
	"bytes"

	"github.com/ethereum/go-ethereum/ethdb"
)

type state map[[8]byte][]kv

func (s state) put(key []byte, value []byte) {
	insert := kv{
		key:   key,
		value: value,
	}
	state := s[insert.arr()]
	if len(state) > 0 {
		for i := range state {
			if bytes.Equal(state[i].key, insert.key) {
				state[i].value = insert.value
				return
			}
		}
	}
	s[insert.arr()] = append(state, insert)
}

func (s state) get(key []byte) ([]byte, bool) {
	insert := kv{
		key: key,
	}
	state := s[insert.arr()]
	for _, row := range state {
		if bytes.Equal(row.key, key) {
			return row.value, true
		}
	}
	return nil, false
}

type kv struct {
	key   []byte
	value []byte
}

func (k kv) arr() (out [8]byte) {
	copy(out[:], k.key)
	return
}

type DbWrapper struct {
	db    ethdb.Database
	wrap  bool
	state state
}

func (d *DbWrapper) ValueSize() int {
	return 0
}

func (d *DbWrapper) Reset() {
}

func (d *DbWrapper) Replay(w ethdb.KeyValueWriter) error {
	return nil
}

func NewDbWrapper(db ethdb.Database) *DbWrapper {
	return &DbWrapper{
		db:    db,
		wrap:  false,
		state: make(map[[8]byte][]kv),
	}
}

func (d *DbWrapper) Wrap(wrap bool) {
	d.wrap = wrap
}

func (d *DbWrapper) Has(key []byte) (bool, error) {
	if d.wrap {
		_, ok := d.state.get(key)
		if ok {
			return true, nil
		}
	}
	return d.db.Has(key)
}

func (d *DbWrapper) Get(key []byte) ([]byte, error) {
	if d.wrap {
		if val, ok := d.state.get(key); ok {
			return val, nil
		}
	}
	return d.db.Get(key)
}

/*
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
*/

func (d *DbWrapper) HasAncient(kind string, number uint64) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Ancient(kind string, number uint64) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Ancients() (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) AncientSize(kind string) (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Put(key []byte, value []byte) error {
	if d.wrap {
		d.state.put(key, value)
		return nil
	}
	return d.db.Put(key, value)
}

func (d *DbWrapper) Delete(key []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) AppendAncient(number uint64, hash, header, body, receipt, td []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) TruncateAncients(n uint64) error {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Sync() error {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) NewBatch() ethdb.Batch {
	return d
}

func (d *DbWrapper) NewIterator() ethdb.Iterator {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) NewIteratorWithStart(start []byte) ethdb.Iterator {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) NewIteratorWithPrefix(prefix []byte) ethdb.Iterator {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Stat(property string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Compact(start []byte, limit []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Close() error {
	//TODO implement me
	panic("implement me")
}

func (d *DbWrapper) Clear() {
	d.state = make(map[[8]byte][]kv)
}

func (d *DbWrapper) Write() error {
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

type dbBatchWrapper struct {
	state map[[8]byte][]kv
}
