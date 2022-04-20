package prefix

import (
	"github.com/ethereum/go-ethereum/ethdb"
)

type PrefixIterator struct {
	prefix    []byte
	prefixLen int

	it ethdb.Iterator
}

func newPrefixIterator(prefix []byte, it ethdb.Iterator) *PrefixIterator {
	return &PrefixIterator{prefix: prefix, prefixLen: len(prefix), it: it}
}

func (i *PrefixIterator) Next() bool {
	return i.it.Next()
}

func (i *PrefixIterator) Error() error {
	return i.it.Error()
}

func (i *PrefixIterator) Key() []byte {
	key := i.it.Key()
	if len(key) < len(i.prefix) {
		return nil
	}

	return key[i.prefixLen:]
}

func (i *PrefixIterator) Value() []byte {
	return i.it.Value()
}

func (i *PrefixIterator) Release() {
	i.it.Release()
}
