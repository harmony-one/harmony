// Package verify implements the shared verification machinery of
// harmony-recovery-db: the standalone BLS certificate verifier, the
// purpose-built state traversal, DigestSet computation, the logical KV
// digest, the recovery-marker schema, the normalized-metadata convergence
// proof, and the deep read-only verifier of the compact validator artifact
// (plan WS6).
package verify

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// Check IDs (referenced by the acceptance tests).
const (
	CheckGenesis            = "raw.genesis"
	CheckHeads              = "raw.heads"
	CheckAnchorIdentity     = "raw.anchor.identity"
	CheckAnchorShardState   = "raw.anchor.shard-state-digest"
	CheckMarkerPresent      = "raw.marker.present"
	CheckMarkerAnchor       = "raw.marker.anchor-digest"
	CheckMarkerReference    = "raw.marker.reference-mode"
	CheckMarkerToolIdentity = "raw.marker.tool-identity"
	CheckMarkerNormalized   = "raw.marker.normalized-digest"
	CheckMarkerLogical      = "raw.marker.logical-digest"
	CheckCanonicalTarget    = "raw.canonical.target"
	CheckAboveTarget        = "raw.above-target"
	CheckWindowContinuity   = "raw.window.continuity"
	CheckWindowRoots        = "raw.window.roots"
	CheckWindowCerts        = "raw.window.certs"
	CheckWindowLookups      = "raw.window.lookups"
	CheckForks              = "raw.forks"
	CheckAbandonedChild     = "raw.known-bad.abandoned-child"
	CheckRejectedShard1     = "raw.known-bad.rejected-shard1"
	CheckRuntimeMarkers     = "raw.runtime-markers"
	CheckPendingQueues      = "raw.pending-queues"
	CheckValidatorStats     = "raw.validator-stats"
	CheckBareHash32         = "raw.bare-hash32"
	CheckCodeOrphans        = "raw.code-orphans"
	CheckPreimagePolicy     = "raw.preimage-policy"
	CheckBloomPolicy        = "raw.bloom-policy"
	CheckMalformed          = "raw.malformed-keys"
	CheckStateOpen          = "state.open"
	CheckStateTraversal     = "state.traversal"
	CheckStateReopen        = "state.reopen"
	CheckDigestMatch        = "digestset.match"
	CheckValidatorListState = "offchain.validator-list-vs-state"
	CheckDelegationsState   = "offchain.delegations-vs-state"
	CheckEpochBounds        = "offchain.epoch-bounds"
	CheckVRFReferences      = "offchain.vrf-references"
	CheckLogicalDigest      = "logical.digest-match"
)

// Params configures a deep verification run.
type Params struct {
	Network     string
	ShardID     uint32
	ChainConfig *params.ChainConfig

	Anchor       *anchor.Manifest
	AnchorSHA256 string

	// Compact is the checksum-gated --source-reference (compact.json).
	Compact *report.CompactReport

	// MetadataReferenceManifestPath is the optional in-place manifest; must
	// be supplied iff Compact.Mode == "reference" (plan WS6).
	MetadataReferenceManifestPath string

	Window anchor.Window

	// TargetIsEpochLast is true when the target is its epoch's last block
	// (computed by the caller from the schedule); shard states/snapshots
	// for epoch target+1 are then legitimate.
	TargetIsEpochLast bool

	// TempDir hosts the reachable-set scratch database for the bare-hash32
	// orphan proof.
	TempDir string
}

// Result is the outcome of a deep verification.
type Result struct {
	Checks               []report.Check
	Passed               bool
	DigestSet            *report.DigestSet
	Logical              *LogicalDigest
	NormalizedOutput     string
	CertificatesVerified uint64
}

type runner struct {
	db     ethdb.Database
	p      Params
	checks []report.Check
	failed bool

	reach *leveldb.DB // reachable trie-node set (scratch)

	expectedCode map[string]map[common.Hash]bool // location -> set
	seenCode     map[string]map[common.Hash]bool // from raw scan

	targetHash common.Hash
	stateRoot  common.Hash

	txLookups, cxLookups uint64 // counted during raw scan
	windowTxs, windowCxs uint64 // counted during window walk
	seenTxLookup         map[common.Hash]bool
	seenCxLookup         map[common.Hash]bool

	logical    *LogicalDigest
	vrfScanErr error // undecodable epoch-VRF record seen during the raw scan

	preimagesAllowed bool
	statsAllowed     bool
	maxEpoch         uint64
}

func (r *runner) check(id string, err error) bool {
	if err != nil {
		r.checks = append(r.checks, report.Check{ID: id, OK: false, Detail: err.Error()})
		r.failed = true
		return false
	}
	r.checks = append(r.checks, report.Check{ID: id, OK: true})
	return true
}

func (r *runner) fail(id, format string, args ...interface{}) {
	r.checks = append(r.checks, report.Check{ID: id, OK: false, Detail: fmt.Sprintf(format, args...)})
	r.failed = true
}

func (r *runner) ok(id string) {
	r.checks = append(r.checks, report.Check{ID: id, OK: true})
}

// Run performs the full deep verification (plan §11.1-§11.4) over an
// already-open read-only database handle. It returns environmental errors
// (IO, scratch space) as error; verification findings land in
// Result.Checks/Passed.
func Run(db ethdb.Database, p Params) (*Result, error) {
	if err := p.Compact.DigestSet.Validate(); err != nil {
		return nil, fmt.Errorf("verify: source-reference DigestSet: %w", err)
	}
	r := &runner{
		db: db, p: p,
		expectedCode: map[string]map[common.Hash]bool{
			CodeLocPrefixed: {}, CodeLocValidator: {}, CodeLocLegacy: {},
		},
		seenCode: map[string]map[common.Hash]bool{
			CodeLocPrefixed: {}, CodeLocValidator: {}, CodeLocLegacy: {},
		},
		seenTxLookup:     map[common.Hash]bool{},
		seenCxLookup:     map[common.Hash]bool{},
		preimagesAllowed: p.Compact.Counts["preimages"] > 0,
		statsAllowed:     p.Compact.ValidatorStatsIncluded,
	}
	r.targetHash = p.Anchor.TargetHash
	// Epoch bound: shard states/snapshots for epoch target+1 may exist only
	// if the target is its epoch's last block (plan §11.3).
	r.maxEpoch = p.Window.Epoch
	if p.TargetIsEpochLast {
		r.maxEpoch = p.Window.Epoch + 1
	}

	scratch, err := os.MkdirTemp(p.TempDir, "recoverydb-reach-*")
	if err != nil {
		return nil, fmt.Errorf("verify: scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)
	r.reach, err = leveldb.OpenFile(scratch, &opt.Options{WriteBuffer: 64 * 1024 * 1024})
	if err != nil {
		return nil, fmt.Errorf("verify: open scratch reachable-set db: %w", err)
	}
	defer r.reach.Close()

	// Phase A: heads, marker, genesis, canonical-at-target.
	r.phaseHeadsAndMarker()
	// Phase B: full state traversal from the target root (also feeds the
	// reachable set and expected code locations).
	stateRes := r.phaseState()
	// Phase C: single lexical scan of the whole keyspace — logical digest,
	// per-bucket shape/height checks, orphan probes.
	if err := r.phaseRawScan(); err != nil {
		return nil, err
	}
	// Phase D: canonical window walk — continuity, roots, certificates,
	// per-tx lookups.
	certs := r.phaseWindow()
	// Phase E: DigestSet comparison + validator/delegation/VRF checks.
	var digestSet *report.DigestSet
	if stateRes != nil {
		off, err := ComputeOffchainDigests(r.db, report.DigestWindow{RetainFrom: p.Window.RetainFrom, Target: p.Window.Target})
		if err != nil {
			r.fail(CheckDigestMatch, "off-chain digest pass failed: %v", err)
		} else {
			digestSet = BuildDigestSet(p.Network, p.ShardID, p.Window.Target, r.targetHash, r.stateRoot,
				report.DigestWindow{RetainFrom: p.Window.RetainFrom, Target: p.Window.Target}, stateRes, off)
			if diffs := digestSet.Diff(p.Compact.DigestSet); len(diffs) > 0 {
				r.fail(CheckDigestMatch, "DigestSet differs from --source-reference: %v", diffs)
			} else {
				r.ok(CheckDigestMatch)
			}
		}
		r.phaseStakingCrossChecks()
	}
	// Phase F: marker digest cross-checks (logical + normalized).
	normalized := r.phaseMarkerDigests()

	return &Result{
		Checks:               r.checks,
		Passed:               !r.failed,
		DigestSet:            digestSet,
		Logical:              r.logical,
		NormalizedOutput:     normalized,
		CertificatesVerified: certs,
	}, nil
}

func (r *runner) phaseHeadsAndMarker() {
	// Anchor identity: the manifest's network/shard/epoch must match the
	// run's CLI values and the schedule-derived window epoch (round 13
	// finding 3 — a mainnet anchor must not verify under --network testnet).
	switch {
	case r.p.Anchor.Network != r.p.Network:
		r.fail(CheckAnchorIdentity, "anchor network %q != run network %q", r.p.Anchor.Network, r.p.Network)
	case r.p.Anchor.ShardID != r.p.ShardID:
		r.fail(CheckAnchorIdentity, "anchor shard %d != run shard %d", r.p.Anchor.ShardID, r.p.ShardID)
	case r.p.Anchor.TargetEpoch != r.p.Window.Epoch:
		r.fail(CheckAnchorIdentity, "anchor target_epoch %d != schedule epoch %d for target %d",
			r.p.Anchor.TargetEpoch, r.p.Window.Epoch, r.p.Window.Target)
	default:
		r.ok(CheckAnchorIdentity)
	}

	// All four heads == pinned target hash (plan §2.2.4, round 6 finding 2).
	headKeys := [][]byte{keys.HeadBlockKey, keys.HeadHeaderKey, keys.HeadFastBlockKey, keys.HeadFinalizedKey}
	headsOK := true
	for _, hk := range headKeys {
		val, found, err := hasThenGet(r.db, hk)
		if err != nil {
			r.fail(CheckHeads, "read head %s: %v", hk, err)
			headsOK = false
			continue
		}
		if !found {
			r.fail(CheckHeads, "head key %s absent", hk)
			headsOK = false
			continue
		}
		if common.BytesToHash(val) != r.targetHash {
			r.fail(CheckHeads, "head %s = %x, want pinned target %s", hk, val, r.targetHash.Hex())
			headsOK = false
		}
	}
	if headsOK {
		r.ok(CheckHeads)
	}

	// Recovery marker present with every §2.2.4 field checked.
	marker, err := ReadMarker(r.db)
	if !r.check(CheckMarkerPresent, err) {
		return
	}
	if marker.AnchorManifestSHA256 != r.p.AnchorSHA256 {
		r.fail(CheckMarkerAnchor, "marker anchor digest %s != anchor manifest %s", marker.AnchorManifestSHA256, r.p.AnchorSHA256)
	} else if marker.TargetHeight != r.p.Anchor.TargetHeight || marker.TargetHash != r.targetHash.Hex() {
		r.fail(CheckMarkerAnchor, "marker target (%d,%s) != anchor (%d,%s)",
			marker.TargetHeight, marker.TargetHash, r.p.Anchor.TargetHeight, r.targetHash.Hex())
	} else if marker.Network != r.p.Network || marker.ShardID != r.p.ShardID {
		r.fail(CheckMarkerAnchor, "marker identity (%s, shard %d) != run (%s, shard %d)",
			marker.Network, marker.ShardID, r.p.Network, r.p.ShardID)
	} else {
		r.ok(CheckMarkerAnchor)
	}

	// Mode-aware reference check (plan WS6, round 8 finding 1, revision 11).
	switch r.p.Compact.Mode {
	case report.ModeInternal:
		if r.p.MetadataReferenceManifestPath != "" {
			r.fail(CheckMarkerReference, "internal-mode build verified with a --metadata-reference-manifest; mode mismatch is fatal")
		} else if marker.MetadataReferenceDigest != MetadataReferenceInternalNone {
			r.fail(CheckMarkerReference, "internal mode requires the %q sentinel, marker has %q", MetadataReferenceInternalNone, marker.MetadataReferenceDigest)
		} else if r.p.Compact.MetadataReferenceDigest != MetadataReferenceInternalNone {
			r.fail(CheckMarkerReference, "compact.json records mode internal but reference digest %q", r.p.Compact.MetadataReferenceDigest)
		} else {
			r.ok(CheckMarkerReference)
		}
	case report.ModeReference:
		if r.p.MetadataReferenceManifestPath == "" {
			r.fail(CheckMarkerReference, "reference-mode build cannot be verified without --metadata-reference-manifest; mode mismatch is fatal")
		} else {
			manifest, sum, err := LoadMetadataReferenceManifest(r.p.MetadataReferenceManifestPath)
			if err != nil {
				r.fail(CheckMarkerReference, "%v", err)
			} else if marker.MetadataReferenceDigest != sum {
				r.fail(CheckMarkerReference, "marker reference digest %s != manifest %s", marker.MetadataReferenceDigest, sum)
			} else if r.p.Compact.MetadataReferenceDigest != sum {
				r.fail(CheckMarkerReference, "compact.json reference digest %s != manifest %s", r.p.Compact.MetadataReferenceDigest, sum)
			} else {
				// Independent per-section convergence proof, re-established
				// from scratch (plan WS6 §11.1).
				artifact, err := ComputeNormalizedSections(r.db)
				if err != nil {
					r.fail(CheckMarkerReference, "recompute normalized sections: %v", err)
				} else if diffs := CompareNormalizedSections(manifest, artifact); len(diffs) > 0 {
					r.fail(CheckMarkerReference, "normalized-metadata convergence failed: %v", diffs)
				} else {
					r.ok(CheckMarkerReference)
				}
			}
		}
	default:
		r.fail(CheckMarkerReference, "compact.json records unknown mode %q", r.p.Compact.Mode)
	}

	// Tool identity: exact equality against compact.json (round 9 finding 2).
	if marker.ToolVersion != r.p.Compact.ToolVersion || marker.ToolBinarySHA256 != r.p.Compact.ToolBinary {
		r.fail(CheckMarkerToolIdentity, "marker tool identity (%s, %s) != compact.json producer (%s, %s)",
			marker.ToolVersion, marker.ToolBinarySHA256, r.p.Compact.ToolVersion, r.p.Compact.ToolBinary)
	} else {
		r.ok(CheckMarkerToolIdentity)
	}

	// Genesis identity + canonical mapping and inverse at target.
	genHash := rawdb.ReadCanonicalHash(r.db, 0)
	if genHash == (common.Hash{}) {
		r.fail(CheckGenesis, "no canonical genesis mapping")
	} else if r.p.Anchor.GenesisHash != (common.Hash{}) && genHash != r.p.Anchor.GenesisHash {
		r.fail(CheckGenesis, "genesis hash %s != anchor %s", genHash.Hex(), r.p.Anchor.GenesisHash.Hex())
	} else if gh := rawdb.ReadHeader(r.db, genHash, 0); gh == nil {
		r.fail(CheckGenesis, "genesis header missing")
	} else if gh.ShardID() != r.p.ShardID {
		r.fail(CheckGenesis, "genesis shard %d != %d", gh.ShardID(), r.p.ShardID)
	} else {
		r.ok(CheckGenesis)
	}

	tgt := r.p.Window.Target
	ch := rawdb.ReadCanonicalHash(r.db, tgt)
	if ch != r.targetHash {
		r.fail(CheckCanonicalTarget, "canonical(%d) = %s, want %s", tgt, ch.Hex(), r.targetHash.Hex())
		return
	}
	if num := rawdb.ReadHeaderNumber(r.db, r.targetHash); num == nil || *num != tgt {
		r.fail(CheckCanonicalTarget, "inverse mapping for target hash wrong (got %v)", num)
		return
	}
	hdr := rawdb.ReadHeader(r.db, r.targetHash, tgt)
	if hdr == nil {
		r.fail(CheckCanonicalTarget, "target header missing")
		return
	}
	if r.p.Anchor.TargetParentHash != (common.Hash{}) && hdr.ParentHash() != r.p.Anchor.TargetParentHash {
		r.fail(CheckCanonicalTarget, "target parent %s != anchor %s", hdr.ParentHash().Hex(), r.p.Anchor.TargetParentHash.Hex())
		return
	}
	// Every filled anchor identity field is verified against the target
	// header (round 13 finding 3). TargetEpoch is always pinned;
	// TargetViewID is FILL_AND_VERIFY (0 = unfilled).
	if hdr.Epoch().Uint64() != r.p.Anchor.TargetEpoch {
		r.fail(CheckCanonicalTarget, "target header epoch %d != anchor target_epoch %d", hdr.Epoch().Uint64(), r.p.Anchor.TargetEpoch)
		return
	}
	if r.p.Anchor.TargetViewID != 0 && hdr.ViewID().Uint64() != r.p.Anchor.TargetViewID {
		r.fail(CheckCanonicalTarget, "target header view ID %d != anchor target_view_id %d", hdr.ViewID().Uint64(), r.p.Anchor.TargetViewID)
		return
	}
	r.stateRoot = hdr.Root()
	if r.p.Compact.StateRoot != "" && r.p.Compact.StateRoot != r.stateRoot.Hex() {
		r.fail(CheckCanonicalTarget, "target state root %s != compact.json %s", r.stateRoot.Hex(), r.p.Compact.StateRoot)
		return
	}
	if r.p.Anchor.TargetStateRoot != (common.Hash{}) && hdr.Root() != r.p.Anchor.TargetStateRoot {
		r.fail(CheckCanonicalTarget, "target state root %s != anchor %s", hdr.Root().Hex(), r.p.Anchor.TargetStateRoot.Hex())
		return
	}
	r.ok(CheckCanonicalTarget)

	// FILL_AND_VERIFY shard-state digest: SHA-256 over the exact
	// ss<target-epoch> value bytes (round 13 finding 3; empty = unfilled,
	// same convention as the other FILL_AND_VERIFY fields).
	if r.p.Anchor.ShardStateDigestSHA256 != "" {
		ssKey := keys.ShardStateKey(new(big.Int).SetUint64(r.p.Anchor.TargetEpoch))
		val, found, err := hasThenGet(r.db, ssKey)
		switch {
		case err != nil:
			r.fail(CheckAnchorShardState, "read ss<%d>: %v", r.p.Anchor.TargetEpoch, err)
		case !found:
			r.fail(CheckAnchorShardState, "shard state ss<%d> absent but anchor pins its digest", r.p.Anchor.TargetEpoch)
		default:
			if sum := integrity.BytesSHA256(val); sum != r.p.Anchor.ShardStateDigestSHA256 {
				r.fail(CheckAnchorShardState, "ss<%d> sha256 %s != anchor %s", r.p.Anchor.TargetEpoch, sum, r.p.Anchor.ShardStateDigestSHA256)
			} else {
				r.ok(CheckAnchorShardState)
			}
		}
	}
}

func (r *runner) phaseState() *StateWalkResult {
	if r.stateRoot == (common.Hash{}) {
		r.fail(CheckStateOpen, "no target state root (earlier checks failed)")
		return nil
	}
	if _, err := state.New(r.stateRoot, state.NewDatabase(r.db), nil); err != nil {
		r.fail(CheckStateOpen, "state.New(%s): %v", r.stateRoot.Hex(), err)
		return nil
	}
	r.ok(CheckStateOpen)

	batch := &reachBatch{db: r.reach}
	res, err := WalkState(r.db, r.stateRoot, StateWalkOptions{
		CheckPreimages: false,
		OnNode:         batch.add,
		OnCode: func(codeHash common.Hash, location string, code []byte) error {
			r.expectedCode[location][codeHash] = true
			return nil
		},
	})
	if err != nil {
		r.fail(CheckStateTraversal, "%v", err)
		return nil
	}
	if err := batch.flush(); err != nil {
		r.fail(CheckStateTraversal, "reachable-set flush: %v", err)
		return nil
	}
	r.ok(CheckStateTraversal)
	return res
}

type reachBatch struct {
	db    *leveldb.DB
	batch leveldb.Batch
	n     int
}

func (b *reachBatch) add(h common.Hash) error {
	b.batch.Put(h.Bytes(), nil)
	b.n++
	if b.n >= 100000 {
		return b.flush()
	}
	return nil
}

func (b *reachBatch) flush() error {
	if b.n == 0 {
		return nil
	}
	if err := b.db.Write(&b.batch, nil); err != nil {
		return err
	}
	b.batch.Reset()
	b.n = 0
	return nil
}

// legalHeight reports whether a chain-table height belongs in the compact
// artifact: genesis or the retention window.
func (r *runner) legalHeight(num uint64) bool {
	return num == 0 || (num >= r.p.Window.RetainFrom && num <= r.p.Window.Target)
}

func (r *runner) phaseRawScan() error {
	total := report.NewHasher(LogicalDigestDomain)
	bucketHashers := map[string]*report.Hasher{}
	logical := &LogicalDigest{Buckets: map[string]report.Digest{}}

	var (
		aboveTargetErr, forkErr, runtimeErr, pendingErr, statsErr error
		bareUnresolved, malformed, bloomErr, preimageErr          error
		abandonedErr, rejectedErr, lookupErr                      error
	)
	setOnce := func(dst *error, format string, args ...interface{}) {
		if *dst == nil {
			*dst = fmt.Errorf(format, args...)
		}
	}

	abandoned := r.p.Anchor.AbandonedChildHash
	rejected := r.p.Anchor.RejectedShard1Hash
	expBloomCount, expBloomHead, expBloomOK := anchor.BloomCheckpoint(r.p.Window)

	// The preimage-generation bookkeeping pair is digest-excluded (see
	// DigestExcludedKey), with presence and VALUES pinned separately after
	// the scan by validatePreimageMarkers (rounds 14-16).
	var preimageMarkers preimageMarkerState
	scanErr := strictdb.ForEach(r.db, nil, func(key, value []byte) error {
		if bytes.Equal(key, keys.PreimageGenStartKey) {
			preimageMarkers.startPresent = true
			preimageMarkers.start = append([]byte(nil), value...)
		}
		if bytes.Equal(key, keys.PreimageGenEndKey) {
			preimageMarkers.endPresent = true
			preimageMarkers.end = append([]byte(nil), value...)
		}
		// Single shared exclusion predicate — identical for compact's
		// marker write, package binding and this scan (round 15 finding 1).
		if DigestExcludedKey(key) {
			return nil // recovery-marker presence is checked separately
		}
		bucket := keys.Classify(key)
		h, ok := bucketHashers[bucket]
		if !ok {
			h = report.NewHasher("logical." + bucket)
			bucketHashers[bucket] = h
		}
		h.Add(key, value)
		total.Add(key, value)
		logical.TotalKeys++
		logical.TotalBytes += uint64(len(key) + len(value))

		switch bucket {
		case keys.BucketCanonical:
			num := binary.BigEndian.Uint64(key[1:9])
			if !r.legalHeight(num) {
				setOnce(&aboveTargetErr, "canonical mapping at height %d outside {0} ∪ window", num)
			}
		case keys.BucketHeader:
			num := binary.BigEndian.Uint64(key[1:9])
			hash := common.BytesToHash(key[9:41])
			if !r.legalHeight(num) {
				setOnce(&aboveTargetErr, "header at height %d outside {0} ∪ window", num)
			} else if canon := rawdb.ReadCanonicalHash(r.db, num); canon != hash {
				setOnce(&forkErr, "non-canonical fork header at height %d: %s (canonical %s)", num, hash.Hex(), canon.Hex())
			}
			if hash == abandoned {
				setOnce(&abandonedErr, "abandoned-child header present at height %d", num)
			}
			if hash == rejected {
				setOnce(&rejectedErr, "rejected shard-1 header present at height %d", num)
			}
		case keys.BucketTD:
			num := binary.BigEndian.Uint64(key[1:9])
			if !r.legalHeight(num) {
				setOnce(&aboveTargetErr, "TD record at height %d outside {0} ∪ window", num)
			}
		case keys.BucketBody, keys.BucketReceipts:
			num := binary.BigEndian.Uint64(key[1:9])
			hash := common.BytesToHash(key[9:41])
			if !r.legalHeight(num) {
				setOnce(&aboveTargetErr, "%s at height %d outside {0} ∪ window", bucket, num)
			} else if canon := rawdb.ReadCanonicalHash(r.db, num); canon != hash {
				setOnce(&forkErr, "non-canonical %s at height %d: %s", bucket, num, hash.Hex())
			}
		case keys.BucketHeaderNumber:
			hash := common.BytesToHash(key[1:33])
			num := binary.BigEndian.Uint64(value)
			if hash == abandoned {
				setOnce(&abandonedErr, "abandoned-child header-number entry present")
			}
			if hash == rejected {
				setOnce(&rejectedErr, "rejected shard-1 header-number entry present")
			}
			if !r.legalHeight(num) {
				setOnce(&aboveTargetErr, "header-number entry maps to height %d outside {0} ∪ window", num)
			} else if canon := rawdb.ReadCanonicalHash(r.db, num); canon != hash {
				setOnce(&forkErr, "header-number entry for non-canonical hash %s at %d", hash.Hex(), num)
			}
		case keys.BucketBlockSig:
			num := binary.BigEndian.Uint64(key[len(keys.BlockSigPrefix):])
			if num < r.p.Window.RetainFrom || num > r.p.Window.Target {
				setOnce(&aboveTargetErr, "block-sig-%d outside retention window", num)
			}
		case keys.BucketTxLookup:
			r.txLookups++
			var entry rawdb.LegacyTxLookupEntry
			if err := rlp.DecodeBytes(value, &entry); err != nil {
				setOnce(&lookupErr, "undecodable tx lookup %x: %v", key, err)
			} else if entry.BlockIndex < r.p.Window.RetainFrom || entry.BlockIndex > r.p.Window.Target {
				setOnce(&lookupErr, "tx lookup %x points outside window (height %d)", key[1:], entry.BlockIndex)
			} else if canon := rawdb.ReadCanonicalHash(r.db, entry.BlockIndex); canon != entry.BlockHash {
				setOnce(&lookupErr, "tx lookup %x points to non-canonical block %s at %d", key[1:], entry.BlockHash.Hex(), entry.BlockIndex)
			}
		case keys.BucketCxLookup:
			r.cxLookups++
			var entry rawdb.LegacyTxLookupEntry
			if err := rlp.DecodeBytes(value, &entry); err != nil {
				setOnce(&lookupErr, "undecodable cx lookup %x: %v", key, err)
			} else if entry.BlockIndex < r.p.Window.RetainFrom || entry.BlockIndex > r.p.Window.Target {
				setOnce(&lookupErr, "cx lookup points outside window (height %d)", entry.BlockIndex)
			} else if canon := rawdb.ReadCanonicalHash(r.db, entry.BlockIndex); canon != entry.BlockHash {
				setOnce(&lookupErr, "cx lookup points to non-canonical block at %d", entry.BlockIndex)
			}
		case keys.BucketCxReceipt:
			num := binary.BigEndian.Uint64(key[len(keys.CxReceiptPrefix)+4 : len(keys.CxReceiptPrefix)+12])
			if num < r.p.Window.RetainFrom || num > r.p.Window.Target {
				setOnce(&aboveTargetErr, "outgoing cxReceipt at source height %d outside window", num)
			}
		case keys.BucketCxSpent:
			// Keyed by foreign source shard + foreign height; heights above
			// the numeric target are legitimate (plan WS5 acceptance:
			// shard-1 CX fixture). Set equality is checked via DigestSet.
		case keys.BucketCrosslinkIndex, keys.BucketCrosslinkShardLast:
			if bytes.Contains(value, abandoned.Bytes()) {
				setOnce(&abandonedErr, "crosslink record references abandoned-child hash")
			}
			if bytes.Contains(value, rejected.Bytes()) {
				setOnce(&rejectedErr, "crosslink record references rejected shard-1 hash (anchor known-bad, in-place §2.5)")
			}
		case keys.BucketShardState:
			epoch := new(big.Int).SetBytes(key[len(keys.ShardStatePrefix):]).Uint64()
			if epoch > r.maxEpoch {
				setOnce(&aboveTargetErr, "shard state for epoch %d beyond allowed max %d", epoch, r.maxEpoch)
			}
		case keys.BucketValidatorSnapshot:
			epoch := new(big.Int).SetBytes(key[len(keys.ValidatorSnapshotPrefix)+20:]).Uint64()
			if epoch > r.maxEpoch {
				setOnce(&aboveTargetErr, "validator snapshot for epoch %d beyond allowed max %d", epoch, r.maxEpoch)
			}
		case keys.BucketEpochBlockNumber:
			epoch := new(big.Int).SetBytes(key[len(keys.EpochBlockNumberPrefix):]).Uint64()
			if epoch > r.maxEpoch {
				setOnce(&aboveTargetErr, "epoch-block-number record for epoch %d beyond allowed max %d", epoch, r.maxEpoch)
			}
		case keys.BucketEpochVrf:
			epoch := new(big.Int).SetBytes(key[len(keys.EpochVrfPrefix):]).Uint64()
			if epoch > r.maxEpoch {
				setOnce(&aboveTargetErr, "epoch VRF record for epoch %d beyond allowed max %d", epoch, r.maxEpoch)
			}
			// Every present record must decode (round 13 finding 7).
			nums := []uint64{}
			if err := rlp.DecodeBytes(value, &nums); err != nil && r.vrfScanErr == nil {
				r.vrfScanErr = fmt.Errorf("undecodable epoch-%d VRF record: %w", epoch, err)
			}
		case keys.BucketEpochVdf:
			epoch := new(big.Int).SetBytes(key[len(keys.EpochVdfPrefix):]).Uint64()
			if epoch > r.maxEpoch {
				setOnce(&aboveTargetErr, "epoch VDF record for epoch %d beyond allowed max %d", epoch, r.maxEpoch)
			}
		case keys.BucketRewardAccum:
			num := binary.BigEndian.Uint64(key[len(keys.RewardAccumPrefix):])
			if num < r.p.Window.RetainFrom || num > r.p.Window.Target {
				setOnce(&aboveTargetErr, "reward accumulator at height %d outside window", num)
			}
		case keys.BucketCode:
			r.seenCode[CodeLocPrefixed][common.BytesToHash(key[1:])] = true
		case keys.BucketValidatorCode:
			r.seenCode[CodeLocValidator][common.BytesToHash(key[2:])] = true
		case keys.BucketBareHash32:
			// Reachability: a bare key must be a reachable trie node or a
			// legacy code blob referenced by the target state. In the fresh
			// compact artifact any unresolved bare key is FATAL (plan
			// §2.2.9 severity split, round 6 finding 5).
			if _, err := r.reach.Get(key, nil); err == leveldb.ErrNotFound {
				if r.expectedCode[CodeLocLegacy][common.BytesToHash(key)] {
					r.seenCode[CodeLocLegacy][common.BytesToHash(key)] = true
				} else {
					setOnce(&bareUnresolved, "unresolved bare-hash32 key %x (neither reachable trie node nor referenced legacy code)", key)
				}
			} else if err != nil {
				return fmt.Errorf("verify: reachable-set probe: %w", err)
			}
		case keys.BucketMalformed:
			setOnce(&malformed, "malformed key %x", key)
		case keys.BucketBloomBits:
			setOnce(&bloomErr, "bloom-bits data key present (excluded namespace)")
		case keys.BucketBloomIndex:
			// The checkpoint must be exactly the advanceable one computed
			// by anchor.BloomCheckpoint (round 13 finding 4): the stored
			// count so the next section needs no pruned headers, and the
			// single section head pointing at the retained head block.
			payload := key[len(keys.BloomBitsIndexPrefix):]
			if bytes.Equal(payload, []byte("count")) {
				sections := binary.BigEndian.Uint64(value)
				if !expBloomOK {
					setOnce(&bloomErr, "bloom index count present but no checkpoint is defined for this window")
				} else if sections != expBloomCount {
					setOnce(&bloomErr, "bloom index count %d != expected checkpoint %d", sections, expBloomCount)
				}
			} else {
				s := binary.BigEndian.Uint64(payload[5:])
				switch {
				case !expBloomOK:
					setOnce(&bloomErr, "bloom section head present but no checkpoint is defined for this window")
				case s != expBloomCount-1:
					setOnce(&bloomErr, "bloom section head for section %d, expected only %d", s, expBloomCount-1)
				case rawdb.ReadCanonicalHash(r.db, expBloomHead) != common.BytesToHash(value):
					setOnce(&bloomErr, "bloom section head %x != canonical(%d)", value, expBloomHead)
				}
			}
		case keys.BucketPreimage:
			if !r.preimagesAllowed {
				setOnce(&preimageErr, "preimage key present but compact.json declares none copied")
			}
		case keys.BucketPendingCrosslink:
			setOnce(&pendingErr, "pendingCL key present (must be cleared/omitted — in-place §2.2 alignment)")
		case keys.BucketPendingSlashing:
			setOnce(&pendingErr, "pendingSC key present (must be cleared/omitted)")
		case keys.BucketValidatorStats:
			if !r.statsAllowed {
				setOnce(&statsErr, "validator-stats key present without recorded opt-in")
			}
		case keys.BucketSkeletonHeader:
			setOnce(&runtimeErr, "skeleton header present")
		case keys.BucketSnapAccount, keys.BucketSnapStorage:
			setOnce(&runtimeErr, "flat snapshot table key present")
		}

		// Meta keys: only the four heads, DatabaseVersion and the
		// leader-rotation continuous count are legitimate in the compact
		// artifact; every other singleton is a runtime/sync marker (plan
		// §10.6, §11.1 — including the legacy LastCommits fallback key,
		// whose presence would mask a missing exact block-sig-N).
		if len(bucket) > len(keys.BucketMetaPrefix) && bucket[:len(keys.BucketMetaPrefix)] == keys.BucketMetaPrefix {
			switch bucket[len(keys.BucketMetaPrefix):] {
			case "LastBlock", "LastHeader", "LastFast", "LastFinalized", "DatabaseVersion", "continuous":
			default:
				setOnce(&runtimeErr, "runtime/sync marker %q present", bucket)
			}
		}
		return nil
	})
	if scanErr != nil {
		return scanErr
	}
	if pairErr := validatePreimageMarkers(preimageMarkers, r.p.Window.Target); pairErr != nil {
		setOnce(&runtimeErr, "%v", pairErr)
	}

	names := make([]string, 0, len(bucketHashers))
	for name := range bucketHashers {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		logical.Buckets[name] = bucketHashers[name].Digest()
	}
	logical.Total = total.Digest()
	r.logical = logical

	r.check(CheckAboveTarget, aboveTargetErr)
	r.check(CheckForks, forkErr)
	r.check(CheckAbandonedChild, abandonedErr)
	r.check(CheckRejectedShard1, rejectedErr)
	r.check(CheckRuntimeMarkers, runtimeErr)
	r.check(CheckPendingQueues, pendingErr)
	r.check(CheckValidatorStats, statsErr)
	r.check(CheckBareHash32, bareUnresolved)
	r.check(CheckMalformed, malformed)
	r.check(CheckBloomPolicy, bloomErr)
	r.check(CheckPreimagePolicy, preimageErr)
	r.check(CheckWindowLookups, lookupErr)

	// Prefixed code orphans: every c/vc key must be referenced by the
	// target state (plan §11.1 "planted unreachable prefixed future key ⇒
	// fatal").
	var orphanErr error
	for _, loc := range []string{CodeLocPrefixed, CodeLocValidator} {
		for h := range r.seenCode[loc] {
			if !r.expectedCode[loc][h] {
				orphanErr = fmt.Errorf("orphan %s code key %s not referenced by target state", loc, h.Hex())
				break
			}
		}
		for h := range r.expectedCode[loc] {
			if !r.seenCode[loc][h] {
				orphanErr = fmt.Errorf("expected %s code key %s missing from raw scan", loc, h.Hex())
				break
			}
		}
	}
	r.check(CheckCodeOrphans, orphanErr)
	return nil
}

func (r *runner) phaseWindow() uint64 {
	if r.stateRoot == (common.Hash{}) {
		return 0
	}
	cv := NewCertVerifier(r.db, r.p.ChainConfig, r.p.ShardID)
	var (
		contErr, rootsErr, certErr, lookupErr error
		verified                              uint64
		prevHash                              common.Hash
	)
	setOnce := func(dst *error, format string, args ...interface{}) {
		if *dst == nil {
			*dst = fmt.Errorf(format, args...)
		}
	}

	for n := r.p.Window.RetainFrom; n <= r.p.Window.Target; n++ {
		ch := rawdb.ReadCanonicalHash(r.db, n)
		if ch == (common.Hash{}) {
			setOnce(&contErr, "canonical mapping missing at %d", n)
			break
		}
		hdr := rawdb.ReadHeader(r.db, ch, n)
		if hdr == nil {
			setOnce(&contErr, "header missing at %d", n)
			break
		}
		if hdr.Number().Uint64() != n {
			setOnce(&contErr, "header at %d claims number %d", n, hdr.Number().Uint64())
			break
		}
		if n > r.p.Window.RetainFrom && prevHash != (common.Hash{}) && hdr.ParentHash() != prevHash {
			setOnce(&contErr, "parent linkage broken at %d: parent %s, canonical(%d) %s", n, hdr.ParentHash().Hex(), n-1, prevHash.Hex())
			break
		}
		prevHash = ch

		body := rawdb.ReadBody(r.db, ch, n)
		if body == nil {
			setOnce(&contErr, "body missing at %d", n)
			break
		}
		// Roots recomputed (plan §11.1).
		txs := types.Transactions(body.Transactions())
		stxs := staking.StakingTransactions(body.StakingTransactions())
		if txRoot := types.DeriveSha(txs, stxs); txRoot != hdr.TxHash() {
			setOnce(&rootsErr, "tx root mismatch at %d: %s vs header %s", n, txRoot.Hex(), hdr.TxHash().Hex())
		}
		// Harmony's ReadReceipts ignores the config argument (no
		// DeriveFields pass); nil is the stock call shape.
		receipts := rawdb.ReadReceipts(r.db, ch, n, nil)
		if receipts == nil {
			setOnce(&contErr, "receipts missing at %d", n)
			break
		}
		if len(receipts) > 0 || hdr.ReceiptHash() != types.EmptyRootHash {
			if rroot := types.DeriveSha(receipts); rroot != hdr.ReceiptHash() {
				setOnce(&rootsErr, "receipt root mismatch at %d: %s vs header %s", n, rroot.Hex(), hdr.ReceiptHash().Hex())
			}
		}
		incoming := body.IncomingReceipts()
		inRoot := types.EmptyRootHash
		if len(incoming) > 0 {
			inRoot = types.DeriveSha(incoming)
		}
		if inRoot != hdr.IncomingReceiptHash() {
			setOnce(&rootsErr, "incoming receipt root mismatch at %d", n)
		}

		// Exact block-sig-N read (never the fallback accessor), parsed,
		// quorum-checked and BLS-verified (plan §11.1).
		sigVal, found, err := hasThenGet(r.db, keys.BlockSigKey(n))
		if err != nil {
			setOnce(&certErr, "read block-sig-%d: %v", n, err)
		} else if !found {
			setOnce(&certErr, "exact block-sig-%d missing (fallback accessor is forbidden)", n)
		} else if err := cv.VerifyCommitSigBytes(hdr, sigVal); err != nil {
			setOnce(&certErr, "%v", err)
		} else {
			verified++
		}

		// Per-tx lookups, deep direction (the raw scan checked the reverse).
		// Plain transactions carry TWO lookup entries — harmony hash and
		// eth-converted hash (rawdb.WriteBlockTxLookUpEntries) — deduped
		// when they coincide.
		for i, tx := range body.Transactions() {
			for _, h := range []common.Hash{tx.Hash(), tx.ConvertToEth().Hash()} {
				bh, bn, idx := rawdb.ReadTxLookupEntry(r.db, h)
				if bh != ch || bn != n || idx != uint64(i) {
					setOnce(&lookupErr, "tx %s lookup mismatch at block %d index %d", h.Hex(), n, i)
				}
				if !r.seenTxLookup[h] {
					r.seenTxLookup[h] = true
					r.windowTxs++
				}
			}
		}
		for i, stx := range body.StakingTransactions() {
			bh, bn, idx := rawdb.ReadTxLookupEntry(r.db, stx.Hash())
			if bh != ch || bn != n || idx != uint64(i) {
				setOnce(&lookupErr, "staking tx %s lookup mismatch at block %d", stx.Hash().Hex(), n)
			}
			if !r.seenTxLookup[stx.Hash()] {
				r.seenTxLookup[stx.Hash()] = true
				r.windowTxs++
			}
		}
		// Incoming cross-shard receipts each carry a cx lookup entry
		// (rawdb.WriteCxLookupEntries).
		for _, cxp := range incoming {
			for _, cx := range cxp.Receipts {
				bh, bn, _ := rawdb.ReadCxLookupEntry(r.db, cx.TxHash)
				if bh != ch || bn != n {
					setOnce(&lookupErr, "cx lookup for %s mismatch at block %d", cx.TxHash.Hex(), n)
				}
				if !r.seenCxLookup[cx.TxHash] {
					r.seenCxLookup[cx.TxHash] = true
					r.windowCxs++
				}
			}
		}
	}

	// No canonical mapping at target+1 (plan §11.1).
	if ch := rawdb.ReadCanonicalHash(r.db, r.p.Window.Target+1); ch != (common.Hash{}) {
		setOnce(&contErr, "canonical mapping present at target+1 (%s)", ch.Hex())
	}

	// Target certificate cross-checks against the anchor.
	if r.p.Anchor.TargetCertificateSHA256 != "" {
		sigVal, found, err := hasThenGet(r.db, keys.BlockSigKey(r.p.Window.Target))
		if err == nil && found {
			if sum := integrity.BytesSHA256(sigVal); sum != r.p.Anchor.TargetCertificateSHA256 {
				setOnce(&certErr, "target certificate sha256 %s != anchor %s", sum, r.p.Anchor.TargetCertificateSHA256)
			}
		}
	}

	// Lookup count equality closes the bijection (scan checked resolution).
	if lookupErr == nil && r.txLookups != r.windowTxs {
		setOnce(&lookupErr, "tx lookup keys %d != retained unique tx hashes %d", r.txLookups, r.windowTxs)
	}
	if lookupErr == nil && r.cxLookups != r.windowCxs {
		setOnce(&lookupErr, "cx lookup keys %d != retained unique incoming cx hashes %d", r.cxLookups, r.windowCxs)
	}

	r.check(CheckWindowContinuity, contErr)
	r.check(CheckWindowRoots, rootsErr)
	r.check(CheckWindowCerts, certErr)
	if lookupErr != nil {
		r.fail(CheckWindowLookups+".deep", "%v", lookupErr)
	} else {
		r.ok(CheckWindowLookups + ".deep")
	}
	return verified
}

func (r *runner) phaseStakingCrossChecks() {
	// Validator list == target-state validators, no duplicates; delegation
	// indexes mutually complete with state (plan §11.3).
	st, err := state.New(r.stateRoot, state.NewDatabase(r.db), nil)
	if err != nil {
		r.fail(CheckValidatorListState, "reopen state: %v", err)
		return
	}
	addrs, err := rawdb.ReadValidatorList(r.db)
	if err != nil {
		r.fail(CheckValidatorListState, "read validator list: %v", err)
		return
	}
	seen := map[common.Address]bool{}
	var vlErr error
	validatorSet := map[common.Address]bool{}
	for _, a := range addrs {
		if seen[a] {
			vlErr = fmt.Errorf("duplicate validator-list entry %s", a.Hex())
			break
		}
		seen[a] = true
		validatorSet[a] = true
		if _, err := st.ValidatorWrapper(a, true, false); err != nil {
			vlErr = fmt.Errorf("validator %s in list has no wrapper in target state: %v", a.Hex(), err)
			break
		}
	}
	r.check(CheckValidatorListState, vlErr)

	// Delegations: every dvl index entry references a listed validator, and
	// every delegation inside every wrapper has a dvl entry.
	var dvlErr error
	delegatorHasIndex := map[common.Address]map[common.Address]bool{}
	err = strictdb.ForEach(r.db, keys.DVLPrefix, func(key, value []byte) error {
		if keys.Classify(key) != keys.BucketDVL {
			return nil
		}
		delegator := common.BytesToAddress(key[len(keys.DVLPrefix):])
		indexes, err := rawdb.ReadDelegationsByDelegator(r.db, delegator)
		if err != nil {
			return fmt.Errorf("decode dvl for %s: %w", delegator.Hex(), err)
		}
		m := map[common.Address]bool{}
		for _, idx := range indexes {
			if !validatorSet[idx.ValidatorAddress] {
				return fmt.Errorf("dvl of %s references unlisted validator %s", delegator.Hex(), idx.ValidatorAddress.Hex())
			}
			m[idx.ValidatorAddress] = true
		}
		delegatorHasIndex[delegator] = m
		return nil
	})
	if err != nil {
		dvlErr = err
	} else {
		for _, a := range addrs {
			wrapper, err := st.ValidatorWrapper(a, true, false)
			if err != nil {
				dvlErr = fmt.Errorf("wrapper %s: %v", a.Hex(), err)
				break
			}
			for _, d := range wrapper.Delegations {
				if !delegatorHasIndex[d.DelegatorAddress][a] {
					dvlErr = fmt.Errorf("delegation of %s to %s has no dvl index entry", d.DelegatorAddress.Hex(), a.Hex())
					break
				}
			}
			if dvlErr != nil {
				break
			}
		}
	}
	r.check(CheckDelegationsState, dvlErr)

	// Current-epoch VRF references resolve to retained canonical blocks.
	// Read errors and undecodable records are FAILURES (round 13 finding 7).
	// ABSENCE is not: the production node's epoch-VRF write path is
	// commented out (core/offchain.go:70-96 — nothing in the stock tree
	// calls WriteEpochVrfBlockNums), so a replayed artifact legitimately
	// has no record for recent epochs; any record that IS present must be
	// decodable and fully resolvable.
	vrfErr := r.vrfScanErr
	epoch := new(big.Int).SetUint64(r.p.Window.Epoch)
	raw, found, err := hasThenGet(r.db, keys.EpochVrfKey(epoch))
	switch {
	case vrfErr != nil:
	case err != nil:
		vrfErr = fmt.Errorf("read epoch-%d VRF block numbers: %w", r.p.Window.Epoch, err)
	case found:
		nums := []uint64{}
		if err := rlp.DecodeBytes(raw, &nums); err != nil {
			vrfErr = fmt.Errorf("decode epoch-%d VRF block numbers: %w", r.p.Window.Epoch, err)
		} else {
			for _, n := range nums {
				if n > r.p.Window.Target || n < r.p.Window.RetainFrom {
					vrfErr = fmt.Errorf("epoch-%d VRF reference %d outside retained window", r.p.Window.Epoch, n)
					break
				}
				if ch := rawdb.ReadCanonicalHash(r.db, n); ch == (common.Hash{}) {
					vrfErr = fmt.Errorf("epoch-%d VRF reference %d unresolvable", r.p.Window.Epoch, n)
					break
				}
			}
		}
	}
	r.check(CheckVRFReferences, vrfErr)
}

func (r *runner) phaseMarkerDigests() string {
	marker, err := ReadMarker(r.db)
	if err != nil {
		return "" // already failed in phase A
	}
	if r.logical != nil {
		if marker.LogicalKVDigest != r.logical.Total.SHA256 {
			r.fail(CheckMarkerLogical, "marker logical digest %s != recomputed %s", marker.LogicalKVDigest, r.logical.Total.SHA256)
		} else if r.p.Compact.LogicalKVDigest != r.logical.Total.SHA256 {
			r.fail(CheckMarkerLogical, "compact.json logical digest %s != recomputed %s", r.p.Compact.LogicalKVDigest, r.logical.Total.SHA256)
		} else {
			r.ok(CheckMarkerLogical)
		}
	}
	sections, err := ComputeNormalizedSections(r.db)
	if err != nil {
		r.fail(CheckMarkerNormalized, "recompute normalized sections: %v", err)
		return ""
	}
	normalized := NormalizedOutputDigest(sections)
	if marker.NormalizedOutputDigest != normalized {
		r.fail(CheckMarkerNormalized, "marker normalized-output digest %s != recomputed %s", marker.NormalizedOutputDigest, normalized)
	} else if r.p.Compact.NormalizedOutputDigest != normalized {
		r.fail(CheckMarkerNormalized, "compact.json normalized-output digest %s != recomputed %s", r.p.Compact.NormalizedOutputDigest, normalized)
	} else {
		r.ok(CheckMarkerNormalized)
	}
	return normalized
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
