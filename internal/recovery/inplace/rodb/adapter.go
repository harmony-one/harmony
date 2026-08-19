package rodb

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Latch records non-not-found read errors seen through the adapter. Stock
// rawdb readers and the trie resolver swallow read errors into "absence";
// the latch keeps a transient I/O error distinguishable from genuine
// absence, so it surfaces as a read error (exit 3), never as a false FAIL.
type Latch struct {
	mu            sync.Mutex
	first         error
	count         int
	writeAttempts int
}

// Record notes a non-nil, non-not-found error.
func (l *Latch) Record(err error) {
	if err == nil || IsNotFound(err) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.first == nil {
		l.first = err
	}
	l.count++
}

func (l *Latch) recordWriteAttempt() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeAttempts++
}

// First returns the first recorded error (nil if clean).
func (l *Latch) First() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.first
}

// Count returns the number of recorded errors.
func (l *Latch) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// WriteAttempts returns how many times a write method was invoked (each one
// was refused; a non-zero value indicates a programming error somewhere in
// the read pipeline).
func (l *Latch) WriteAttempts() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writeAttempts
}

// Reset clears the latch (used when a stage is retried after reopen).
func (l *Latch) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.first = nil
	l.count = 0
}

// KV is the write-refusing ethdb.KeyValueStore adapter over the read-only
// goleveldb handle (never-write layer 3).
type KV struct {
	ldb   *leveldb.DB
	latch *Latch
}

var _ ethdb.KeyValueStore = (*KV)(nil)

// Get passes through with error propagation; a missing key returns
// leveldb.ErrNotFound untouched, any other error is latched.
func (kv *KV) Get(key []byte) ([]byte, error) {
	dat, err := kv.ldb.Get(key, nil)
	if err != nil {
		kv.latch.Record(err)
		return nil, err
	}
	return dat, nil
}

// Has passes through with error propagation.
func (kv *KV) Has(key []byte) (bool, error) {
	ret, err := kv.ldb.Has(key, nil)
	if err != nil {
		kv.latch.Record(err)
		return false, err
	}
	return ret, nil
}

// Put is refused.
func (kv *KV) Put(key []byte, value []byte) error {
	kv.latch.recordWriteAttempt()
	return ErrWriteRefused
}

// Delete is refused.
func (kv *KV) Delete(key []byte) error {
	kv.latch.recordWriteAttempt()
	return ErrWriteRefused
}

// Compact is refused (it is a write path).
func (kv *KV) Compact(start []byte, limit []byte) error {
	kv.latch.recordWriteAttempt()
	return ErrWriteRefused
}

// Stat passes through to goleveldb's property reader.
func (kv *KV) Stat(property string) (string, error) {
	return kv.ldb.GetProperty(property)
}

// NewBatch returns a batch whose write methods are refused (the Batcher
// signature admits no error, so refusal happens on the batch's methods).
func (kv *KV) NewBatch() ethdb.Batch {
	return &refusingBatch{latch: kv.latch}
}

// NewBatchWithSize returns a write-refusing batch.
func (kv *KV) NewBatchWithSize(size int) ethdb.Batch {
	return &refusingBatch{latch: kv.latch}
}

// NewIterator wraps goleveldb's iterator over a binary-alphabetical prefix
// range, mirroring geth's leveldb wrapper semantics.
func (kv *KV) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	r := util.BytesPrefix(prefix)
	r.Start = append(r.Start, start...)
	return &latchingIterator{it: kv.ldb.NewIterator(r, nil), latch: kv.latch}
}

// NewSnapshot wraps goleveldb's native read-only snapshot.
func (kv *KV) NewSnapshot() (ethdb.Snapshot, error) {
	snap, err := kv.ldb.GetSnapshot()
	if err != nil {
		kv.latch.Record(err)
		return nil, err
	}
	return &roSnapshot{snap: snap, latch: kv.latch}, nil
}

// Close is a no-op: the rodb.DB owner controls the database lifecycle (the
// adapter is handed to library code that must not close it).
func (kv *KV) Close() error { return nil }

// Unlatched returns a view over the same database whose read errors are NOT
// recorded in the shared latch (writes are still refused). It backs
// strictly informational reads - the head sample - which must never gate
// the run or engage the retry machinery: a latched error from a moving head
// would send an otherwise clean run into retries and exit 3.
func (kv *KV) Unlatched() ethdb.KeyValueReader {
	return &KV{ldb: kv.ldb, latch: &Latch{}}
}

type latchingIterator struct {
	it    iterator.Iterator
	latch *Latch
}

func (i *latchingIterator) Next() bool    { return i.it.Next() }
func (i *latchingIterator) Key() []byte   { return i.it.Key() }
func (i *latchingIterator) Value() []byte { return i.it.Value() }
func (i *latchingIterator) Release()      { i.err(); i.it.Release() }

func (i *latchingIterator) Error() error { return i.err() }

func (i *latchingIterator) err() error {
	err := i.it.Error()
	if err != nil {
		i.latch.Record(err)
	}
	return err
}

type roSnapshot struct {
	snap  *leveldb.Snapshot
	latch *Latch
}

func (s *roSnapshot) Has(key []byte) (bool, error) {
	ok, err := s.snap.Has(key, nil)
	if err != nil {
		s.latch.Record(err)
	}
	return ok, err
}

func (s *roSnapshot) Get(key []byte) ([]byte, error) {
	dat, err := s.snap.Get(key, nil)
	if err != nil {
		s.latch.Record(err)
		return nil, err
	}
	return dat, nil
}

func (s *roSnapshot) Release() { s.snap.Release() }

type refusingBatch struct {
	latch *Latch
}

var _ ethdb.Batch = (*refusingBatch)(nil)

func (b *refusingBatch) Put(key, value []byte) error {
	b.latch.recordWriteAttempt()
	return ErrWriteRefused
}

func (b *refusingBatch) Delete(key []byte) error {
	b.latch.recordWriteAttempt()
	return ErrWriteRefused
}

func (b *refusingBatch) ValueSize() int { return 0 }

func (b *refusingBatch) Write() error {
	b.latch.recordWriteAttempt()
	return ErrWriteRefused
}

func (b *refusingBatch) Reset() {}

func (b *refusingBatch) Replay(w ethdb.KeyValueWriter) error {
	b.latch.recordWriteAttempt()
	return ErrWriteRefused
}
