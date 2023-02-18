package remote

import (
	"context"
	"sync"
)

const (
	iteratorOnce = 300
)

type RemoteIterator struct {
	db   *RemoteDatabase
	lock sync.Mutex

	limit []byte
	start []byte

	err error
	end bool
	pos int

	keys, values             [][]byte
	currentKey, currentValue []byte
}

func newRemoteIterator(db *RemoteDatabase, start, limit []byte) *RemoteIterator {
	return &RemoteIterator{
		db:    db,
		start: start,
		limit: limit,
	}
}

// Next moves the iterator to the next key/value pair. It returns whether the
// iterator is exhausted.
func (i *RemoteIterator) Next() bool {
	if i.end {
		return false
	}

	if i.keys == nil {
		if next := i.scanNext(); !next {
			return false
		}
	}

	i.currentKey = i.keys[i.pos]
	i.currentValue = i.values[i.pos]
	i.pos++

	if i.pos >= len(i.keys) {
		i.scanNext()
	}

	return true
}

// scanNext real scan from tikv, and cache it
func (i *RemoteIterator) scanNext() bool {
	keys, values, err := i.db.client.Scan(context.Background(), i.start, i.limit, iteratorOnce)
	if err != nil {
		i.err = err
		i.end = true
		return false
	}

	if len(keys) == 0 {
		i.end = true
		return false
	} else {
		i.start = append(keys[len(keys)-1], 0)
	}

	i.pos = 0
	i.keys = keys
	i.values = values
	return true
}

// Error returns any accumulated error. Exhausting all the key/value pairs
// is not considered to be an error.
func (i *RemoteIterator) Error() error {
	return i.err
}

// Key returns the key of the current key/value pair, or nil if done. The caller
// should not modify the contents of the returned slice, and its contents may
// change on the next call to Next.
func (i *RemoteIterator) Key() []byte {
	return i.currentKey
}

// Value returns the value of the current key/value pair, or nil if done. The
// caller should not modify the contents of the returned slice, and its contents
// may change on the next call to Next.
func (i *RemoteIterator) Value() []byte {
	return i.currentValue
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (i *RemoteIterator) Release() {
	i.db = nil
	i.end = true
	i.keys = nil
	i.values = nil
	i.currentKey = nil
	i.currentValue = nil
}
