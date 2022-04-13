package ethdb_memwrap

import "github.com/ethereum/go-ethereum/ethdb"

var _ ethdb.Batch = &BatchMemWrap{}

type BatchMemWrap struct {
	state     state
	dbWrapper *DbWrapper
}

func (b *BatchMemWrap) Put(key []byte, value []byte) error {
	b.state.put(key, value)
	return nil
}

func (b *BatchMemWrap) Delete(key []byte) error {
	panic("Delete method is not implemented for BatchMemWrap")
}

func (b *BatchMemWrap) ValueSize() int {
	return 0
}

func (b *BatchMemWrap) Write() error {
	for _, v := range b.state {
		for _, val := range v {
			err := b.dbWrapper.Put(val.key, val.value)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BatchMemWrap) Reset() {
	b.state = make(state)
}

func (b *BatchMemWrap) Replay(w ethdb.KeyValueWriter) error {
	// No reason to have such method.
	panic("Replay method is not implemented for BatchMemWrap")
}
