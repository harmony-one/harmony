package audit

// Injected source read-error tests for side.Header and its two callers
// (checkPreconditions, CommitSigFor): an I/O failure on the canonical-hash
// key or the header record must surface as an error (exit 14 at the
// callers), never be misread as absence — absence reroutes CommitSigFor to
// the block-sig fallback and turns the precondition check into a spurious
// "missing header" invocation error (exit 15).

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// faultKV injects read errors on selected keys over a real memory store.
type faultKV struct {
	ethdb.KeyValueStore
	failOn func(key []byte) error
}

func (f faultKV) Has(key []byte) (bool, error) {
	if err := f.failOn(key); err != nil {
		return false, err
	}
	return f.KeyValueStore.Has(key)
}

func (f faultKV) Get(key []byte) ([]byte, error) {
	if err := f.failOn(key); err != nil {
		return nil, err
	}
	return f.KeyValueStore.Get(key)
}

func failKeys(bad ...[]byte) func([]byte) error {
	return func(key []byte) error {
		for _, b := range bad {
			if bytes.Equal(key, b) {
				return errors.New("injected read fault")
			}
		}
		return nil
	}
}

// childHeader builds a decodable header carrying LastCommit material.
func childHeader(t *testing.T, n uint64) (*block.Header, []byte) {
	t.Helper()
	var sig [96]byte
	for i := range sig {
		sig[i] = byte(i + 1)
	}
	h := blockfactory.ForTest.NewHeader(big.NewInt(3)).With().
		Number(new(big.Int).SetUint64(n)).
		LastCommitSignature(sig).
		LastCommitBitmap([]byte{0xff, 0x01}).
		Header()
	want := append(append([]byte(nil), sig[:]...), 0xff, 0x01)
	return h, want
}

func storeHeader(t *testing.T, mem ethdb.KeyValueStore, n uint64, h *block.Header) {
	t.Helper()
	raw, err := rlp.EncodeToBytes(h)
	if err != nil {
		t.Fatalf("encode header: %v", err)
	}
	if err := mem.Put(canonicalKey(n), h.Hash().Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(headerKey(n, h.Hash()), raw); err != nil {
		t.Fatal(err)
	}
}

func TestSideHeaderCanonicalReadFault(t *testing.T) {
	mem := memorydb.New()
	h, _ := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	sd := newSide(faultKV{mem, failKeys(canonicalKey(42))})
	if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "canonical hash 42 unreadable") {
		t.Fatalf("want canonical read fault, got %v", err)
	}
}

func TestSideHeaderRecordReadFault(t *testing.T) {
	mem := memorydb.New()
	h, _ := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	sd := newSide(faultKV{mem, failKeys(headerKey(42, h.Hash()))})
	if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("want header record read fault, got %v", err)
	}
}

func TestSideHeaderUndecodable(t *testing.T) {
	mem := memorydb.New()
	h, _ := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	if err := mem.Put(headerKey(42, h.Hash()), []byte("garbage")); err != nil {
		t.Fatal(err)
	}
	sd := newSide(mem)
	if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "undecodable") {
		t.Fatalf("want decode error, got %v", err)
	}
}

func TestSideHeaderGenuineAbsence(t *testing.T) {
	sd := newSide(memorydb.New())
	h, err := sd.Header(42)
	if err != nil || h != nil {
		t.Fatalf("want (nil, nil) for genuine absence, got (%v, %v)", h, err)
	}
}

// A canonical mapping whose header record is missing is corruption, never
// absence: it must fail closed at Header, block the CommitSigFor fallback,
// and surface as exit 14 (not 15) from the precondition check.
func TestSideHeaderDanglingCanonicalMappingIsCorruption(t *testing.T) {
	mem := memorydb.New()
	h, _ := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	if err := mem.Delete(headerKey(42, h.Hash())); err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(blockSigKey(41), []byte("planted-fallback")); err != nil {
		t.Fatal(err)
	}
	sd := newSide(mem)
	if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "header record is missing") {
		t.Fatalf("want dangling-mapping corruption error, got %v", err)
	}
	if got, _, err := sd.CommitSigFor(41); err == nil || !strings.Contains(err.Error(), "header record is missing") {
		t.Fatalf("CommitSigFor must not fall back over a dangling mapping, got (%x, %v)", got, err)
	}
	res := &anchor.Resolved{Config: anchor.Config{TargetHeight: 41}}
	var errBuf strings.Builder
	if code := checkPreconditions(sd, res, 0, &errBuf); code != report.ExitIO {
		t.Fatalf("want exit %d (I/O/corruption), got %d (stderr: %s)", report.ExitIO, code, errBuf.String())
	}
}

// A canonical mapping value that is not exactly 32 bytes is corruption:
// BytesToHash would silently pad/truncate it into a plausible hash.
func TestSideHeaderMalformedCanonicalMapping(t *testing.T) {
	for name, val := range map[string][]byte{
		"short": bytes.Repeat([]byte{0xaa}, 31),
		"long":  bytes.Repeat([]byte{0xbb}, 33),
	} {
		mem := memorydb.New()
		if err := mem.Put(canonicalKey(42), val); err != nil {
			t.Fatal(err)
		}
		if err := mem.Put(blockSigKey(41), []byte("planted-fallback")); err != nil {
			t.Fatal(err)
		}
		sd := newSide(mem)
		if _, err := sd.CanonicalHash(42); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("%s: want malformed-mapping error from CanonicalHash, got %v", name, err)
		}
		if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("%s: want malformed-mapping error from Header, got %v", name, err)
		}
		if got, _, err := sd.CommitSigFor(41); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("%s: CommitSigFor must not fall back over a malformed mapping, got (%x, %v)", name, got, err)
		}
	}
}

// A header record that decodes to a hash other than the mapping it was
// found under fails closed at Header (exit 14 from the precondition
// check). CommitSigFor alone proceeds with the stored record's commit
// material — never the block-sig fallback — so the audit's cryptographic
// checks classify the tamper at the affected heights instead of masking
// it as an I/O abort.
func TestSideHeaderHashMismatchIsCorruption(t *testing.T) {
	mem := memorydb.New()
	real, _ := childHeader(t, 42)
	other, want := childHeader(t, 43) // different header, different hash
	raw, err := rlp.EncodeToBytes(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(canonicalKey(42), real.Hash().Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(headerKey(42, real.Hash()), raw); err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(blockSigKey(41), []byte("planted-fallback")); err != nil {
		t.Fatal(err)
	}
	sd := newSide(mem)
	if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "decodes to hash") {
		t.Fatalf("want hash-mismatch error, got %v", err)
	}
	res := &anchor.Resolved{Config: anchor.Config{TargetHeight: 41}}
	var errBuf strings.Builder
	if code := checkPreconditions(sd, res, 0, &errBuf); code != report.ExitIO {
		t.Fatalf("want exit %d for a tampered precondition header, got %d (stderr: %s)",
			report.ExitIO, code, errBuf.String())
	}
	got, ide, err := sd.CommitSigFor(41)
	if err != nil {
		t.Fatalf("CommitSigFor must classify identity tampering through the validity pipeline, got error %v", err)
	}
	if ide == nil {
		t.Fatal("CommitSigFor must surface the identity mismatch (mandatory finding), got nil")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("CommitSigFor must use the stored record's material (never the fallback): got %x want %x", got, want)
	}
}

// A header record that decodes cleanly and matches its own hash but claims
// a different height than the mapping it sits under is the same identity
// tampering: Header errors, preconditions exit 14, CommitSigFor proceeds
// with the stored material for the validity pipeline to convict.
func TestSideHeaderHeightMismatchIsCorruption(t *testing.T) {
	mem := memorydb.New()
	h, want := childHeader(t, 43) // header says height 43
	raw, err := rlp.EncodeToBytes(h)
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(canonicalKey(42), h.Hash().Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(headerKey(42, h.Hash()), raw); err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(blockSigKey(41), []byte("planted-fallback")); err != nil {
		t.Fatal(err)
	}
	sd := newSide(mem)
	if _, err := sd.Header(42); err == nil || !strings.Contains(err.Error(), "decodes to height") {
		t.Fatalf("want height-mismatch error, got %v", err)
	}
	res := &anchor.Resolved{Config: anchor.Config{TargetHeight: 41}}
	var errBuf strings.Builder
	if code := checkPreconditions(sd, res, 0, &errBuf); code != report.ExitIO {
		t.Fatalf("want exit %d for a tampered precondition header, got %d (stderr: %s)",
			report.ExitIO, code, errBuf.String())
	}
	got, ide, err := sd.CommitSigFor(41)
	if err != nil {
		t.Fatalf("CommitSigFor must classify identity tampering through the validity pipeline, got error %v", err)
	}
	if ide == nil {
		t.Fatal("CommitSigFor must surface the identity mismatch (mandatory finding), got nil")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("CommitSigFor must use the stored record's material (never the fallback): got %x want %x", got, want)
	}
}

func TestCommitSigForUsesChildHeader(t *testing.T) {
	mem := memorydb.New()
	h, want := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	sd := newSide(mem)
	got, ide, err := sd.CommitSigFor(41)
	if err != nil {
		t.Fatal(err)
	}
	if ide != nil {
		t.Fatalf("clean child must carry no identity error, got %v", ide)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("sigAndBitmap mismatch: got %x want %x", got, want)
	}
}

// A child-header read fault must abort CommitSigFor — never silently swap
// the signature source to the block-sig fallback (planted here to prove the
// fallback WOULD have answered).
func TestCommitSigForChildReadFaultNoFallback(t *testing.T) {
	mem := memorydb.New()
	h, _ := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	if err := mem.Put(blockSigKey(41), []byte("planted-fallback")); err != nil {
		t.Fatal(err)
	}
	for name, key := range map[string][]byte{
		"canonical": canonicalKey(42),
		"header":    headerKey(42, h.Hash()),
	} {
		sd := newSide(faultKV{mem, failKeys(key)})
		got, _, err := sd.CommitSigFor(41)
		if err == nil || !strings.Contains(err.Error(), "injected read fault") {
			t.Fatalf("%s fault: want propagated read error, got (%x, %v)", name, got, err)
		}
	}
}

func TestCommitSigForGenuineAbsenceFallsBack(t *testing.T) {
	mem := memorydb.New()
	if err := mem.Put(blockSigKey(41), []byte("stored-sig")); err != nil {
		t.Fatal(err)
	}
	sd := newSide(mem)
	got, ide, err := sd.CommitSigFor(41)
	if err != nil {
		t.Fatal(err)
	}
	if ide != nil {
		t.Fatalf("genuine absence must carry no identity error, got %v", ide)
	}
	if string(got) != "stored-sig" {
		t.Fatalf("want block-sig fallback, got %q", got)
	}
}

// A redirected canonical mapping — otherwise-valid block bytes stored under
// a wrong hash key — must fail block identity validation, carrying the
// decoded block for the pass runner to record and keep validating.
func TestSideBlockIdentityMismatch(t *testing.T) {
	mem := memorydb.New()
	db := rawdb.NewDatabase(mem)
	h, _ := childHeader(t, 42)
	blk := types.NewBlockWithHeader(h)
	if err := rawdb.WriteBlock(db, blk); err != nil {
		t.Fatal(err)
	}
	// Redirect: canonical(42) points at a wrong hash, with the true
	// header/body records copied under that wrong key.
	wrong := blk.Hash()
	wrong[0] ^= 0xff
	if err := mem.Put(canonicalKey(42), wrong.Bytes()); err != nil {
		t.Fatal(err)
	}
	hdrRaw, err := mem.Get(headerKey(42, blk.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(headerKey(42, wrong), hdrRaw); err != nil {
		t.Fatal(err)
	}
	bodyRaw, err := mem.Get(bodyKey(42, blk.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Put(bodyKey(42, wrong), bodyRaw); err != nil {
		t.Fatal(err)
	}
	sd := newSide(mem)
	_, err = sd.Block(42)
	var ide *identityError
	if !errors.As(err, &ide) {
		t.Fatalf("want *identityError from redirected mapping, got %v", err)
	}
	if ide.kind != "block" || ide.block == nil || ide.block.Hash() != blk.Hash() {
		t.Fatalf("identity error must carry the decoded block: %+v", ide)
	}
	if !strings.Contains(err.Error(), "decodes to hash") {
		t.Fatalf("unexpected identity error text: %v", err)
	}
}

func TestPreconditionsChildReadFaultIsExitIO(t *testing.T) {
	mem := memorydb.New()
	h, _ := childHeader(t, 42)
	storeHeader(t, mem, 42, h)
	sd := newSide(faultKV{mem, failKeys(canonicalKey(42))})
	res := &anchor.Resolved{Config: anchor.Config{TargetHeight: 41}}
	var errBuf strings.Builder
	if code := checkPreconditions(sd, res, 0, &errBuf); code != report.ExitIO {
		t.Fatalf("want exit %d (I/O), got %d (stderr: %s)", report.ExitIO, code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "injected read fault") {
		t.Fatalf("stderr must carry the read fault, got: %s", errBuf.String())
	}
}

func TestPreconditionsChildAbsentIsBadInvocation(t *testing.T) {
	sd := newSide(memorydb.New())
	res := &anchor.Resolved{Config: anchor.Config{TargetHeight: 41}}
	var errBuf strings.Builder
	if code := checkPreconditions(sd, res, 0, &errBuf); code != report.ExitBadInvocation {
		t.Fatalf("want exit %d (bad invocation), got %d", report.ExitBadInvocation, code)
	}
	if !strings.Contains(errBuf.String(), "no canonical header at 42") {
		t.Fatalf("unexpected stderr: %s", errBuf.String())
	}
}
