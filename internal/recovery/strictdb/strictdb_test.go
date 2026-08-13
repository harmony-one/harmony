package strictdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
)

func snapKey(addr common.Address, epoch *big.Int) []byte {
	k := append([]byte("validator-snapshot"), addr.Bytes()...)
	return append(k, epoch.Bytes()...)
}

func TestClassifyNamespaces(t *testing.T) {
	addr := common.HexToAddress("0x00112233445566778899aabbccddeeff00112233")
	u64 := func(n uint64) []byte { b := make([]byte, 8); binary.BigEndian.PutUint64(b, n); return b }
	u32 := func(n uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, n); return b }
	cl := func(shard uint32, num uint64) []byte { return append(append([]byte("cl"), u32(shard)...), u64(num)...) }

	cases := []struct {
		name   string
		key    []byte
		wantNS Namespace
		check  func(Meta) bool
	}{
		{"validator-list", []byte("validator-list"), NsValidatorList, nil},
		{"dvl", append([]byte("dvl"), addr.Bytes()...), NsDVL, func(m Meta) bool { return bytes.Equal(m.Address, addr.Bytes()) }},
		{"snapshot-2byte-epoch", snapKey(addr, big.NewInt(3002)), NsValidatorSnapshot, func(m Meta) bool {
			return m.Epoch.Uint64() == 3002 && m.CanonicalEpochSuffix
		}},
		{"snapshot-epoch-0", snapKey(addr, big.NewInt(0)), NsValidatorSnapshot, func(m Meta) bool {
			return m.Epoch.Sign() == 0 && m.CanonicalEpochSuffix
		}},
		{"ss", append([]byte("ss"), big.NewInt(3002).Bytes()...), NsShardState, func(m Meta) bool { return m.Epoch.Uint64() == 3002 }},
		{"stats", append([]byte("validator-stats"), addr.Bytes()...), NsValidatorStats, nil},
		{"blk-rwd", append([]byte("blk-rwd-"), u64(92730034)...), NsBlockRewardAccum, func(m Meta) bool { return m.Number == 92730034 }},
		{"block-sig", append([]byte("block-sig-"), u64(5)...), NsBlockCommitSig, func(m Meta) bool { return m.Number == 5 }},
		{"LastCommits", []byte("LastCommits"), NsLastCommits, nil},
		{"pendingCL", []byte("pendingCL"), NsPendingCrossLink, nil},
		{"crosslink-record", cl(1, 42), NsCrossLink, func(m Meta) bool { return m.ShardID == 1 && m.Number == 42 }},
		{"crosslink-pointer", append([]byte("cl"), u32(1)...), NsCrossLinkPointer, func(m Meta) bool { return m.ShardID == 1 }},
		{"cxReceiptSpent", append(append([]byte("cxReceiptSpent"), u32(1)...), u64(7)...), NsCXReceiptSpent, func(m Meta) bool { return m.ShardID == 1 && m.Number == 7 }},
		{"LastHeader", []byte("LastHeader"), NsHead, nil},
		{"LastPivot", []byte("LastPivot"), NsSyncEra, nil},
		{"continuous", []byte("continuous"), NsLeaderContinuous, nil},
		{"epoch-vrf", append([]byte("epoch-vrf-block-numbers"), big.NewInt(3003).Bytes()...), NsEpochVRF, func(m Meta) bool { return m.Epoch.Uint64() == 3003 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ns, meta := Classify(c.key)
			if ns != c.wantNS {
				t.Fatalf("Classify(%x) ns = %s, want %s", c.key, ns, c.wantNS)
			}
			if c.check != nil && !c.check(meta) {
				t.Fatalf("Classify(%x) meta check failed: %+v", c.key, meta)
			}
		})
	}
}

func TestClassifyNoncanonicalEpochSuffix(t *testing.T) {
	// Leading-zero alias for epoch 3002 (0x0BBA) => 0x000BBA.
	alias := append([]byte("ss"), 0x00, 0x0b, 0xba)
	ns, meta := Classify(alias)
	if ns != NsShardState {
		t.Fatalf("ns = %s", ns)
	}
	if meta.CanonicalEpochSuffix {
		t.Fatal("leading-zero suffix must be flagged noncanonical")
	}
	if meta.Epoch.Uint64() != 3002 {
		t.Fatalf("parsed epoch %s, want 3002", meta.Epoch)
	}
}

func TestForEachChecksIteratorError(t *testing.T) {
	mem := memorydb.New()
	_ = mem.Put([]byte("dvl\x01"), []byte("v1"))
	_ = mem.Put([]byte("dvl\x02"), []byte("v2"))
	// Wrap in an iteratee that injects an error on exhaustion.
	it := &erroringIteratee{inner: mem, failAfter: 1}
	var seen int
	err := ForEach(it, []byte("dvl"), func(k, v []byte) error { seen++; return nil })
	if err == nil {
		t.Fatal("ForEach must surface a latched iterator error")
	}
}

// erroringIteratee returns an iterator that reports an error after
// failAfter keys.
type erroringIteratee struct {
	inner     ethdb.KeyValueStore
	failAfter int
}

func (e *erroringIteratee) NewIterator(prefix, start []byte) ethdb.Iterator {
	return &erroringIter{inner: e.inner.NewIterator(prefix, start), failAfter: e.failAfter}
}

type erroringIter struct {
	inner   ethdb.Iterator
	failAfter int
	seen    int
}

func (i *erroringIter) Next() bool {
	if i.seen >= i.failAfter {
		return false
	}
	i.seen++
	return i.inner.Next()
}
func (i *erroringIter) Key() []byte   { return i.inner.Key() }
func (i *erroringIter) Value() []byte { return i.inner.Value() }
func (i *erroringIter) Release()      { i.inner.Release() }
func (i *erroringIter) Error() error  { return errors.New("injected iterator error") }
