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

// Next moves the iterator to the next key/value pair. It returns whether the
// iterator is exhausted.
func (i *PrefixIterator) Next() bool {
	return i.it.Next()
}

// Error returns any accumulated error. Exhausting all the key/value pairs
// is not considered to be an error.
func (i *PrefixIterator) Error() error {
	return i.it.Error()
}

// Key returns the key of the current key/value pair, or nil if done. The caller
// should not modify the contents of the returned slice, and its contents may
// change on the next call to Next.
func (i *PrefixIterator) Key() []byte {
	key := i.it.Key()
	if len(key) < len(i.prefix) {
		return nil
	}

	return key[i.prefixLen:]
}

// Value returns the value of the current key/value pair, or nil if done. The
// caller should not modify the contents of the returned slice, and its contents
// may change on the next call to Next.
func (i *PrefixIterator) Value() []byte {
	return i.it.Value()
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (i *PrefixIterator) Release() {
	i.it.Release()
}
