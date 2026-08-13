package dbopen

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/ethdb"
)

// ProbeDB wraps a ReadOnlyDB for the inspect baseline gate's harness-open
// probe (plan §7.2 "open/close causes no repair/rewind"). The stock
// BlockChainImpl unconditionally re-writes the LastHeader head pointer at
// every open (HeaderChain.SetCurrentHeader), even when nothing changed, so a
// literal write-refusing handle can never complete a clean open. The probe
// therefore tolerates exactly the no-op case: a Put whose value is
// byte-identical to the stored value is swallowed (nothing reaches the
// disk); any value-CHANGING Put — i.e. an actual repair or rewind — and any
// Delete is refused with ErrReadOnly, failing the open. The directory
// remains byte-untouched in all cases.
type ProbeDB struct {
	inner *ReadOnlyDB
	// SwallowedWrites counts tolerated identical-value writes (reported).
	SwallowedWrites uint64
}

var _ ethdb.KeyValueStore = (*ProbeDB)(nil)

// NewProbe wraps an open read-only handle.
func NewProbe(ro *ReadOnlyDB) *ProbeDB { return &ProbeDB{inner: ro} }

// Has delegates.
func (p *ProbeDB) Has(key []byte) (bool, error) { return p.inner.Has(key) }

// Get delegates.
func (p *ProbeDB) Get(key []byte) ([]byte, error) { return p.inner.Get(key) }

// Put tolerates identical-value rewrites, refuses anything else.
func (p *ProbeDB) Put(key []byte, value []byte) error {
	existing, err := p.inner.Get(key)
	if err == nil && bytes.Equal(existing, value) {
		p.SwallowedWrites++
		return nil
	}
	return fmt.Errorf("%w (probe: value-changing write to %x)", ErrReadOnly, key)
}

// Delete refuses.
func (p *ProbeDB) Delete(key []byte) error {
	return fmt.Errorf("%w (probe: delete of %x)", ErrReadOnly, key)
}

// Stat delegates.
func (p *ProbeDB) Stat(property string) (string, error) { return p.inner.Stat(property) }

// Compact refuses.
func (p *ProbeDB) Compact(start []byte, limit []byte) error { return ErrReadOnly }

// NewBatch returns a batch with the same idempotent-tolerant semantics,
// validated at Write time.
func (p *ProbeDB) NewBatch() ethdb.Batch { return &probeBatch{db: p} }

// NewBatchWithSize returns a probe batch.
func (p *ProbeDB) NewBatchWithSize(size int) ethdb.Batch { return &probeBatch{db: p} }

// NewIterator delegates.
func (p *ProbeDB) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return p.inner.NewIterator(prefix, start)
}

// NewSnapshot delegates.
func (p *ProbeDB) NewSnapshot() (ethdb.Snapshot, error) { return p.inner.NewSnapshot() }

// Close does NOT close the inner handle (the caller owns it).
func (p *ProbeDB) Close() error { return nil }

type probeOp struct {
	key, value []byte
	delete     bool
}

type probeBatch struct {
	db  *ProbeDB
	ops []probeOp
	sz  int
}

var _ ethdb.Batch = (*probeBatch)(nil)

func (b *probeBatch) Put(key, value []byte) error {
	b.ops = append(b.ops, probeOp{key: append([]byte{}, key...), value: append([]byte{}, value...)})
	b.sz += len(key) + len(value)
	return nil
}

func (b *probeBatch) Delete(key []byte) error {
	b.ops = append(b.ops, probeOp{key: append([]byte{}, key...), delete: true})
	b.sz += len(key)
	return nil
}

func (b *probeBatch) ValueSize() int { return b.sz }

func (b *probeBatch) Write() error {
	for _, op := range b.ops {
		if op.delete {
			return fmt.Errorf("%w (probe batch: delete of %x)", ErrReadOnly, op.key)
		}
		if err := b.db.Put(op.key, op.value); err != nil {
			return err
		}
	}
	return nil
}

func (b *probeBatch) Reset() { b.ops, b.sz = nil, 0 }

func (b *probeBatch) Replay(w ethdb.KeyValueWriter) error {
	for _, op := range b.ops {
		if op.delete {
			if err := w.Delete(op.key); err != nil {
				return err
			}
			continue
		}
		if err := w.Put(op.key, op.value); err != nil {
			return err
		}
	}
	return nil
}
