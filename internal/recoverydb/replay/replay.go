// Package replay implements replay-bundle: strict offline replay of a
// verified bundle into the working copy up to the pinned target (plan WS4).
package replay

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/bundle"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

// Config parameterizes a replay run (CLI contract, plan §4).
type Config struct {
	Network string
	ShardID uint32

	DestinationDB string

	AnchorPath            string
	InspectReportPath     string
	BaselineAgreementPath string
	BundleDir             string
	BundleComparisonPath  string // optional (single-donor mode)

	TargetHeight uint64
	MinFreeBytes uint64

	ToolVersion string
	OutputPath  string
}

// Run executes the full replay per plan WS4 steps 1-9. Any error leaves the
// journal IN_PROGRESS (or not yet created if the failure precedes mutation).
func Run(cfg Config) (*report.ReplayReport, error) {
	start := time.Now()

	// ---- Step 1: checksum gates (before touching the destination). ----
	inputs := []integrity.InputRef{}
	gate := func(name, path string) (integrity.InputRef, error) {
		if _, err := integrity.VerifyChecksumFile(path); err != nil {
			return integrity.InputRef{}, fmt.Errorf("replay: checksum gate: %w", err)
		}
		ref, err := integrity.NewInputRef(name, path)
		if err != nil {
			return integrity.InputRef{}, err
		}
		inputs = append(inputs, ref)
		return ref, nil
	}

	anchorRef, err := gate("anchor-manifest", cfg.AnchorPath)
	if err != nil {
		return nil, err
	}
	anc, err := anchor.Load(cfg.AnchorPath)
	if err != nil {
		return nil, err
	}
	if err := anc.RequireTargetHeight(cfg.TargetHeight); err != nil {
		return nil, err
	}
	if anc.Network != cfg.Network || anc.ShardID != cfg.ShardID {
		return nil, fmt.Errorf("replay: anchor is for %s shard %d, run is %s shard %d", anc.Network, anc.ShardID, cfg.Network, cfg.ShardID)
	}

	inspectRef, err := gate("inspect-report", cfg.InspectReportPath)
	if err != nil {
		return nil, err
	}
	var inspect report.InspectReport
	if err := report.ReadJSONStrict(cfg.InspectReportPath, &inspect); err != nil {
		return nil, err
	}
	if _, err := gate("baseline-agreement", cfg.BaselineAgreementPath); err != nil {
		return nil, err
	}
	var verdict report.AgreementVerdict
	if err := report.ReadJSONStrict(cfg.BaselineAgreementPath, &verdict); err != nil {
		return nil, err
	}
	if !verdict.Agreed {
		return nil, fmt.Errorf("replay: baseline agreement verdict is negative")
	}
	if verdict.LeftReport != inspectRef.SHA256 && verdict.RightReport != inspectRef.SHA256 {
		return nil, fmt.Errorf("replay: agreement verdict does not name the supplied inspect report (%s)", inspectRef.SHA256)
	}

	manifest, manifestSHA, err := bundle.LoadManifest(cfg.BundleDir)
	if err != nil {
		return nil, err
	}
	inputs = append(inputs, integrity.InputRef{Name: "bundle-manifest", Path: bundle.ManifestPath(cfg.BundleDir), SHA256: manifestSHA})
	if err := manifest.VerifyChunks(cfg.BundleDir); err != nil {
		return nil, err
	}
	if cfg.BundleComparisonPath != "" {
		if _, err := gate("bundle-comparison", cfg.BundleComparisonPath); err != nil {
			return nil, err
		}
		var cmp bundle.CompareResult
		if err := report.ReadJSONStrict(cfg.BundleComparisonPath, &cmp); err != nil {
			return nil, err
		}
		if !cmp.Identical {
			return nil, fmt.Errorf("replay: bundle comparison reports a chain difference: %s", cmp.FirstDifference)
		}
	}

	// ---- Step 2 + 3: bundle-range gate and destination preconditions. ----
	if !inspect.ReplayPreflight.Ran || !inspect.ReplayPreflight.FullArchival {
		return nil, fmt.Errorf("replay: inspect report does not certify a full-archival source (plan §4: the only input class)")
	}
	if !inspect.BaselineGate.Ran || !inspect.BaselineGate.Passed {
		return nil, fmt.Errorf("replay: inspect report's baseline gate did not pass")
	}
	baselineHeight, baselineHash, err := headTupleOf(&inspect)
	if err != nil {
		return nil, err
	}
	if manifest.FromHeight != baselineHeight+1 || manifest.ToHeight != cfg.TargetHeight {
		return nil, fmt.Errorf("replay: bundle range [%d,%d] must equal exactly [baseline+1, target] = [%d,%d] (a bundle past the target is rejected outright)",
			manifest.FromHeight, manifest.ToHeight, baselineHeight+1, cfg.TargetHeight)
	}
	if manifest.BaselineHash != baselineHash.Hex() {
		return nil, fmt.Errorf("replay: bundle baseline hash %s != inspected baseline %s", manifest.BaselineHash, baselineHash.Hex())
	}
	if manifest.Sidecar.ChildHash != anc.AbandonedChildHash.Hex() {
		return nil, fmt.Errorf("replay: sidecar child hash %s != ABANDONED_CHILD_HASH %s", manifest.Sidecar.ChildHash, anc.AbandonedChildHash.Hex())
	}
	if manifest.TargetHash != anc.TargetHash.Hex() {
		return nil, fmt.Errorf("replay: bundle target hash %s != pinned %s", manifest.TargetHash, anc.TargetHash.Hex())
	}

	if err := dbopen.RequireAbsolute(cfg.DestinationDB); err != nil {
		return nil, err
	}
	free, err := dbopen.FreeSpace(filepath.Dir(cfg.DestinationDB))
	if err != nil {
		return nil, fmt.Errorf("replay: free-space check: %w", err)
	}
	if free < cfg.MinFreeBytes {
		return nil, fmt.Errorf("replay: %d bytes free below --min-free-bytes %d", free, cfg.MinFreeBytes)
	}
	journalPath := report.JournalPath(cfg.DestinationDB)
	if _, err := os.Stat(journalPath); err == nil {
		return nil, fmt.Errorf("replay: journal %s exists; v1 never resumes an unclean destination (discard and rebuild)", journalPath)
	}

	// Open the destination writable (the designated working copy; it is
	// nonempty by design — the no-resume property is the journal's). The
	// crash-matrix wrapper is inert unless $RECOVERYDB_CRASHPOINT selects
	// the mid-insert-batch point.
	destDB, err := dbopen.OpenDestination(cfg.DestinationDB, false)
	if err != nil {
		return nil, err
	}
	cdb := newCrashDB(destDB)
	var db ethdb.Database = cdb
	closed := false
	defer func() {
		if !closed {
			db.Close()
		}
	}()

	// Destination identity: heads must equal the inspected baseline.
	if got := rawdb.ReadHeadBlockHash(db); got != baselineHash {
		return nil, fmt.Errorf("replay: destination head %s != inspected baseline %s (wrong copy?)", got.Hex(), baselineHash.Hex())
	}

	// Pre-mutation bundle pass: contiguity, parent chain from the baseline,
	// ordered digest, HasBlock=false for every record (ErrKnownBlock
	// semantics), all validated BEFORE the first insert.
	if err := prepassBundle(db, cfg, manifest, baselineHash); err != nil {
		return nil, err
	}

	// ---- Step 4: journal + offline chain. ----
	journal, err := report.CreateJournal(journalPath)
	if err != nil {
		return nil, err
	}
	defer journal.Close()

	bc, err := harness.OpenChain(db, cfg.Network, cfg.ShardID, harness.ModeReplay)
	if err != nil {
		return nil, err
	}
	chainConfig, err := harness.ChainConfig(cfg.Network, cfg.ShardID)
	if err != nil {
		return nil, err
	}

	rep := &report.ReplayReport{
		Destination:    cfg.DestinationDB,
		BaselineHeight: baselineHeight,
		BaselineHash:   baselineHash.Hex(),
		RangeFrom:      manifest.FromHeight,
		RangeTo:        manifest.ToHeight,
	}
	meta, err := report.NewMeta(report.ReplaySchemaV1, "replay-bundle", cfg.Network, cfg.ShardID, cfg.ToolVersion, inputs)
	if err != nil {
		return nil, err
	}
	rep.Meta = meta
	_ = anchorRef

	// ---- Step 5: per-record validate-then-insert. ----
	cdb.arm() // mid-insert-batch crash window opens with the first insert
	targetSig, err := insertLoop(db, bc, cfg, manifest, rep)
	if err != nil {
		return nil, err
	}
	report.CrashPoint("replay.after-inserts-before-finalize")

	// ---- Step 6: target certificate cross-checks. ----
	sidecarHdr, sidecarSig, sidecarBitmap, err := loadSidecar(cfg.BundleDir, manifest, anc)
	if err != nil {
		return nil, err
	}
	if err := bc.Engine().VerifyHeaderSignature(bc, targetHeaderOf(bc), sidecarSig96(sidecarSig), sidecarBitmap); err != nil {
		return nil, fmt.Errorf("replay: sidecar certificate does not verify over the target header: %w", err)
	}
	_ = sidecarHdr
	persisted, err := db.Get(keys.BlockSigKey(cfg.TargetHeight))
	if err != nil {
		return nil, fmt.Errorf("replay: read persisted block-sig-%d: %w", cfg.TargetHeight, err)
	}
	if !bytes.Equal(persisted, targetSig) {
		return nil, fmt.Errorf("replay: persisted target certificate differs from the manifest-pinned record certificate")
	}
	certSHA := integrity.BytesSHA256(persisted)
	if anc.TargetCertificateSHA256 != "" && anc.TargetCertificateSHA256 != certSHA {
		return nil, fmt.Errorf("replay: persisted target certificate sha256 %s != anchor %s", certSHA, anc.TargetCertificateSHA256)
	}
	rep.TargetCertificateSHA256 = certSHA

	if head := bc.CurrentBlock(); head.Hash() != anc.TargetHash {
		return nil, fmt.Errorf("replay: destination head %s does not match the pinned target hash %s at completion", head.Hash().Hex(), anc.TargetHash.Hex())
	}
	if err := postTargetSweep(db, cfg.TargetHeight, anc.AbandonedChildHash); err != nil {
		return nil, fmt.Errorf("replay: post-target sweep: %w", err)
	}

	// ---- Step 7: runtime-metadata cleanup, itemized. ----
	if err := runtimeCleanup(db, rep); err != nil {
		return nil, err
	}

	// ---- Step 8: strict finalizer (never BlockChainImpl.Stop). ----
	targetRoot := bc.CurrentBlock().Root()
	if err := finalize(db, bc, cfg, targetRoot, rep); err != nil {
		return nil, err
	}
	report.CrashPoint("replay.after-finalize-before-close")
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("replay: close destination: %w", err)
	}
	closed = true
	if err := report.FsyncWalk(report.OSFS, cfg.DestinationDB); err != nil {
		return nil, err
	}
	report.CrashPoint("replay.after-close-before-gate")

	// ---- Step 9: reopen read-only and run the replay gate. The journal
	// completes only after the gate passes AND replay.json is durably
	// written (fail-closed: a crash or gate failure leaves IN_PROGRESS ⇒
	// discard-and-rebuild; a COMPLETE_VERIFIED journal therefore always has
	// its replay.json). ----
	if err := runGate(cfg, anc, chainConfig, rep); err != nil {
		return nil, err
	}
	rep.WallSeconds = time.Since(start).Seconds()
	sum, err := report.WriteJSON(cfg.OutputPath, rep)
	if err != nil {
		return nil, err
	}
	report.CrashPoint("replay.after-report-before-journal")
	if err := journal.Complete(report.StateCompleteVerified, "replay gate passed; replay.json "+sum); err != nil {
		return nil, err
	}
	return rep, nil
}

func headTupleOf(inspect *report.InspectReport) (uint64, common.Hash, error) {
	if len(inspect.Heads) == 0 || !inspect.HeadsAgree {
		return 0, common.Hash{}, fmt.Errorf("replay: inspect report has no agreed head tuple")
	}
	h := inspect.Heads[0]
	return h.Height, common.HexToHash(h.Hash), nil
}

// prepassBundle streams every record before any mutation: heights contiguous
// [from..to], parent chain anchored at the baseline hash, ordered-hash
// digest equal to the manifest's, and no record already known to the chain.
func prepassBundle(db interface {
	Get([]byte) ([]byte, error)
	Has([]byte) (bool, error)
}, cfg Config, manifest *bundle.Manifest, baselineHash common.Hash) error {
	expected := manifest.FromHeight
	prevHash := baselineHash
	ordered := report.NewHasher("bundle.orderedHashes")
	err := streamBundle(cfg.BundleDir, manifest, func(rec *bundle.Record) error {
		if rec.Height != expected {
			return fmt.Errorf("replay: record order broken: got height %d, want %d (missing/reordered/duplicate record)", rec.Height, expected)
		}
		if rec.Height > cfg.TargetHeight {
			return fmt.Errorf("replay: bundle extends past the target (%d > %d)", rec.Height, cfg.TargetHeight)
		}
		if rec.ShardID != cfg.ShardID {
			return fmt.Errorf("replay: record %d is for shard %d", rec.Height, rec.ShardID)
		}
		if rec.ParentHash != prevHash {
			return fmt.Errorf("replay: record %d parent %s does not extend %s", rec.Height, rec.ParentHash.Hex(), prevHash.Hex())
		}
		// ErrKnownBlock semantics, checked for EVERY manifest hash before
		// any insert (plan WS4 step 2).
		if num, err := headerNumberOf(db, rec.Hash); err != nil {
			return err
		} else if num != nil {
			return fmt.Errorf("replay: block %s (height %d) already exists in the destination (ErrKnownBlock)", rec.Hash.Hex(), rec.Height)
		}
		ordered.Add(rec.Hash.Bytes())
		prevHash = rec.Hash
		expected++
		return nil
	})
	if err != nil {
		return err
	}
	if expected != manifest.ToHeight+1 {
		return fmt.Errorf("replay: bundle ends at %d, manifest promises %d (truncated bundle)", expected-1, manifest.ToHeight)
	}
	if got := ordered.Digest().SHA256; got != manifest.OrderedHashDigest {
		return fmt.Errorf("replay: ordered block-hash digest mismatch: %s vs manifest %s", got, manifest.OrderedHashDigest)
	}
	return nil
}

func headerNumberOf(db interface {
	Get([]byte) ([]byte, error)
	Has([]byte) (bool, error)
}, hash common.Hash) (*uint64, error) {
	key := keys.HeaderNumberKey(hash)
	ok, err := db.Has(key)
	if err != nil {
		return nil, fmt.Errorf("replay: probe header number: %w", err)
	}
	if !ok {
		return nil, nil
	}
	val, err := db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("replay: read header number: %w", err)
	}
	if len(val) != 8 {
		return nil, fmt.Errorf("replay: malformed header-number entry for %s", hash.Hex())
	}
	n := new(uint64)
	*n = uint64(val[0])<<56 | uint64(val[1])<<48 | uint64(val[2])<<40 | uint64(val[3])<<32 |
		uint64(val[4])<<24 | uint64(val[5])<<16 | uint64(val[6])<<8 | uint64(val[7])
	return n, nil
}

func streamBundle(dir string, manifest *bundle.Manifest, fn func(*bundle.Record) error) error {
	for _, c := range manifest.Chunks {
		f, err := os.Open(filepath.Join(dir, c.Name))
		if err != nil {
			return fmt.Errorf("replay: open chunk %s: %w", c.Name, err)
		}
		r := bufio.NewReaderSize(f, 1<<20)
		count := uint64(0)
		for {
			rec, err := bundle.ReadFrame(r)
			if err == bundle.ErrEndOfChunk {
				break
			}
			if err != nil {
				f.Close()
				return err
			}
			count++
			if err := fn(rec); err != nil {
				f.Close()
				return err
			}
		}
		f.Close()
		if count != c.Records {
			return fmt.Errorf("replay: chunk %s has %d records, manifest says %d", c.Name, count, c.Records)
		}
	}
	return nil
}

func insertLoop(db interface {
	Get([]byte) ([]byte, error)
	Has([]byte) (bool, error)
}, bc core.BlockChain, cfg Config, manifest *bundle.Manifest, rep *report.ReplayReport) ([]byte, error) {
	var targetSig []byte
	blockCount := uint64(0)
	err := streamBundle(cfg.BundleDir, manifest, func(rec *bundle.Record) error {
		head := bc.CurrentBlock()
		if rec.Height != head.NumberU64()+1 {
			return fmt.Errorf("replay: record %d does not extend head %d", rec.Height, head.NumberU64())
		}
		if rec.ParentHash != head.Hash() {
			return fmt.Errorf("replay: record %d parent %s != head hash %s", rec.Height, rec.ParentHash.Hex(), head.Hash().Hex())
		}
		if rec.Height > cfg.TargetHeight {
			return fmt.Errorf("replay: record %d beyond target", rec.Height)
		}
		blk, sigAndBitmap, err := rec.DecodeBlock()
		if err != nil {
			return err
		}
		sig, bitmap, err := chain.ParseCommitSigAndBitmap(sigAndBitmap)
		if err != nil {
			return fmt.Errorf("replay: parse certificate of %d: %w", rec.Height, err)
		}
		// The block's own certificate, verified against the destination
		// chain's committee before any insert (plan WS4 step 5).
		if err := bc.Engine().VerifyHeaderSignature(bc, blk.Header(), sig, bitmap); err != nil {
			return fmt.Errorf("replay: block %d certificate: %w", rec.Height, err)
		}
		blk.SetCurrentCommitSig(sigAndBitmap) // persisted via rawdb.WriteBlock in the insert batch (§2.1)
		if err := bc.ValidateNewBlock(blk, bc); err != nil {
			return fmt.Errorf("replay: ValidateNewBlock(%d): %w", rec.Height, err)
		}
		if _, err := bc.InsertChain(types.Blocks{blk}, false); err != nil {
			return fmt.Errorf("replay: InsertChain(%d): %w", rec.Height, err)
		}
		// Post-insert assertions (plan WS4 step 5).
		if got := bc.CurrentBlock().Hash(); got != rec.Hash {
			return fmt.Errorf("replay: head after insert is %s, want %s", got.Hex(), rec.Hash.Hex())
		}
		if ch := rawdb.ReadCanonicalHash(bc.ChainDb(), rec.Height); ch != rec.Hash {
			return fmt.Errorf("replay: canonical(%d) = %s, want %s", rec.Height, ch.Hex(), rec.Hash.Hex())
		}
		if root := bc.CurrentBlock().Root(); root != rec.StateRoot {
			return fmt.Errorf("replay: state root after %d is %s, record says %s", rec.Height, root.Hex(), rec.StateRoot.Hex())
		}
		persisted, err := db.Get(keys.BlockSigKey(rec.Height))
		if err != nil {
			return fmt.Errorf("replay: exact block-sig-%d unreadable after insert: %w", rec.Height, err)
		}
		if !bytes.Equal(persisted, sigAndBitmap) {
			return fmt.Errorf("replay: persisted block-sig-%d differs from the record certificate", rec.Height)
		}
		if rec.Height == cfg.TargetHeight {
			targetSig = append([]byte{}, sigAndBitmap...)
		}
		blockCount++
		// Deterministic mid-batch crash point (fires on the first armed
		// insert — a partially replayed destination).
		report.CrashPoint("replay.mid-insert")
		if blockCount%1000 == 0 {
			free, err := dbopen.FreeSpace(filepath.Dir(cfg.DestinationDB))
			if err != nil {
				return fmt.Errorf("replay: free-space check: %w", err)
			}
			if free < cfg.MinFreeBytes {
				return fmt.Errorf("replay: free space %d fell below --min-free-bytes %d at height %d", free, cfg.MinFreeBytes, rec.Height)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	rep.BlocksReplayed = blockCount
	if targetSig == nil {
		return nil, fmt.Errorf("replay: bundle never reached the target height")
	}
	return targetSig, nil
}

func targetHeaderOf(bc core.BlockChain) *block.Header {
	return bc.CurrentBlock().Header()
}

func sidecarSig96(sig []byte) (out [96]byte) {
	copy(out[:], sig)
	return out
}

func loadSidecar(dir string, manifest *bundle.Manifest, anc *anchor.Manifest) (*block.Header, []byte, []byte, error) {
	raw, err := os.ReadFile(bundle.SidecarPath(dir))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("replay: read sidecar: %w", err)
	}
	var hdr block.Header
	if err := rlp.DecodeBytes(raw, &hdr); err != nil {
		return nil, nil, nil, fmt.Errorf("replay: decode sidecar header: %w", err)
	}
	if hdr.Hash() != anc.AbandonedChildHash {
		return nil, nil, nil, fmt.Errorf("replay: sidecar header hash %s != ABANDONED_CHILD_HASH %s", hdr.Hash().Hex(), anc.AbandonedChildHash.Hex())
	}
	if hdr.ParentHash() != anc.TargetHash {
		return nil, nil, nil, fmt.Errorf("replay: sidecar parent %s != pinned target hash %s", hdr.ParentHash().Hex(), anc.TargetHash.Hex())
	}
	if manifest.Sidecar.ChildHash != hdr.Hash().Hex() {
		return nil, nil, nil, fmt.Errorf("replay: sidecar header does not match manifest")
	}
	sig := hdr.LastCommitSignature()
	return &hdr, sig[:], hdr.LastCommitBitmap(), nil
}

func runtimeCleanup(db interface {
	Has([]byte) (bool, error)
	Delete([]byte) error
}, rep *report.ReplayReport) error {
	markers := []struct {
		name string
		key  []byte
	}{
		{"LastPivot", keys.LastPivotKey},
		{"TrieSync", keys.TrieSyncKey},
		{"SnapshotDisabled", keys.SnapshotDisabledKey},
		{"SnapshotRoot", keys.SnapshotRootKey},
		{"SnapshotJournal", keys.SnapshotJournalKey},
		{"SnapshotGenerator", keys.SnapshotGeneratorKey},
		{"SnapshotRecovery", keys.SnapshotRecoveryKey},
		{"SnapshotSyncStatus", keys.SnapshotSyncStatusKey},
		{"SkeletonSyncStatus", keys.SkeletonSyncStatusKey},
		{"TransactionIndexTail", keys.TxIndexTailKey},
		{"FastTransactionLookupLimit", keys.FastTxLookupLimitKey},
		{"unclean-shutdown", keys.UncleanShutdownKey},
		{"InvalidBlock", keys.BadBlockKey},
		{"SnapdbInfo", keys.SnapdbInfoKey},
		{"eth2-transition", keys.Eth2TransitionKey},
	}
	for _, m := range markers {
		present, err := db.Has(m.key)
		if err != nil {
			return fmt.Errorf("replay: probe %s: %w", m.name, err)
		}
		if present {
			if err := db.Delete(m.key); err != nil {
				return fmt.Errorf("replay: delete %s: %w", m.name, err)
			}
			rep.RuntimeCleanup = append(rep.RuntimeCleanup, m.name)
		}
	}
	return nil
}

func finalize(db interface {
	Has([]byte) (bool, error)
	Delete([]byte) error
}, bc core.BlockChain, cfg Config, targetRoot common.Hash, rep *report.ReplayReport) error {
	// (a) Pending-queue clear — aligned with in-place §2.2 ("clear
	// intentionally"): delete both stored keys with checked writes
	// regardless of the in-memory cache state insertion may have populated.
	clPresent, err := db.Has(keys.PendingCrosslinkKey)
	if err != nil {
		return fmt.Errorf("replay: probe pendingCL: %w", err)
	}
	scPresent, err := db.Has(keys.PendingSlashingKey)
	if err != nil {
		return fmt.Errorf("replay: probe pendingSC: %w", err)
	}
	if clPresent {
		if err := db.Delete(keys.PendingCrosslinkKey); err != nil {
			return fmt.Errorf("replay: delete pendingCL: %w", err)
		}
	}
	if scPresent {
		if err := db.Delete(keys.PendingSlashingKey); err != nil {
			return fmt.Errorf("replay: delete pendingSC: %w", err)
		}
	}
	rep.PendingQueues = report.PendingQueueClear{
		CrosslinkKeyWasPresent: clPresent,
		SlashingKeyWasPresent:  scPresent,
		Cleared:                true,
	}

	// (b) Commit the target trie.
	if err := bc.TrieDB().Commit(targetRoot, true); err != nil {
		return fmt.Errorf("replay: TrieDB.Commit(%s): %w", targetRoot.Hex(), err)
	}
	// Deterministic crash window between the trie commit and the preimage
	// flush (round 14 finding 3): trie nodes durable, preimage coverage not.
	report.CrashPoint("replay.after-trie-commit-before-preimages")
	// (c) Preimage flush, checked (the pair Stop runs fail-open).
	if err := bc.CommitPreimages(); err != nil {
		return fmt.Errorf("replay: CommitPreimages: %w", err)
	}
	if _, _, err := rawdb.WritePreImageStartEndBlock(bc.ChainDb(), 0, cfg.TargetHeight); err != nil {
		return fmt.Errorf("replay: WritePreImageStartEndBlock: %w", err)
	}
	return nil
}

// postTargetSweep asserts the exact keys that must be absent above the
// target, with CHECKED probes: a read error is surfaced as a gate failure,
// never collapsed into absence the way rawdb.ReadCanonicalHash /
// ReadHeaderNumber collapse it (round 14 finding 1 — a damaged DB must not
// reach COMPLETE_VERIFIED through fail-open reads).
func postTargetSweep(db interface {
	Has([]byte) (bool, error)
}, target uint64, abandonedChild common.Hash) error {
	for n := target + 1; n <= target+8; n++ {
		for _, probe := range []struct {
			name string
			key  []byte
		}{
			{"canonical mapping", keys.CanonicalHashKey(n)},
			{"block-sig", keys.BlockSigKey(n)},
		} {
			present, err := db.Has(probe.key)
			if err != nil {
				return fmt.Errorf("probe %s at %d: %w", probe.name, n, err)
			}
			if present {
				return fmt.Errorf("%s present at %d", probe.name, n)
			}
		}
	}
	present, err := db.Has(keys.HeaderNumberKey(abandonedChild))
	if err != nil {
		return fmt.Errorf("probe abandoned-child header-number: %w", err)
	}
	if present {
		return fmt.Errorf("abandoned child header-number entry present")
	}
	return nil
}

// runGate reopens the destination strictly read-only and runs the §9.4
// replay gate, populating the report's gate checks and DigestSet.
func runGate(cfg Config, anc *anchor.Manifest, chainConfig *params.ChainConfig, rep *report.ReplayReport) error {
	db, ro, err := dbopen.OpenSourceDatabase(cfg.DestinationDB)
	if err != nil {
		return fmt.Errorf("replay: gate reopen: %w", err)
	}
	defer ro.Close()

	fail := func(id, format string, args ...interface{}) error {
		rep.Gate.Checks = append(rep.Gate.Checks, report.Check{ID: id, OK: false, Detail: fmt.Sprintf(format, args...)})
		return fmt.Errorf("replay gate %s: "+format, append([]interface{}{id}, args...)...)
	}
	pass := func(id string) {
		rep.Gate.Checks = append(rep.Gate.Checks, report.Check{ID: id, OK: true})
	}

	// Three heads == pinned target hash (LastFinalized is stock-absent on
	// the working copy; compact-db writes it on the artifact).
	for _, hk := range [][]byte{keys.HeadBlockKey, keys.HeadHeaderKey, keys.HeadFastBlockKey} {
		val, err := db.Get(hk)
		if err != nil {
			return fail("gate.heads", "read %s: %v", hk, err)
		}
		if common.BytesToHash(val) != anc.TargetHash {
			return fail("gate.heads", "%s = %x, want %s", hk, val, anc.TargetHash.Hex())
		}
	}
	pass("gate.heads")

	ch := rawdb.ReadCanonicalHash(db, cfg.TargetHeight)
	if ch != anc.TargetHash {
		return fail("gate.canonical", "canonical(%d) = %s", cfg.TargetHeight, ch.Hex())
	}
	hdr := rawdb.ReadHeader(db, ch, cfg.TargetHeight)
	if hdr == nil {
		return fail("gate.canonical", "target header missing")
	}
	pass("gate.canonical")

	// Target certificate from the exact key; the legacy LastCommits
	// fallback is rejected by construction (exact-key read only).
	sigVal, err := db.Get(keys.BlockSigKey(cfg.TargetHeight))
	if err != nil {
		return fail("gate.target-cert", "exact block-sig-%d unreadable: %v", cfg.TargetHeight, err)
	}
	cv := verify.NewCertVerifier(db, chainConfig, cfg.ShardID)
	if err := cv.VerifyCommitSigBytes(hdr, sigVal); err != nil {
		return fail("gate.target-cert", "%v", err)
	}
	pass("gate.target-cert")

	// No repair/rewind markers, no post-target records in
	// consensus-relevant tables (spot checks: full raw scans of the multi-TB
	// working copy are the compact artifact's job).
	for _, m := range []struct {
		name string
		key  []byte
	}{
		{"unclean-shutdown", keys.UncleanShutdownKey},
		{"InvalidBlock", keys.BadBlockKey},
		{"SnapdbInfo", keys.SnapdbInfoKey},
		{"LastPivot", keys.LastPivotKey},
		{"SkeletonSyncStatus", keys.SkeletonSyncStatusKey},
		{"SnapshotJournal", keys.SnapshotJournalKey},
		{"SnapshotRecovery", keys.SnapshotRecoveryKey},
		{"pendingCL", keys.PendingCrosslinkKey},
		{"pendingSC", keys.PendingSlashingKey},
	} {
		ok, err := db.Has(m.key)
		if err != nil {
			return fail("gate.markers", "probe %s: %v", m.name, err)
		}
		if ok {
			return fail("gate.markers", "%s marker present after cleanup", m.name)
		}
	}
	if err := postTargetSweep(db, cfg.TargetHeight, anc.AbandonedChildHash); err != nil {
		return fail("gate.post-target", "%v", err)
	}
	pass("gate.markers")
	pass("gate.post-target")

	// Full state traversal at the target root; preimage coverage required
	// on the archival working copy.
	walk, err := verify.WalkState(db, hdr.Root(), verify.StateWalkOptions{
		CheckPreimages:   true,
		RequirePreimages: true,
	})
	if err != nil {
		return fail("gate.state", "%v", err)
	}
	pass("gate.state")

	sched, err := harness.Schedule(cfg.Network)
	if err != nil {
		return err
	}
	win, err := anchor.ComputeWindow(sched, cfg.TargetHeight, 0)
	if err != nil {
		return err
	}
	off, err := verify.ComputeOffchainDigests(db, report.DigestWindow{RetainFrom: win.RetainFrom, Target: win.Target})
	if err != nil {
		return fail("gate.offchain", "%v", err)
	}
	pass("gate.offchain")

	rep.DigestSet = verify.BuildDigestSet(cfg.Network, cfg.ShardID, cfg.TargetHeight, anc.TargetHash, hdr.Root(),
		report.DigestWindow{RetainFrom: win.RetainFrom, Target: win.Target}, walk, off)

	for _, hk := range []struct {
		name string
		key  []byte
	}{{"LastBlock", keys.HeadBlockKey}, {"LastHeader", keys.HeadHeaderKey}, {"LastFast", keys.HeadFastBlockKey}} {
		rep.FinalHeads = append(rep.FinalHeads, report.HeadTuple{
			Key:       hk.name,
			Hash:      anc.TargetHash.Hex(),
			Height:    cfg.TargetHeight,
			Epoch:     hdr.Epoch().Uint64(),
			ViewID:    hdr.ViewID().Uint64(),
			StateRoot: hdr.Root().Hex(),
		})
	}
	rep.Gate.Passed = true
	return nil
}
