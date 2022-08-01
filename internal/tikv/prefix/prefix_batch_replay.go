package prefix

import (
	"bytes"

	"github.com/ethereum/go-ethereum/ethdb"
)

type PrefixBatchReplay struct {
	prefix    []byte
	prefixLen int
	w         ethdb.KeyValueWriter
}

func newPrefixBatchReplay(prefix []byte, w ethdb.KeyValueWriter) *PrefixBatchReplay {
	return &PrefixBatchReplay{prefix: prefix, prefixLen: len(prefix), w: w}
}

// Put inserts the given value into the key-value data store.
func (p *PrefixBatchReplay) Put(key []byte, value []byte) error {
	if bytes.HasPrefix(key, p.prefix) {
		return p.w.Put(key[p.prefixLen:], value)
	} else {
		return p.w.Put(key, value)
	}
}

// Delete removes the key from the key-value data store.
func (p *PrefixBatchReplay) Delete(key []byte) error {
	if bytes.HasPrefix(key, p.prefix) {
		return p.w.Delete(key[p.prefixLen:])
	} else {
		return p.w.Delete(key)
	}
}
