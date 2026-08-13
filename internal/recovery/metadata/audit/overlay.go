// Package audit implements `harmony-recovery metadata audit-branch` (plan
// §4.6): the masked-overlay re-execution of the abandoned branch. Reads
// consult scratch first, then the source minus a mask; every write and
// delete goes to scratch only. Before any chain object is constructed,
// scratch is seeded with the full mechanical application of the
// normalization output plus the post-target chain tombstones and the heads
// rewound to the target — the overlay presents exactly the post-apply
// target DB (a dry run of B4's end state). Branch-write accounting starts
// after a seed barrier; seed writes are baseline.
package audit

import (
	"bytes"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/syndtr/goleveldb/leveldb"
	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Overlay is the masked overlay ethdb.KeyValueStore.
type Overlay struct {
	mu      sync.RWMutex
	scratch *leveldb.DB
	source  ethdb.KeyValueStore
	// masked hides source keys: seed tombstones plus runtime deletes.
	masked map[string]struct{}
	// barrier separates seed writes (baseline) from branch writes.
	barrier bool
	// log records post-barrier touched keys (op counts only; final values
	// are read back from scratch at reconciliation time).
	log map[string]*WriteLogEntry
	// pointerTrail records every successive value written to crosslink
	// pointer keys (pointer write evolution, plan §4.6 output 6).
	pointerTrail map[string][][]byte
}

// WriteLogEntry summarizes post-barrier activity on one key.
type WriteLogEntry struct {
	Puts    int
	Deletes int
}

// NewOverlay opens the scratch database (created fresh by the caller).
func NewOverlay(scratchPath string, source ethdb.KeyValueStore) (*Overlay, error) {
	sdb, err := leveldb.OpenFile(scratchPath, nil)
	if err != nil {
		return nil, err
	}
	return &Overlay{
		scratch:      sdb,
		source:       source,
		masked:       map[string]struct{}{},
		log:          map[string]*WriteLogEntry{},
		pointerTrail: map[string][][]byte{},
	}, nil
}

// SealSeed marks the seed complete: subsequent writes are branch writes.
func (o *Overlay) SealSeed() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.barrier = true
}

// Mask hides a source key (seed tombstone).
func (o *Overlay) Mask(key []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.masked[string(key)] = struct{}{}
}

// SeedPut materializes a rewrite during seeding.
func (o *Overlay) SeedPut(key, value []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.masked, string(key))
	return o.scratch.Put(key, value, nil)
}

// Log returns a copy of the post-barrier write log.
func (o *Overlay) Log() map[string]WriteLogEntry {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string]WriteLogEntry, len(o.log))
	for k, v := range o.log {
		out[k] = *v
	}
	return out
}

// PointerTrail returns the recorded pointer write evolution.
func (o *Overlay) PointerTrail() map[string][][]byte {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string][][]byte, len(o.pointerTrail))
	for k, v := range o.pointerTrail {
		out[k] = v
	}
	return out
}

// Close closes the scratch handle (the source is owned by the caller).
func (o *Overlay) Close() error { return o.scratch.Close() }

var _ ethdb.KeyValueStore = (*Overlay)(nil)

// Get consults scratch, then the unmasked source.
func (o *Overlay) Get(key []byte) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.getLocked(key)
}

func (o *Overlay) getLocked(key []byte) ([]byte, error) {
	v, err := o.scratch.Get(key, nil)
	if err == nil {
		return v, nil
	}
	if err != leveldb.ErrNotFound {
		return nil, err
	}
	if _, hidden := o.masked[string(key)]; hidden {
		return nil, ldberrors.ErrNotFound
	}
	return o.source.Get(key)
}

// Has mirrors Get.
func (o *Overlay) Has(key []byte) (bool, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if ok, err := o.scratch.Has(key, nil); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	if _, hidden := o.masked[string(key)]; hidden {
		return false, nil
	}
	return o.source.Has(key)
}

// Put writes to scratch only, unhides the key, and logs post-barrier.
func (o *Overlay) Put(key, value []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.putLocked(key, value)
}

func (o *Overlay) putLocked(key, value []byte) error {
	if err := o.scratch.Put(key, value, nil); err != nil {
		return err
	}
	delete(o.masked, string(key))
	if o.barrier {
		e := o.log[string(key)]
		if e == nil {
			e = &WriteLogEntry{}
			o.log[string(key)] = e
		}
		e.Puts++
		if isPointerKey(key) {
			o.pointerTrail[string(key)] = append(o.pointerTrail[string(key)], append([]byte(nil), value...))
		}
	}
	return nil
}

// Delete tombstones the key (masking any source value) and logs.
func (o *Overlay) Delete(key []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.deleteLocked(key)
}

func (o *Overlay) deleteLocked(key []byte) error {
	if err := o.scratch.Delete(key, nil); err != nil {
		return err
	}
	o.masked[string(key)] = struct{}{}
	if o.barrier {
		e := o.log[string(key)]
		if e == nil {
			e = &WriteLogEntry{}
			o.log[string(key)] = e
		}
		e.Deletes++
	}
	return nil
}

func isPointerKey(key []byte) bool {
	return len(key) == 6 && key[0] == 'c' && key[1] == 'l'
}

func (o *Overlay) Stat(property string) (string, error) { return o.scratch.GetProperty(property) }

// Compact compacts scratch only.
func (o *Overlay) Compact(start, limit []byte) error {
	return o.scratch.CompactRange(util.Range{Start: start, Limit: limit})
}

// NewBatch buffers ops and replays them through the overlay on Write, so
// tombstones and the write log stay consistent.
func (o *Overlay) NewBatch() ethdb.Batch { return &overlayBatch{o: o} }

// NewBatchWithSize is NewBatch.
func (o *Overlay) NewBatchWithSize(size int) ethdb.Batch { return &overlayBatch{o: o} }

// NewSnapshot returns a point-in-time read view. The audit's sequential
// single-writer flow never needs isolation beyond the live view.
func (o *Overlay) NewSnapshot() (ethdb.Snapshot, error) { return &overlaySnap{o: o}, nil }

type overlaySnap struct{ o *Overlay }

func (s *overlaySnap) Has(key []byte) (bool, error)   { return s.o.Has(key) }
func (s *overlaySnap) Get(key []byte) ([]byte, error) { return s.o.Get(key) }
func (s *overlaySnap) Release()                       {}

type batchOp struct {
	del        bool
	key, value []byte
}

type overlayBatch struct {
	o    *Overlay
	ops  []batchOp
	size int
}

var _ ethdb.Batch = (*overlayBatch)(nil)

func (b *overlayBatch) Put(key, value []byte) error {
	b.ops = append(b.ops, batchOp{key: append([]byte(nil), key...), value: append([]byte(nil), value...)})
	b.size += len(key) + len(value)
	return nil
}

func (b *overlayBatch) Delete(key []byte) error {
	b.ops = append(b.ops, batchOp{del: true, key: append([]byte(nil), key...)})
	b.size += len(key)
	return nil
}

func (b *overlayBatch) ValueSize() int { return b.size }

func (b *overlayBatch) Write() error {
	b.o.mu.Lock()
	defer b.o.mu.Unlock()
	for _, op := range b.ops {
		if op.del {
			if err := b.o.deleteLocked(op.key); err != nil {
				return err
			}
			continue
		}
		if err := b.o.putLocked(op.key, op.value); err != nil {
			return err
		}
	}
	return nil
}

func (b *overlayBatch) Reset() { b.ops, b.size = nil, 0 }

func (b *overlayBatch) Replay(w ethdb.KeyValueWriter) error {
	for _, op := range b.ops {
		if op.del {
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

// NewIterator merges scratch and unmasked-source iterators in ascending
// key order; scratch wins ties.
func (o *Overlay) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	r := util.BytesPrefix(prefix)
	r.Start = append(r.Start, start...)
	scratchIt := o.scratch.NewIterator(r, nil)
	srcIt := o.source.NewIterator(prefix, start)
	it := &mergedIterator{o: o, a: &ldbIter{scratchIt}, b: srcIt}
	return it
}

// ldbIter adapts a goleveldb iterator to the ethdb.Iterator shape.
type ldbIter struct{ it interface {
	Next() bool
	Key() []byte
	Value() []byte
	Release()
	Error() error
} }

func (l *ldbIter) Next() bool    { return l.it.Next() }
func (l *ldbIter) Key() []byte   { return l.it.Key() }
func (l *ldbIter) Value() []byte { return l.it.Value() }
func (l *ldbIter) Release()      { l.it.Release() }
func (l *ldbIter) Error() error  { return l.it.Error() }

// mergedIterator is a 2-way merge; a = scratch (wins ties), b = source
// (masked keys skipped).
type mergedIterator struct {
	o    *Overlay
	a, b ethdb.Iterator

	aDone, bDone   bool
	aValid, bValid bool
	inited         bool

	key, value []byte
	err        error
}

func (m *mergedIterator) advanceA() {
	m.aValid = m.a.Next()
	if !m.aValid {
		m.aDone = true
	}
}

func (m *mergedIterator) advanceB() {
	for {
		m.bValid = m.b.Next()
		if !m.bValid {
			m.bDone = true
			return
		}
		m.o.mu.RLock()
		_, hidden := m.o.masked[string(m.b.Key())]
		m.o.mu.RUnlock()
		if !hidden {
			return
		}
	}
}

func (m *mergedIterator) Next() bool {
	if m.err != nil {
		return false
	}
	if !m.inited {
		m.inited = true
		m.advanceA()
		m.advanceB()
	} else {
		// Consume whichever side(s) produced the last key.
		switch {
		case m.aValid && m.bValid && bytes.Equal(m.a.Key(), m.b.Key()):
			m.advanceA()
			m.advanceB()
		case m.aValid && (!m.bValid || bytes.Compare(m.a.Key(), m.b.Key()) < 0):
			m.advanceA()
		case m.bValid:
			m.advanceB()
		}
	}
	switch {
	case m.aValid && m.bValid:
		c := bytes.Compare(m.a.Key(), m.b.Key())
		if c <= 0 {
			m.key, m.value = cp(m.a.Key()), cp(m.a.Value())
		} else {
			m.key, m.value = cp(m.b.Key()), cp(m.b.Value())
		}
		return true
	case m.aValid:
		m.key, m.value = cp(m.a.Key()), cp(m.a.Value())
		return true
	case m.bValid:
		m.key, m.value = cp(m.b.Key()), cp(m.b.Value())
		return true
	default:
		return false
	}
}

func cp(b []byte) []byte { return append([]byte(nil), b...) }

func (m *mergedIterator) Key() []byte   { return m.key }
func (m *mergedIterator) Value() []byte { return m.value }

func (m *mergedIterator) Release() {
	m.a.Release()
	m.b.Release()
}

func (m *mergedIterator) Error() error {
	if m.err != nil {
		return m.err
	}
	if err := m.a.Error(); err != nil {
		return err
	}
	return m.b.Error()
}
