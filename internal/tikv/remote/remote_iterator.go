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

func (i *RemoteIterator) Error() error {
	return i.err
}

func (i *RemoteIterator) Key() []byte {
	return i.currentKey
}

func (i *RemoteIterator) Value() []byte {
	return i.currentValue
}

func (i *RemoteIterator) Release() {
	i.db = nil
	i.end = true
	i.keys = nil
	i.values = nil
	i.currentKey = nil
	i.currentValue = nil
}
