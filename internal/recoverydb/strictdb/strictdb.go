// Package strictdb provides fail-closed ethdb adapters for the recovery
// tools. Stock rawdb/off-chain helpers log-and-continue on write failures
// and never surface Iterator.Error() (plan §2.2.7); everything on a
// recovery write or verify path goes through these wrappers instead.
package strictdb

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
)

// ErrWriteRefused is returned by the write-refusing decorator.
var ErrWriteRefused = errors.New("strictdb: write refused on verification handle")

// LatchingBatch wraps an ethdb.Batch and latches the first error from any
// operation. Once latched, every subsequent operation returns the same
// error and Write refuses, so a partial batch can never be committed.
type LatchingBatch struct {
	mu    sync.Mutex
	inner ethdb.Batch
	err   error
	limit int // flush threshold in bytes (0 = manual flush only)
	db    ethdb.KeyValueStore
	count uint64
	bytes uint64
}

// NewLatchingBatch creates a latching batch over db. If limitBytes > 0 the
// batch self-flushes (checked) whenever the buffered size exceeds it.
func NewLatchingBatch(db ethdb.KeyValueStore, limitBytes int) *LatchingBatch {
	return &LatchingBatch{inner: db.NewBatch(), limit: limitBytes, db: db}
}

func (b *LatchingBatch) latch(err error) error {
	if err != nil && b.err == nil {
		b.err = err
	}
	return b.err
}

// Err returns the latched error, if any.
func (b *LatchingBatch) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// Put stores a key-value pair, latching any error and self-flushing past the
// configured limit.
func (b *LatchingBatch) Put(key, value []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	if err := b.latch(b.inner.Put(key, value)); err != nil {
		return err
	}
	b.count++
	b.bytes += uint64(len(key) + len(value))
	return b.maybeFlushLocked()
}

// Delete removes a key, latching any error.
func (b *LatchingBatch) Delete(key []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	if err := b.latch(b.inner.Delete(key)); err != nil {
		return err
	}
	return b.maybeFlushLocked()
}

func (b *LatchingBatch) maybeFlushLocked() error {
	if b.limit > 0 && b.inner.ValueSize() >= b.limit {
		return b.flushLocked()
	}
	return nil
}

func (b *LatchingBatch) flushLocked() error {
	if b.err != nil {
		return b.err
	}
	if err := b.latch(b.inner.Write()); err != nil {
		return err
	}
	b.inner.Reset()
	return nil
}

// Flush writes any buffered data, latching failures.
func (b *LatchingBatch) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushLocked()
}

// The methods below complete the ethdb.Batch interface so a LatchingBatch
// can be handed to library code (e.g. trie.Sync.Commit).

// ValueSize reports the buffered payload size.
func (b *LatchingBatch) ValueSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.inner.ValueSize()
}

// Write commits buffered writes without resetting (geth semantics), latched.
func (b *LatchingBatch) Write() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	return b.latch(b.inner.Write())
}

// Reset clears the buffer (the latched error, if any, stays latched).
func (b *LatchingBatch) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inner.Reset()
}

// Replay replays buffered operations onto w, latched.
func (b *LatchingBatch) Replay(w ethdb.KeyValueWriter) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	return b.latch(b.inner.Replay(w))
}

// Compile-time interface pin.
var _ ethdb.Batch = (*LatchingBatch)(nil)

// Count returns the number of Put operations accepted so far.
func (b *LatchingBatch) Count() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// Bytes returns the logical bytes accepted so far.
func (b *LatchingBatch) Bytes() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bytes
}

// ForEach iterates db over prefix with a strict iterator: the callback's
// error aborts, and Iterator.Error() is always surfaced (unlike the stock
// off-chain Iterator* helpers). The callback receives copies of key/value
// slices only valid during the call; copy if retained.
func ForEach(db ethdb.Iteratee, prefix []byte, fn func(key, value []byte) error) error {
	it := db.NewIterator(prefix, nil)
	defer it.Release()
	for it.Next() {
		if err := fn(it.Key(), it.Value()); err != nil {
			return err
		}
	}
	if err := it.Error(); err != nil {
		return fmt.Errorf("strictdb: iterator error over prefix %q: %w", prefix, err)
	}
	return nil
}

// Get is a strict read: a missing key is an explicit error naming the key,
// never a silent nil.
func Get(db ethdb.KeyValueReader, key []byte) ([]byte, error) {
	v, err := db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("strictdb: read key %x: %w", key, err)
	}
	return v, nil
}

// WritePreimages is the checked replacement for rawdb.WritePreimages (which
// logs and continues on failure).
func WritePreimages(w ethdb.KeyValueWriter, preimages map[common.Hash][]byte) error {
	for hash, preimage := range preimages {
		key := append(append([]byte{}, rawdb.PreimagePrefix...), hash.Bytes()...)
		if err := w.Put(key, preimage); err != nil {
			return fmt.Errorf("strictdb: write preimage %s: %w", hash.Hex(), err)
		}
	}
	return nil
}

// WriteCode is the checked replacement for rawdb.WriteCode.
func WriteCode(w ethdb.KeyValueWriter, hash common.Hash, code []byte) error {
	key := append(append([]byte{}, rawdb.CodePrefix...), hash.Bytes()...)
	if err := w.Put(key, code); err != nil {
		return fmt.Errorf("strictdb: write code %s: %w", hash.Hex(), err)
	}
	return nil
}

// WriteValidatorCode is the checked replacement for rawdb.WriteValidatorCode.
func WriteValidatorCode(w ethdb.KeyValueWriter, hash common.Hash, code []byte) error {
	key := append(append([]byte{}, rawdb.ValidatorCodePrefix...), hash.Bytes()...)
	if err := w.Put(key, code); err != nil {
		return fmt.Errorf("strictdb: write validator code %s: %w", hash.Hex(), err)
	}
	return nil
}

// WriteRefusingDB decorates an ethdb.Database so that every mutation is
// refused. Used defensively on verification paths that receive a writable
// handle. Reads and iteration delegate.
type WriteRefusingDB struct {
	ethdb.Database
}

// NewWriteRefusing wraps db.
func NewWriteRefusing(db ethdb.Database) *WriteRefusingDB {
	return &WriteRefusingDB{Database: db}
}

// Put refuses.
func (d *WriteRefusingDB) Put(key []byte, value []byte) error { return ErrWriteRefused }

// Delete refuses.
func (d *WriteRefusingDB) Delete(key []byte) error { return ErrWriteRefused }

// Compact refuses (it rewrites SSTs).
func (d *WriteRefusingDB) Compact(start []byte, limit []byte) error { return ErrWriteRefused }

// NewBatch returns a refusing batch.
func (d *WriteRefusingDB) NewBatch() ethdb.Batch { return refusingBatch{} }

// NewBatchWithSize returns a refusing batch.
func (d *WriteRefusingDB) NewBatchWithSize(int) ethdb.Batch { return refusingBatch{} }

type refusingBatch struct{}

var _ ethdb.Batch = refusingBatch{}

func (refusingBatch) Put(key, value []byte) error       { return ErrWriteRefused }
func (refusingBatch) Delete(key []byte) error           { return ErrWriteRefused }
func (refusingBatch) ValueSize() int                    { return 0 }
func (refusingBatch) Write() error                      { return ErrWriteRefused }
func (refusingBatch) Reset()                            {}
func (refusingBatch) Replay(ethdb.KeyValueWriter) error { return ErrWriteRefused }
