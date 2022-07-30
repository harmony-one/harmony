package prefix

import "github.com/ethereum/go-ethereum/ethdb"

type PrefixBatch struct {
	prefix []byte
	batch  ethdb.Batch
}

func newPrefixBatch(prefix []byte, batch ethdb.Batch) *PrefixBatch {
	return &PrefixBatch{prefix: prefix, batch: batch}
}

// Put inserts the given value into the key-value data store.
func (p *PrefixBatch) Put(key []byte, value []byte) error {
	return p.batch.Put(append(append([]byte{}, p.prefix...), key...), value)
}

// Delete removes the key from the key-value data store.
func (p *PrefixBatch) Delete(key []byte) error {
	return p.batch.Delete(append(append([]byte{}, p.prefix...), key...))
}

// ValueSize retrieves the amount of data queued up for writing.
func (p *PrefixBatch) ValueSize() int {
	return p.batch.ValueSize()
}

// Write flushes any accumulated data to disk.
func (p *PrefixBatch) Write() error {
	return p.batch.Write()
}

// Reset resets the batch for reuse.
func (p *PrefixBatch) Reset() {
	p.batch.Reset()
}

// Replay replays the batch contents.
func (p *PrefixBatch) Replay(w ethdb.KeyValueWriter) error {
	return p.batch.Replay(newPrefixBatchReplay(p.prefix, w))
}
