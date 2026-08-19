// Package compact implements compact-db: the strict target-state compactor
// that builds a fresh validator harmony_db_0 from the replayed working copy
// (plan WS5).
package compact

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
	staking "github.com/harmony-one/harmony/staking/types"
)

// DefaultSizeLimitBytes is the 200 GB size gate (operator answer 4).
const DefaultSizeLimitBytes = uint64(200) * 1024 * 1024 * 1024

// DefaultBatchBytes is the default write-batch flush threshold (128 MiB).
const DefaultBatchBytes = 128 * 1024 * 1024

// Config parameterizes compact-db.
type Config struct {
	Network     string
	ShardID     uint32
	ChainConfig *params.ChainConfig

	SourceDB      string
	DestinationDB string

	AnchorPath          string
	SourceReferencePath string // replay.json

	MetadataReferenceManifestPath string // optional (reference mode)

	TargetHeight       uint64
	RetainFromOverride uint64

	BatchBytes     int
	SizeLimitBytes uint64

	WithValidatorStats bool
	WithPreimages      string // optional consumer-list JSON (declared subset)

	ToolVersion string
	OutputPath  string // compact.json (written durably before the journal completes)
}

// Run builds the compact artifact. On any error the journal (if created)
// stays IN_PROGRESS and the destination must be discarded.
func Run(cfg Config, window anchor.Window) (*report.CompactReport, error) {
	start := time.Now()
	if cfg.BatchBytes <= 0 {
		cfg.BatchBytes = DefaultBatchBytes
	}
	if cfg.SizeLimitBytes == 0 {
		cfg.SizeLimitBytes = DefaultSizeLimitBytes
	}

	// ---- Checksum gates. ----
	inputs := []integrity.InputRef{}
	if _, err := integrity.VerifyChecksumFile(cfg.AnchorPath); err != nil {
		return nil, fmt.Errorf("compact: checksum gate: %w", err)
	}
	anchorRef, err := integrity.NewInputRef("anchor-manifest", cfg.AnchorPath)
	if err != nil {
		return nil, err
	}
	inputs = append(inputs, anchorRef)
	anc, err := anchor.Load(cfg.AnchorPath)
	if err != nil {
		return nil, err
	}
	if err := anc.RequireTargetHeight(cfg.TargetHeight); err != nil {
		return nil, err
	}
	if anc.Network != cfg.Network || anc.ShardID != cfg.ShardID {
		return nil, fmt.Errorf("compact: anchor is for %s shard %d, run is %s shard %d (round 13 finding 3)",
			anc.Network, anc.ShardID, cfg.Network, cfg.ShardID)
	}

	if _, err := integrity.VerifyChecksumFile(cfg.SourceReferencePath); err != nil {
		return nil, fmt.Errorf("compact: checksum gate: %w", err)
	}
	srcRef, err := integrity.NewInputRef("source-reference", cfg.SourceReferencePath)
	if err != nil {
		return nil, err
	}
	inputs = append(inputs, srcRef)
	var replayRep report.ReplayReport
	if err := report.ReadJSONStrict(cfg.SourceReferencePath, &replayRep); err != nil {
		return nil, err
	}
	if replayRep.DigestSet == nil {
		return nil, fmt.Errorf("compact: --source-reference carries no DigestSet")
	}
	if err := replayRep.DigestSet.Validate(); err != nil {
		return nil, err
	}
	// The replay report must chain to the same anchor (plan §4 integrity).
	chained := false
	for _, in := range replayRep.Inputs {
		if in.Name == "anchor-manifest" && in.SHA256 == anchorRef.SHA256 {
			chained = true
		}
	}
	if !chained {
		return nil, fmt.Errorf("compact: --source-reference does not chain to the supplied anchor manifest")
	}
	if !replayRep.Gate.Passed {
		return nil, fmt.Errorf("compact: --source-reference gate did not pass")
	}

	// Source journal must be COMPLETE_VERIFIED (plan §4).
	srcJournal := report.JournalPath(cfg.SourceDB)
	if st, _, err := report.JournalState(srcJournal); err != nil {
		return nil, fmt.Errorf("compact: source journal: %w", err)
	} else if st != report.StateCompleteVerified {
		return nil, fmt.Errorf("compact: source journal state %s; only COMPLETE_VERIFIED sources are compactable", st)
	}

	// Optional metadata-reference manifest (reference mode).
	mode := report.ModeInternal
	referenceDigest := verify.MetadataReferenceInternalNone
	var refManifest *verify.MetadataReferenceManifest
	if cfg.MetadataReferenceManifestPath != "" {
		m, sum, err := verify.LoadMetadataReferenceManifest(cfg.MetadataReferenceManifestPath)
		if err != nil {
			return nil, fmt.Errorf("compact: metadata-reference manifest: %w", err)
		}
		refManifest = m
		referenceDigest = sum
		mode = report.ModeReference
		ref, err := integrity.NewInputRef("metadata-reference-manifest", cfg.MetadataReferenceManifestPath)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, ref)
	}

	// ---- Open source strictly read-only; verify head tuple == target. ----
	src, srcRO, err := dbopen.OpenSourceDatabase(cfg.SourceDB)
	if err != nil {
		return nil, err
	}
	defer srcRO.Close()
	for _, hk := range [][]byte{keys.HeadBlockKey, keys.HeadHeaderKey} {
		val, err := src.Get(hk)
		if err != nil {
			return nil, fmt.Errorf("compact: read source head: %w", err)
		}
		if common.BytesToHash(val) != anc.TargetHash {
			return nil, fmt.Errorf("compact: source head %x != pinned target %s (source not at target)", val, anc.TargetHash.Hex())
		}
	}
	targetHdr := rawdb.ReadHeader(src, anc.TargetHash, cfg.TargetHeight)
	if targetHdr == nil {
		return nil, fmt.Errorf("compact: source target header missing")
	}
	targetRoot := targetHdr.Root()
	if replayRep.DigestSet.StateRoot != targetRoot.Hex() {
		return nil, fmt.Errorf("compact: source state root %s != source-reference %s", targetRoot.Hex(), replayRep.DigestSet.StateRoot)
	}

	// ---- Destination + journal. ----
	dst, err := dbopen.OpenDestination(cfg.DestinationDB, true /* --fail-if-destination-nonempty */)
	if err != nil {
		return nil, err
	}
	dstClosed := false
	defer func() {
		if !dstClosed {
			dst.Close()
		}
	}()
	journal, err := report.CreateJournal(report.JournalPath(cfg.DestinationDB))
	if err != nil {
		return nil, err
	}
	defer journal.Close()

	rep := &report.CompactReport{
		SourceDB:                cfg.SourceDB,
		DestinationDB:           cfg.DestinationDB,
		Window:                  report.DigestWindow{RetainFrom: window.RetainFrom, Target: window.Target},
		TargetHash:              anc.TargetHash.Hex(),
		StateRoot:               targetRoot.Hex(),
		Counts:                  map[string]uint64{},
		Mode:                    mode,
		MetadataReferenceDigest: referenceDigest,
		ValidatorStatsIncluded:  cfg.WithValidatorStats,
	}
	meta, err := report.NewMeta(report.CompactSchemaV1, "compact-db", cfg.Network, cfg.ShardID, cfg.ToolVersion, inputs)
	if err != nil {
		return nil, err
	}
	rep.Meta = meta

	c := &compactor{cfg: cfg, anc: anc, window: window, src: src, dst: dst, rep: rep, targetRoot: targetRoot}

	// ---- Content phases (no head key until all succeed). ----
	if err := c.copyState(); err != nil {
		return nil, err
	}
	report.CrashPoint("compact.after-state-copy")
	if err := c.copyCanonicalWindow(); err != nil {
		return nil, err
	}
	report.CrashPoint("compact.after-window-copy")
	if err := c.copyOffchain(); err != nil {
		return nil, err
	}
	report.CrashPoint("compact.after-offchain-copy")
	if err := c.bloomCheckpoint(); err != nil {
		return nil, err
	}
	if err := c.flush(); err != nil {
		return nil, err
	}
	if err := c.validatorCodePass(); err != nil {
		return nil, err
	}

	// ---- Destination DigestSet must equal the source-reference baseline
	// (fail before any head is written). ----
	walk, err := verify.WalkState(dst, targetRoot, verify.StateWalkOptions{})
	if err != nil {
		return nil, fmt.Errorf("compact: destination state walk: %w", err)
	}
	off, err := verify.ComputeOffchainDigests(dst, rep.Window)
	if err != nil {
		return nil, fmt.Errorf("compact: destination off-chain digests: %w", err)
	}
	rep.DigestSet = verify.BuildDigestSet(cfg.Network, cfg.ShardID, cfg.TargetHeight, anc.TargetHash, targetRoot, rep.Window, walk, off)
	baseline := replayRep.DigestSet
	if rep.Window != baseline.Window {
		// --retain-from-height extended the window beyond the one replay
		// recorded (round 13 finding 5). The window-scoped domains (outgoing
		// CX, reward accumulators) are recomputed from the read-only SOURCE
		// over the extended window and rebased onto the replay baseline; the
		// non-window domains must still match replay's record byte for byte,
		// which proves the source is unchanged since the replay gate.
		if rep.Window.Target != baseline.Window.Target || rep.Window.RetainFrom >= baseline.Window.RetainFrom {
			return nil, fmt.Errorf("compact: window %+v is not an extension of the source-reference window %+v",
				rep.Window, baseline.Window)
		}
		srcOff, err := verify.ComputeOffchainDigests(src, rep.Window)
		if err != nil {
			return nil, fmt.Errorf("compact: source off-chain digests over extended window: %w", err)
		}
		rebased := *baseline
		rebased.Window = rep.Window
		rebased.CXOutgoingWindow = srcOff.CXOutgoingWindow
		rebased.RewardAccumulators = srcOff.RewardAccumulators
		for name, d := range map[string][2]report.Digest{
			"cx_spent":             {srcOff.CXSpent, baseline.CXSpent},
			"crosslink_index":      {srcOff.CrosslinkIndex, baseline.CrosslinkIndex},
			"crosslink_shard_last": {srcOff.CrosslinkShardLast, baseline.CrosslinkShardLast},
			"validator_list":       {srcOff.ValidatorList, baseline.ValidatorList},
			"delegations":          {srcOff.Delegations, baseline.Delegations},
			"validator_snapshots":  {srcOff.ValidatorSnapshots, baseline.ValidatorSnapshots},
			"shard_states":         {srcOff.ShardStates, baseline.ShardStates},
			"epoch_block_numbers":  {srcOff.EpochBlockNumbers, baseline.EpochBlockNumbers},
			"epoch_vrf":            {srcOff.EpochVrf, baseline.EpochVrf},
			"epoch_vdf":            {srcOff.EpochVdf, baseline.EpochVdf},
		} {
			if d[0] != d[1] {
				return nil, fmt.Errorf("compact: source domain %s changed since replay (window rebase refused): %+v vs %+v", name, d[0], d[1])
			}
		}
		baseline = &rebased
	}
	if diffs := rep.DigestSet.Diff(baseline); len(diffs) > 0 {
		return nil, fmt.Errorf("compact: destination DigestSet differs from --source-reference baseline: %v", diffs)
	}

	// ---- (1) Metadata-reference convergence proof (reference mode), before
	// any head key (plan WS5 step 1, round 8 finding 1). ----
	sections, err := verify.ComputeNormalizedSections(dst)
	if err != nil {
		return nil, fmt.Errorf("compact: normalized sections: %w", err)
	}
	rep.NormalizedSections = sections
	rep.NormalizedOutputDigest = verify.NormalizedOutputDigest(sections)
	if refManifest != nil {
		if diffs := verify.CompareNormalizedSections(refManifest, sections); len(diffs) > 0 {
			return nil, fmt.Errorf("compact: metadata-reference convergence failed (refusing before writing any head): %v", diffs)
		}
	}

	// ---- (2) All four heads to the pinned target hash. ----
	headBatch := strictdb.NewLatchingBatch(dst, 0)
	for _, hk := range [][]byte{keys.HeadBlockKey, keys.HeadHeaderKey, keys.HeadFastBlockKey, keys.HeadFinalizedKey} {
		if err := headBatch.Put(hk, anc.TargetHash.Bytes()); err != nil {
			return nil, fmt.Errorf("compact: write head: %w", err)
		}
	}
	if err := headBatch.Flush(); err != nil {
		return nil, fmt.Errorf("compact: flush heads: %w", err)
	}

	// ---- (3) Logical KV digest over the now-final keyspace (marker key is
	// the digest's single defined exclusion and does not exist yet). ----
	logical, err := verify.ComputeLogicalDigest(dst)
	if err != nil {
		return nil, fmt.Errorf("compact: logical digest: %w", err)
	}
	rep.LogicalKVDigest = logical.Total.SHA256
	rep.LogicalBuckets = logical.Buckets

	// ---- (4) Recovery-completion marker. ----
	marker := &verify.Marker{
		SchemaVersion:           verify.MarkerSchemaV1,
		Network:                 cfg.Network,
		ShardID:                 cfg.ShardID,
		TargetHeight:            cfg.TargetHeight,
		TargetHash:              anc.TargetHash.Hex(),
		AnchorManifestSHA256:    anchorRef.SHA256,
		MetadataReferenceDigest: referenceDigest,
		ToolVersion:             cfg.ToolVersion,
		ToolBinarySHA256:        meta.ToolBinary,
		NormalizedOutputDigest:  rep.NormalizedOutputDigest,
		LogicalKVDigest:         logical.Total.SHA256,
	}
	markerRaw, err := marker.Encode()
	if err != nil {
		return nil, err
	}
	report.CrashPoint("compact.after-heads-before-marker")
	if err := dst.Put(keys.RecoveryMarkerKey, markerRaw); err != nil {
		return nil, fmt.Errorf("compact: write recovery marker: %w", err)
	}
	var markerMap map[string]interface{}
	if err := jsonRoundTrip(markerRaw, &markerMap); err != nil {
		return nil, err
	}
	rep.Marker = markerMap

	// ---- Durability, size gate, journal. ----
	if err := dst.Close(); err != nil {
		return nil, fmt.Errorf("compact: close destination: %w", err)
	}
	dstClosed = true
	if err := report.FsyncWalk(report.OSFS, cfg.DestinationDB); err != nil {
		return nil, err
	}
	bytesUsed, files, err := dirSize(cfg.DestinationDB)
	if err != nil {
		return nil, err
	}
	rep.DestinationBytes = bytesUsed
	rep.DestinationFiles = files
	rep.SizeGate.LimitBytes = cfg.SizeLimitBytes
	rep.SizeGate.ActualBytes = bytesUsed
	rep.SizeGate.Passed = bytesUsed <= cfg.SizeLimitBytes
	rep.WallSeconds = time.Since(start).Seconds()

	// compact.json is written durably BEFORE the journal completes, so a
	// COMPLETE_* journal always has its report.
	state := report.StateCompleteVerified
	note := "compact build complete"
	if !rep.SizeGate.Passed {
		state = report.StateCompleteUnreleasable
		note = fmt.Sprintf("size gate: %d bytes > limit %d", bytesUsed, cfg.SizeLimitBytes)
	}
	rep.JournalState = state
	if cfg.OutputPath != "" {
		if _, err := report.WriteJSON(cfg.OutputPath, rep); err != nil {
			return nil, err
		}
	}
	report.CrashPoint("compact.after-report-before-journal")
	if err := journal.Complete(state, note); err != nil {
		return nil, err
	}
	return rep, nil
}

type compactor struct {
	cfg        Config
	anc        *anchor.Manifest
	window     anchor.Window
	src        ethdb.Database
	dst        ethdb.Database
	rep        *report.CompactReport
	targetRoot common.Hash

	batch *strictdb.LatchingBatch
}

func (c *compactor) put(counter string, key, value []byte) error {
	if c.batch == nil {
		c.batch = strictdb.NewLatchingBatch(c.dst, c.cfg.BatchBytes)
	}
	if err := c.batch.Put(key, value); err != nil {
		return fmt.Errorf("compact: batch put %s: %w", counter, err)
	}
	c.rep.Counts[counter]++
	return nil
}

func (c *compactor) flush() error {
	if c.batch == nil {
		return nil
	}
	if err := c.batch.Flush(); err != nil {
		return fmt.Errorf("compact: batch flush: %w", err)
	}
	return nil
}

// mustGet is a strict source read.
func (c *compactor) mustGet(key []byte, what string) ([]byte, error) {
	val, err := c.src.Get(key)
	if err != nil {
		return nil, fmt.Errorf("compact: source read %s (%x): %w", what, key, err)
	}
	return val, nil
}

func (c *compactor) maybeGet(key []byte) ([]byte, bool, error) {
	ok, err := c.src.Has(key)
	if err != nil {
		return nil, false, fmt.Errorf("compact: source has %x: %w", key, err)
	}
	if !ok {
		return nil, false, nil
	}
	val, err := c.src.Get(key)
	if err != nil {
		return nil, false, fmt.Errorf("compact: source get %x: %w", key, err)
	}
	return val, true, nil
}

// copyState drives a trie.Sync scheduler over the target root, reading
// nodes from the strictly read-only source by hash, verifying
// keccak256(payload) == hash before Process (plan §10.3). Code is copied
// separately, location-preserving; the scheduler is built with a custom
// account callback that schedules storage subtries but does NOT register
// code entries (trie.Sync.Commit would otherwise write all code under the
// 'c' prefix, clobbering vc/legacy locations).
func (c *compactor) copyState() error {
	var codeHashes []common.Hash
	seenCode := map[common.Hash]bool{}

	var sched *trie.Sync
	onAccount := func(paths [][]byte, path []byte, leaf []byte, parent common.Hash, parentPath []byte) error {
		var acc state.Account
		if err := rlp.DecodeBytes(leaf, &acc); err != nil {
			return fmt.Errorf("compact: decode account leaf: %w", err)
		}
		if acc.Root != state.EmptyRootHash && acc.Root != (common.Hash{}) {
			sched.AddSubTrie(acc.Root, path, parent, parentPath, nil)
		}
		if len(acc.CodeHash) == 32 && !bytes.Equal(acc.CodeHash, state.EmptyCodeHash.Bytes()) {
			ch := common.BytesToHash(acc.CodeHash)
			if !seenCode[ch] {
				seenCode[ch] = true
				codeHashes = append(codeHashes, ch)
			}
		}
		return nil
	}
	sched = trie.NewSync(c.targetRoot, c.dst, onAccount, rawdb.HashScheme)

	batch := strictdb.NewLatchingBatch(c.dst, c.cfg.BatchBytes)
	for {
		paths, nodes, codes := sched.Missing(2048)
		if len(codes) > 0 {
			return fmt.Errorf("compact: scheduler requested code entries unexpectedly")
		}
		if len(nodes) == 0 {
			break
		}
		for i, hash := range nodes {
			data, err := c.mustGet(hash.Bytes(), "trie node")
			if err != nil {
				return err
			}
			if crypto.Keccak256Hash(data) != hash {
				return fmt.Errorf("compact: trie node %s fails content verification", hash.Hex())
			}
			if err := sched.ProcessNode(trie.NodeSyncResult{Path: paths[i], Data: data}); err != nil {
				return fmt.Errorf("compact: ProcessNode %s: %w", hash.Hex(), err)
			}
			c.rep.Counts["state.trieNodes"]++
		}
		if err := sched.Commit(batch); err != nil {
			return fmt.Errorf("compact: scheduler commit: %w", err)
		}
	}
	if sched.Pending() != 0 {
		return fmt.Errorf("compact: scheduler finished with %d pending requests", sched.Pending())
	}
	if err := sched.Commit(batch); err != nil {
		return fmt.Errorf("compact: final scheduler commit: %w", err)
	}
	if err := batch.Flush(); err != nil {
		return err
	}

	// Code copy: same location as found in the source, content-verified.
	for _, ch := range codeHashes {
		code, loc, err := verify.ResolveCode(c.src, ch)
		if err != nil {
			return fmt.Errorf("compact: %w", err)
		}
		var key []byte
		switch loc {
		case verify.CodeLocPrefixed:
			key = keys.CodeKey(ch)
		case verify.CodeLocValidator:
			key = keys.ValidatorCodeKey(ch)
		case verify.CodeLocLegacy:
			key = ch.Bytes()
		}
		if err := c.put("state.codes."+loc, key, code); err != nil {
			return err
		}
	}
	return c.flush()
}

// copyCanonicalWindow copies genesis records, chain config, database
// version, and every canonical block in [retainFrom, target] with exact
// certificates and regenerated lookups (plan §10.4).
func (c *compactor) copyCanonicalWindow() error {
	// Genesis records.
	genHash := rawdb.ReadCanonicalHash(c.src, 0)
	if genHash == (common.Hash{}) {
		return fmt.Errorf("compact: source has no canonical genesis")
	}
	if err := c.copyBlockMaterial(0, genHash, false); err != nil {
		return err
	}
	// Chain config: copy if present, else write fresh from the built-in
	// config (plan §2.1 — stock runs off the built-in config; normalized on
	// output).
	cfgKey := keys.ConfigKey(genHash)
	if val, found, err := c.maybeGet(cfgKey); err != nil {
		return err
	} else if found {
		if err := c.put("chainConfig", cfgKey, val); err != nil {
			return err
		}
	} else {
		raw, err := jsonMarshal(c.cfg.ChainConfig)
		if err != nil {
			return fmt.Errorf("compact: marshal built-in chain config: %w", err)
		}
		if err := c.put("chainConfig", cfgKey, raw); err != nil {
			return err
		}
	}
	// Genesis state spec + database version: copy if present.
	if val, found, err := c.maybeGet(keys.GenesisSpecKey(genHash)); err != nil {
		return err
	} else if found {
		if err := c.put("genesisSpec", keys.GenesisSpecKey(genHash), val); err != nil {
			return err
		}
	}
	if val, found, err := c.maybeGet(keys.DatabaseVersionKey); err != nil {
		return err
	} else if found {
		if err := c.put("databaseVersion", keys.DatabaseVersionKey, val); err != nil {
			return err
		}
	}

	// Window blocks.
	for n := c.window.RetainFrom; n <= c.window.Target; n++ {
		ch := rawdb.ReadCanonicalHash(c.src, n)
		if ch == (common.Hash{}) {
			return fmt.Errorf("compact: source canonical mapping missing at %d", n)
		}
		if err := c.copyBlockMaterial(n, ch, true); err != nil {
			return err
		}
	}
	// The target's exact certificate must match the anchor when pinned.
	if c.anc.TargetCertificateSHA256 != "" {
		val, err := c.mustGet(keys.BlockSigKey(c.window.Target), "target block-sig")
		if err != nil {
			return err
		}
		if got := integrity.BytesSHA256(val); got != c.anc.TargetCertificateSHA256 {
			return fmt.Errorf("compact: target certificate sha256 %s != anchor %s", got, c.anc.TargetCertificateSHA256)
		}
	}
	return c.flush()
}

func (c *compactor) copyBlockMaterial(n uint64, ch common.Hash, window bool) error {
	// Canonical + inverse mappings.
	if err := c.put("canonical", keys.CanonicalHashKey(n), ch.Bytes()); err != nil {
		return err
	}
	numVal, err := c.mustGet(keys.HeaderNumberKey(ch), "header number")
	if err != nil {
		return err
	}
	if err := c.put("headerNumber", keys.HeaderNumberKey(ch), numVal); err != nil {
		return err
	}
	// Header + body + receipts.
	hdrVal, err := c.mustGet(keys.HeaderKey(n, ch), "header")
	if err != nil {
		return err
	}
	if err := c.put("header", keys.HeaderKey(n, ch), hdrVal); err != nil {
		return err
	}
	bodyVal, err := c.mustGet(keys.BodyKey(n, ch), "body")
	if err != nil {
		return err
	}
	if err := c.put("body", keys.BodyKey(n, ch), bodyVal); err != nil {
		return err
	}
	if rcVal, found, err := c.maybeGet(keys.ReceiptsKey(n, ch)); err != nil {
		return err
	} else if found {
		if err := c.put("receipts", keys.ReceiptsKey(n, ch), rcVal); err != nil {
			return err
		}
	} else if window {
		return fmt.Errorf("compact: receipts missing for window block %d", n)
	}
	// TD copy-if-present (plan §2.2.2: never written in production).
	if tdVal, found, err := c.maybeGet(keys.HeaderTDKey(n, ch)); err != nil {
		return err
	} else if found {
		if err := c.put("td", keys.HeaderTDKey(n, ch), tdVal); err != nil {
			return err
		}
	}

	if !window {
		return nil
	}

	// Exact block-sig-N for EVERY retained block: copied from the source's
	// exact key, or synthesized from child header N+1 after verification
	// (plan §10.4 — blocks below the replayed range on some sources).
	sigVal, found, err := c.maybeGet(keys.BlockSigKey(n))
	if err != nil {
		return err
	}
	if !found {
		childHash := rawdb.ReadCanonicalHash(c.src, n+1)
		childHdr := rawdb.ReadHeader(c.src, childHash, n+1)
		if childHdr == nil {
			return fmt.Errorf("compact: block-sig-%d missing and child header %d unavailable for synthesis", n, n+1)
		}
		sig := childHdr.LastCommitSignature()
		sigVal = append(sig[:], childHdr.LastCommitBitmap()...)
	}
	hdr := rawdb.ReadHeader(c.src, ch, n)
	if hdr == nil {
		return fmt.Errorf("compact: header %d undecodable", n)
	}
	cv := verify.NewCertVerifier(c.src, c.cfg.ChainConfig, c.cfg.ShardID)
	if err := cv.VerifyCommitSigBytes(hdr, sigVal); err != nil {
		return fmt.Errorf("compact: certificate for %d: %w", n, err)
	}
	if err := c.put("blockSig", keys.BlockSigKey(n), sigVal); err != nil {
		return err
	}

	// Reward accumulator copy-if-present (staking era).
	if rwVal, found, err := c.maybeGet(keys.RewardAccumKey(n)); err != nil {
		return err
	} else if found {
		if err := c.put("rewardAccum", keys.RewardAccumKey(n), rwVal); err != nil {
			return err
		}
	}

	// Lookups regenerated from the retained canonical block (strict batch
	// variants of core/blockchain_impl.go:1643-1651).
	body := rawdb.ReadBody(c.src, ch, n)
	if body == nil {
		return fmt.Errorf("compact: body %d undecodable", n)
	}
	for i, tx := range body.Transactions() {
		entry := rawdb.LegacyTxLookupEntry{BlockHash: ch, BlockIndex: n, Index: uint64(i)}
		val, err := rlp.EncodeToBytes(entry)
		if err != nil {
			return fmt.Errorf("compact: encode lookup: %w", err)
		}
		if err := c.put("txLookup", keys.TxLookupKey(tx.Hash()), val); err != nil {
			return err
		}
		if err := c.put("txLookup", keys.TxLookupKey(tx.ConvertToEth().Hash()), val); err != nil {
			return err
		}
	}
	for i, stx := range body.StakingTransactions() {
		entry := rawdb.LegacyTxLookupEntry{BlockHash: ch, BlockIndex: n, Index: uint64(i)}
		val, err := rlp.EncodeToBytes(entry)
		if err != nil {
			return fmt.Errorf("compact: encode staking lookup: %w", err)
		}
		if err := c.put("txLookup", keys.TxLookupKey(stx.Hash()), val); err != nil {
			return err
		}
	}
	prev := 0
	for _, cxp := range body.IncomingReceipts() {
		for j, cx := range cxp.Receipts {
			entry := rawdb.LegacyTxLookupEntry{BlockHash: ch, BlockIndex: n, Index: uint64(prev + j)}
			val, err := rlp.EncodeToBytes(entry)
			if err != nil {
				return fmt.Errorf("compact: encode cx lookup: %w", err)
			}
			if err := c.put("cxLookup", keys.CxLookupKey(cx.TxHash), val); err != nil {
				return err
			}
		}
		prev += len(cxp.Receipts)
	}
	return nil
}

// copyOffchain copies the consensus/off-chain metadata per plan §10.5, with
// semantics aligned with in-place §2.2 (pending queues omitted entirely,
// validator stats omitted unless opted in).
func (c *compactor) copyOffchain() error {
	copyPrefix := func(counter string, prefix []byte, wantBucket string, filter func(key []byte) (bool, error)) error {
		return strictdb.ForEach(c.src, prefix, func(key, value []byte) error {
			bucket := keys.Classify(key)
			if bucket == keys.BucketBareHash32 {
				return nil // trie node that happens to share the prefix
			}
			if bucket != wantBucket {
				return fmt.Errorf("compact: unexpected key %x (bucket %s) under prefix %q", key, bucket, prefix)
			}
			if filter != nil {
				keep, err := filter(key)
				if err != nil {
					return err
				}
				if !keep {
					return nil
				}
			}
			return c.put(counter, key, value)
		})
	}

	// Full cxReceiptSpent set (never windowed).
	if err := copyPrefix("cxSpent", keys.CxSpentPrefix, keys.BucketCxSpent, nil); err != nil {
		return err
	}
	// Outgoing cxReceipt records for the window.
	if err := copyPrefix("cxReceipt", keys.CxReceiptPrefix, keys.BucketCxReceipt, func(key []byte) (bool, error) {
		num := binary.BigEndian.Uint64(key[len(keys.CxReceiptPrefix)+4 : len(keys.CxReceiptPrefix)+12])
		return num >= c.window.RetainFrom && num <= c.window.Target, nil
	}); err != nil {
		return err
	}
	// Full crosslink index + per-shard last values ("cl" covers both).
	if err := strictdb.ForEach(c.src, keys.CrosslinkPrefix, func(key, value []byte) error {
		switch keys.Classify(key) {
		case keys.BucketCrosslinkIndex:
			return c.put("crosslinkIndex", key, value)
		case keys.BucketCrosslinkShardLast:
			return c.put("crosslinkShardLast", key, value)
		case keys.BucketBareHash32:
			return nil
		default:
			return fmt.Errorf("compact: unexpected key %x under cl prefix", key)
		}
	}); err != nil {
		return err
	}
	// Validator list (target-state-validated in validatorCodePass).
	if val, found, err := c.maybeGet(keys.ValidatorListKey); err != nil {
		return err
	} else if found {
		if err := c.put("validatorList", keys.ValidatorListKey, val); err != nil {
			return err
		}
	}
	// All validator snapshots and all shard states (operator-approved
	// retention default, answer 4).
	if err := copyPrefix("validatorSnapshot", keys.ValidatorSnapshotPrefix, keys.BucketValidatorSnapshot, nil); err != nil {
		return err
	}
	if err := copyPrefix("shardState", keys.ShardStatePrefix, keys.BucketShardState, nil); err != nil {
		return err
	}
	// Delegator indexes (cross-validated in validatorCodePass).
	if err := copyPrefix("dvl", keys.DVLPrefix, keys.BucketDVL, nil); err != nil {
		return err
	}
	// Epoch block-number / VRF / VDF records.
	if err := copyPrefix("epochBlockNumber", keys.EpochBlockNumberPrefix, keys.BucketEpochBlockNumber, nil); err != nil {
		return err
	}
	if err := copyPrefix("epochVrf", keys.EpochVrfPrefix, keys.BucketEpochVrf, nil); err != nil {
		return err
	}
	if err := copyPrefix("epochVdf", keys.EpochVdfPrefix, keys.BucketEpochVdf, nil); err != nil {
		return err
	}
	// Leader-rotation meta copy-if-present (boot also rebuilds it).
	if val, found, err := c.maybeGet(keys.ContinuousKey); err != nil {
		return err
	} else if found {
		if err := c.put("leaderRotationMeta", keys.ContinuousKey, val); err != nil {
			return err
		}
	}
	// Validator stats: omitted by default (in-place §2.2); --with-validator-stats opts in.
	if c.cfg.WithValidatorStats {
		if err := copyPrefix("validatorStats", keys.ValidatorStatsPrefix, keys.BucketValidatorStats, nil); err != nil {
			return err
		}
	}
	// Optional preimage subset for a declared consumer list.
	if c.cfg.WithPreimages != "" {
		var list struct {
			Preimages []string `json:"preimages"` // hex hashed keys
		}
		if err := report.ReadJSONStrict(c.cfg.WithPreimages, &list); err != nil {
			return err
		}
		for _, hx := range list.Preimages {
			h := common.HexToHash(hx)
			val, err := c.mustGet(keys.PreimageKey(h), "preimage")
			if err != nil {
				return err
			}
			if err := c.put("preimages", keys.PreimageKey(h), val); err != nil {
				return err
			}
		}
	}
	// Pending queues: omitted entirely (absent = cleared — one semantic
	// with in-place §2.2 and WS4 step 8a).
	return c.flush()
}

// bloomCheckpoint writes the chain-indexer checkpoint so the RUNNING node's
// bloom indexer can advance on the artifact (round 13 finding 4): the next
// section to process (== the stored count) must need no headers below
// retainFrom, and the recorded section head must be a retained block. The
// plan's literal "last completed section below retainFrom" would leave the
// indexer permanently stuck on a section whose first 4,095 headers were
// pruned — deviation flagged. Sections below the checkpoint are marked done
// without bloom-bits data, exactly like stock dumpdb snapshots.
func (c *compactor) bloomCheckpoint() error {
	k, headBlock, ok := anchor.BloomCheckpoint(c.window)
	if !ok {
		return nil
	}
	ch := rawdb.ReadCanonicalHash(c.src, headBlock)
	if ch == (common.Hash{}) {
		return fmt.Errorf("compact: source canonical hash missing at %d for bloom checkpoint", headBlock)
	}
	count := make([]byte, 8)
	binary.BigEndian.PutUint64(count, k)
	if err := c.put("bloomIndex", keys.BloomIndexCountKey(), count); err != nil {
		return err
	}
	if err := c.put("bloomIndex", keys.BloomIndexSectionHeadKey(k-1), ch.Bytes()); err != nil {
		return err
	}
	return nil
}

// validatorCodePass asserts every target validator wrapper's code hash
// resolves through the stock state.Database on the DESTINATION (plan §10.3),
// and cross-validates delegator indexes against target wrappers (plan §10.5).
func (c *compactor) validatorCodePass() error {
	addrs, err := rawdb.ReadValidatorList(c.dst)
	if err != nil {
		return fmt.Errorf("compact: read destination validator list: %w", err)
	}
	st, err := state.New(c.targetRoot, state.NewDatabase(c.dst), nil)
	if err != nil {
		return fmt.Errorf("compact: open destination state: %w", err)
	}
	validators := map[common.Address]bool{}
	for _, a := range addrs {
		if _, err := st.ValidatorWrapper(a, true, false); err != nil {
			return fmt.Errorf("compact: validator %s wrapper does not resolve on the destination: %w", a.Hex(), err)
		}
		validators[a] = true
	}
	// Delegator indexes reference only known validators.
	if err := strictdb.ForEach(c.dst, keys.DVLPrefix, func(key, value []byte) error {
		if keys.Classify(key) != keys.BucketDVL {
			return nil
		}
		var indexes staking.DelegationIndexes
		if err := rlp.DecodeBytes(value, &indexes); err != nil {
			return fmt.Errorf("compact: decode dvl %x: %w", key, err)
		}
		for _, idx := range indexes {
			if !validators[idx.ValidatorAddress] {
				return fmt.Errorf("compact: dvl entry references unknown validator %s", idx.ValidatorAddress.Hex())
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func dirSize(root string) (uint64, uint64, error) {
	var bytesUsed, files uint64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			bytesUsed += uint64(info.Size())
			files++
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("compact: size %s: %w", root, err)
	}
	return bytesUsed, files, nil
}

// jsonMarshal mirrors rawdb.WriteChainConfig's encoding (plain json.Marshal).
func jsonMarshal(v interface{}) ([]byte, error) { return json.Marshal(v) }

func jsonRoundTrip(raw []byte, into *map[string]interface{}) error {
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("compact: marker round-trip: %w", err)
	}
	return nil
}
