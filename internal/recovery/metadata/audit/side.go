package audit

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
)

// side is the unmasked read-only view of the source: the audit reads the
// original branch blocks (and reconciliation baselines) from here while
// the overlay presents the post-apply view to the chain.
type side struct {
	kv ethdb.KeyValueStore // strict read-only source adapter
	db ethdb.Database      // rawdb wrapper
}

func newSide(kv ethdb.KeyValueStore) *side {
	return &side{kv: kv, db: rawdb.NewDatabase(kv)}
}

// CanonicalHash implements sourceReader for the seed builder. It reads the
// raw canonical key through the strict adapter (never the fail-open
// rawdb.ReadCanonicalHash, which converts I/O errors into an empty hash —
// here a read error would silently truncate the seed's tombstone walk).
// A mapping value that is not exactly 32 bytes is corruption, never
// absence: BytesToHash would silently pad/truncate it into a plausible
// hash and reroute every downstream read.
func (s *side) CanonicalHash(n uint64) (common.Hash, error) {
	raw, found, err := s.Get(canonicalKey(n))
	if err != nil {
		return common.Hash{}, fmt.Errorf("canonical hash %d unreadable: %w", n, err)
	}
	if !found {
		return common.Hash{}, nil
	}
	if len(raw) != common.HashLength {
		return common.Hash{}, fmt.Errorf("canonical mapping %d malformed: %d bytes, want %d (corrupt source)",
			n, len(raw), common.HashLength)
	}
	return common.BytesToHash(raw), nil
}

// identityError reports a WELL-FORMED header or block record whose
// recomputed hash or claimed height contradicts the canonical mapping it
// was found under: stored-content tampering (or a redirected mapping)
// rather than a broken or incomplete read. It carries the decoded object.
// The default disposition is fail-closed (checkPreconditions turns any
// Header error into exit 14); the pass runner alone unwraps it — the
// mismatch is recorded as a MANDATORY source-identity validity failure
// (gating the audit to a non-zero exit via the known-bad cross-check)
// while validation continues over the decoded content, so the remaining
// ancestry/cryptographic/execution checks classify the tamper too instead
// of the run aborting as I/O and masking where it sits.
type identityError struct {
	kind   string // "header" or "block"
	n      uint64
	mapped common.Hash
	header *block.Header
	block  *types.Block
	reason string
}

func (e *identityError) Error() string {
	return fmt.Sprintf("%s record at %d %s %s (tampered, redirected or corrupt source)",
		e.kind, e.n, e.mapped.Hex(), e.reason)
}

// Header reads the canonical header at n. (nil, nil) means EXACTLY one
// thing: no canonical mapping exists at n (genuine absence). Everything
// else fails closed with a non-nil error:
//
//   - read errors on the canonical key or the header record (strict
//     adapter; never the fail-open rawdb readers, which convert read
//     errors into nil and would let an unreadable child header masquerade
//     as absence, silently rerouting CommitSigFor to the block-sig
//     fallback or turning a precondition I/O failure into a "missing
//     header" invocation error);
//   - a malformed canonical mapping (not exactly 32 bytes);
//   - a canonical mapping whose header record is MISSING — a dangling
//     mapping is corruption, not absence;
//   - a header record that does not decode;
//   - a decoded header whose recomputed hash or claimed height contradicts
//     the mapping it sits under — reported as *headerIdentityError, which
//     still carries the decoded header for the one caller (CommitSigFor)
//     that must classify content tampering through the validity pipeline
//     instead of aborting.
func (s *side) Header(n uint64) (*block.Header, error) {
	hash, err := s.CanonicalHash(n)
	if err != nil {
		return nil, err
	}
	if hash == (common.Hash{}) {
		return nil, nil
	}
	raw, found, err := s.Get(headerKey(n, hash))
	if err != nil {
		return nil, fmt.Errorf("header %d %s unreadable: %w", n, hash.Hex(), err)
	}
	if !found {
		return nil, fmt.Errorf("canonical mapping %d points at %s but the header record is missing (corrupt source)",
			n, hash.Hex())
	}
	header := new(block.Header)
	if err := rlp.Decode(bytes.NewReader(raw), header); err != nil {
		return nil, fmt.Errorf("header %d %s undecodable: %w", n, hash.Hex(), err)
	}
	if got := header.Hash(); got != hash {
		return nil, &identityError{kind: "header", n: n, mapped: hash, header: header,
			reason: fmt.Sprintf("decodes to hash %s", got.Hex())}
	}
	if header.Number() == nil || !header.Number().IsUint64() || header.Number().Uint64() != n {
		return nil, &identityError{kind: "header", n: n, mapped: hash, header: header,
			reason: fmt.Sprintf("decodes to height %v", header.Number())}
	}
	return header, nil
}

// Block reads the canonical block at n and verifies its identity: the
// decoded content must reproduce the mapped hash and claim height n. A
// mismatch — a redirected canonical mapping over otherwise-valid bytes
// would pass ancestry and cryptographic checks unnoticed — is returned as
// *identityError carrying the decoded block, which the pass runner records
// as a mandatory source-identity validity failure before continuing.
func (s *side) Block(n uint64) (*types.Block, error) {
	hash, err := s.CanonicalHash(n)
	if err != nil {
		return nil, err
	}
	if hash == (common.Hash{}) {
		return nil, fmt.Errorf("no canonical hash at height %d in the source", n)
	}
	b := rawdb.ReadBlock(s.db, hash, n)
	if b == nil {
		return nil, fmt.Errorf("block %d %s not readable from the source", n, hash.Hex())
	}
	if got := b.Hash(); got != hash {
		return nil, &identityError{kind: "block", n: n, mapped: hash, block: b,
			reason: fmt.Sprintf("decodes to hash %s", got.Hex())}
	}
	if b.NumberU64() != n {
		return nil, &identityError{kind: "block", n: n, mapped: hash, block: b,
			reason: fmt.Sprintf("decodes to height %d", b.NumberU64())}
	}
	return b, nil
}

// CommitSigFor returns sigAndBitmap covering block n: the child header's
// LastCommit fields, or the exact block-sig-N key when no child GENUINELY
// exists. A child-header read/decode error is propagated, never treated as
// absence — silently falling back to block-sig-N on an I/O failure would
// swap the signature source without anyone noticing.
//
// A child record whose hash/height contradicts its mapping (stored-content
// tampering or a redirected mapping — *identityError) does NOT abort:
// the commit material extracted here is UNTRUSTED input to the audit's
// cryptographic header-signature and seal checks, which classify such
// tampering at the affected heights. The identity error itself is returned
// alongside the material so the pass runner records it as a MANDATORY
// source-identity validity failure — it can never be discarded, even when
// the tampered record still carries commit material that verifies (e.g. a
// redirected mapping over a copy of the true child).
func (s *side) CommitSigFor(n uint64) ([]byte, *identityError, error) {
	child, err := s.Header(n + 1)
	var ide *identityError
	if err != nil {
		if !errors.As(err, &ide) || ide.header == nil {
			return nil, nil, fmt.Errorf("commit signature source for block %d: child %w", n, err)
		}
		child = ide.header
	}
	if child != nil {
		sig := child.LastCommitSignature()
		bitmap := child.LastCommitBitmap()
		if len(bitmap) > 0 {
			return append(append([]byte(nil), sig[:]...), bitmap...), ide, nil
		}
	}
	raw, err := s.kv.Get(blockSigKey(n))
	if err != nil {
		return nil, nil, fmt.Errorf("no commit signature source for block %d (no child header, no block-sig key): %w", n, err)
	}
	return raw, ide, nil
}

// CrossLink reads the stored crosslink for (shardID, blockNum) from the raw
// source (nil when absent). Restoration source for legacy Copy-bug bitmap
// repair — see legacybitmap.go.
func (s *side) CrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error) {
	raw, found, err := s.Get(crosslinkKey(shardID, blockNum))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return types.DeserializeCrossLink(raw)
}

// Get reads a raw source key. found=false with a nil error means genuine
// absence; a non-nil error is a real I/O failure that must surface as exit
// 14, never as absence/corruption/anomaly.
func (s *side) Get(key []byte) (val []byte, found bool, err error) {
	has, err := s.kv.Has(key)
	if err != nil {
		return nil, false, err
	}
	if !has {
		return nil, false, nil
	}
	v, err := s.kv.Get(key)
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// HeadHeight resolves the source head height via LastHeader.
func (s *side) HeadHeight() (uint64, error) {
	raw, err := s.kv.Get([]byte("LastHeader"))
	if err != nil {
		return 0, fmt.Errorf("source LastHeader unreadable: %w", err)
	}
	num := rawdb.ReadHeaderNumber(s.db, common.BytesToHash(raw))
	if num == nil {
		return 0, fmt.Errorf("source LastHeader %x has no number mapping", raw)
	}
	return *num, nil
}
