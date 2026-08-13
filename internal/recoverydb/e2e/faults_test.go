package e2e

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/bundle"
	"github.com/harmony-one/harmony/internal/recoverydb/compact"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/fixture"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/inspect"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/release"
	"github.com/harmony-one/harmony/internal/recoverydb/replay"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

// sharedWorld builds the fixture kit + inspect/agreement/bundle + a good
// compact build ONCE for the whole fault matrix.
type sharedWorld struct {
	k          *kit
	inspectA   string
	agreement  string
	bundleDir  string
	replayed   string // replayed working copy (baseline A after replay)
	replayJSON string
	compactDir string
	compactOut string
	compactRep *report.CompactReport
	window     anchor.Window
}

var (
	worldOnce sync.Once
	world     *sharedWorld
	worldErr  error
)

func getWorld(t *testing.T) *sharedWorld {
	t.Helper()
	worldOnce.Do(func() {
		worldErr = buildWorld()
	})
	if worldErr != nil {
		t.Fatalf("shared world: %v", worldErr)
	}
	return world
}

var worldRoot string

func buildWorld() error {
	root, err := os.MkdirTemp("", "recoverydb-world-*")
	if err != nil {
		return err
	}
	worldRoot = root
	k := &kit{root: root}
	k.donorDir = filepath.Join(root, "donor", "harmony_db_0")

	c, err := fixture.Open(k.donorDir, fixture.RepoKeysDir())
	if err != nil {
		return err
	}
	if err := c.Generate(fixture.Params{Blocks: baselineHeight, TxEvery: 5, DeployContractAt: 6, CreateValidatorAt: 9, DelegateAt: 11}); err != nil {
		return err
	}
	if err := c.Finalize(); err != nil {
		return err
	}
	k.baseA = filepath.Join(root, "baseline-a", "harmony_db_0")
	k.baseB = filepath.Join(root, "baseline-b", "harmony_db_0")
	if err := fixture.CopyDir(k.donorDir, k.baseA); err != nil {
		return err
	}
	if err := fixture.CopyDir(k.donorDir, k.baseB); err != nil {
		return err
	}
	c, err = fixture.Open(k.donorDir, fixture.RepoKeysDir())
	if err != nil {
		return err
	}
	if err := c.Generate(fixture.Params{Blocks: donorHeight - baselineHeight, TxEvery: 5}); err != nil {
		return err
	}
	if err := c.Finalize(); err != nil {
		return err
	}

	db, ro, err := dbopen.OpenSourceDatabase(k.donorDir)
	if err != nil {
		return err
	}
	k.targetHash = rawdb.ReadCanonicalHash(db, targetHeight)
	tHdr := rawdb.ReadHeader(db, k.targetHash, targetHeight)
	k.parentHash = tHdr.ParentHash()
	k.targetEpoch = tHdr.Epoch().Uint64()
	k.childHash = rawdb.ReadCanonicalHash(db, targetHeight+1)
	ro.Close()

	m := &anchor.Manifest{
		SchemaVersion: anchor.SchemaVersionV1, Network: "localnet", ShardID: 0,
		TargetHeight: targetHeight, TargetHash: k.targetHash, TargetParentHash: k.parentHash,
		TargetEpoch: k.targetEpoch, BaselineHeight: baselineHeight,
		AbandonedChildHeight: targetHeight + 1, AbandonedChildHash: k.childHash,
	}
	k.anchorPath = filepath.Join(root, "anchor.json")
	if err := writeJSONSum(k.anchorPath, m); err != nil {
		return err
	}

	// inspect both + agreement.
	inspectA := filepath.Join(root, "inspect-a.json")
	repA, _, err := inspect.Run(inspect.Params{
		Network: "localnet", ShardID: 0, DBPath: k.baseA,
		FullState: true, FullOffchain: true, RequirePreimages: true,
		TargetHeight: targetHeight, AnchorPath: k.anchorPath,
		Output: inspectA, ToolVersion: toolVersion,
	})
	if err != nil {
		return err
	}
	if inspect.Failed(repA) {
		return fmt.Errorf("inspect A failed: %+v", repA.Checks)
	}
	inspectB := filepath.Join(root, "inspect-b.json")
	if _, _, err := inspect.Run(inspect.Params{
		Network: "localnet", ShardID: 0, DBPath: k.baseB,
		FullState: true, FullOffchain: true, RequirePreimages: true,
		TargetHeight: targetHeight, AnchorPath: k.anchorPath,
		Output: inspectB, ToolVersion: toolVersion,
	}); err != nil {
		return err
	}
	agreementPath := filepath.Join(root, "agreement.json")
	verdict, err := inspect.Agreement("localnet", 0, toolVersion, inspectA, inspectB, agreementPath)
	if err != nil {
		return err
	}
	if !verdict.Agreed {
		return fmt.Errorf("agreement failed: %v", verdict.Differences)
	}

	// export.
	if _, err := harness.InitSchedule("localnet"); err != nil {
		return err
	}
	chainCfg, err := harness.ChainConfig("localnet", 0)
	if err != nil {
		return err
	}
	donorDB, donorRO, err := dbopen.OpenSourceDatabase(k.donorDir)
	if err != nil {
		return err
	}
	anc, err := anchor.Load(k.anchorPath)
	if err != nil {
		return err
	}
	bundleDir := filepath.Join(root, "bundle")
	if _, err := bundle.Export(donorDB, bundle.ExportConfig{
		Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
		FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
		BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(repA.Heads[0].Hash),
		Anchor: anc, OutputDir: bundleDir, ChunkBytes: 1 << 20,
		Donor: "fixture-donor", ToolVersion: toolVersion,
	}); err != nil {
		donorRO.Close()
		return err
	}
	donorRO.Close()

	// replay into baseline A.
	replayOut := filepath.Join(root, "replay.json")
	if _, err := replay.Run(replay.Config{
		Network: "localnet", ShardID: 0, DestinationDB: k.baseA,
		AnchorPath: k.anchorPath, InspectReportPath: inspectA,
		BaselineAgreementPath: agreementPath, BundleDir: bundleDir,
		TargetHeight: targetHeight, ToolVersion: toolVersion, OutputPath: replayOut,
	}); err != nil {
		return err
	}

	// compact (internal mode).
	sched, _ := harness.Schedule("localnet")
	window, err := anchor.ComputeWindow(sched, targetHeight, 0)
	if err != nil {
		return err
	}
	compactDir := filepath.Join(root, "compact", "harmony_db_0")
	compactOut := filepath.Join(root, "compact.json")
	compactRep, err := compact.Run(compact.Config{
		Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
		SourceDB: k.baseA, DestinationDB: compactDir,
		AnchorPath: k.anchorPath, SourceReferencePath: replayOut,
		TargetHeight: targetHeight, ToolVersion: toolVersion, OutputPath: compactOut,
	}, window)
	if err != nil {
		return err
	}

	world = &sharedWorld{
		k: k, inspectA: inspectA, agreement: agreementPath, bundleDir: bundleDir,
		replayed: k.baseA, replayJSON: replayOut,
		compactDir: compactDir, compactOut: compactOut, compactRep: compactRep,
		window: window,
	}
	return nil
}

func writeJSONSum(path string, v interface{}) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}
	_, err = integrity.WriteChecksumFile(path)
	return err
}

// freshBaseline copies the pristine baseline B for a destructive replay test.
func freshBaseline(t *testing.T, w *sharedWorld) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := fixture.CopyDir(w.k.baseB, dst); err != nil {
		t.Fatal(err)
	}
	// The fresh copy is byte-identical to B (and to pre-replay A), so the
	// A-inspect report and agreement verdict apply to it.
	return dst
}

func replayInto(w *sharedWorld, dest, bundleDir, anchorPath, out string) error {
	_, err := replay.Run(replay.Config{
		Network: "localnet", ShardID: 0, DestinationDB: dest,
		AnchorPath: anchorPath, InspectReportPath: w.inspectA,
		BaselineAgreementPath: w.agreement, BundleDir: bundleDir,
		TargetHeight: targetHeight, ToolVersion: toolVersion, OutputPath: out,
	})
	return err
}

// ---- replay fault matrix (plan WS4 acceptance) ----

func TestReplayFaults(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)

	t.Run("corruptedAnchorChecksum", func(t *testing.T) {
		dest := freshBaseline(t, w)
		badAnchor := filepath.Join(t.TempDir(), "anchor.json")
		raw, _ := os.ReadFile(w.k.anchorPath)
		os.WriteFile(badAnchor, raw, 0o644)
		integrity.WriteChecksumFile(badAnchor)
		// Corrupt after checksumming.
		raw[len(raw)-2] ^= 0xff
		os.WriteFile(badAnchor, raw, 0o644)
		err := replayInto(w, dest, w.bundleDir, badAnchor, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("want checksum gate failure, got %v", err)
		}
	})

	t.Run("corruptedChunk", func(t *testing.T) {
		dest := freshBaseline(t, w)
		dir := copyBundle(t, w.bundleDir)
		chunk := filepath.Join(dir, bundle.ChunkName(0))
		raw, _ := os.ReadFile(chunk)
		raw[len(raw)/2] ^= 0x01
		os.WriteFile(chunk, raw, 0o644)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "checksum gate failed") {
			t.Fatalf("want chunk checksum failure, got %v", err)
		}
	})

	t.Run("truncatedChunk", func(t *testing.T) {
		dest := freshBaseline(t, w)
		dir := copyBundle(t, w.bundleDir)
		chunk := filepath.Join(dir, bundle.ChunkName(0))
		fi, _ := os.Stat(chunk)
		os.Truncate(chunk, fi.Size()-7)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "checksum gate failed") {
			t.Fatalf("want truncation failure, got %v", err)
		}
	})

	t.Run("bundlePastTarget", func(t *testing.T) {
		// Export without the anchor up to target+1; the range gate must
		// reject a bundle extending past the target outright.
		donorDB, donorRO, err := dbopen.OpenSourceDatabase(w.k.donorDir)
		if err != nil {
			t.Fatal(err)
		}
		defer donorRO.Close()
		chainCfg, _ := harness.ChainConfig("localnet", 0)
		longDir := filepath.Join(t.TempDir(), "bundle-long")
		if _, err := bundle.Export(donorDB, bundle.ExportConfig{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			FromHeight: baselineHeight + 1, ToHeight: targetHeight + 1, CertChildHeight: targetHeight + 2,
			BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
			OutputDir: longDir, ChunkBytes: 1 << 20, Donor: "fixture", ToolVersion: toolVersion,
		}); err != nil {
			t.Fatal(err)
		}
		dest := freshBaseline(t, w)
		err = replayInto(w, dest, longDir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "must equal exactly") {
			t.Fatalf("want exact-range refusal, got %v", err)
		}
	})

	t.Run("plantedPreexistingBlock", func(t *testing.T) {
		dest := freshBaseline(t, w)
		// Plant the header-number entry of bundle block baseline+1.
		firstHash := bundleRecordHash(t, w.bundleDir, 0)
		wdb, err := rawdb.NewLevelDBDatabase(dest, 16, 64, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := wdb.Put(keys.HeaderNumberKey(firstHash), keys.Uint64BE(baselineHeight+1)); err != nil {
			t.Fatal(err)
		}
		wdb.Close()
		err = replayInto(w, dest, w.bundleDir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("want ErrKnownBlock-semantics refusal, got %v", err)
		}
	})

	t.Run("wrongParentRecord", func(t *testing.T) {
		dest := freshBaseline(t, w)
		dir := tamperBundle(t, w, func(recs []*bundle.Record) {
			recs[1].ParentHash = common.HexToHash("0xdead")
		}, false)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "does not extend") {
			t.Fatalf("want parent-chain refusal, got %v", err)
		}
	})

	t.Run("wrongCertificate", func(t *testing.T) {
		dest := freshBaseline(t, w)
		dir := tamperBundle(t, w, func(recs []*bundle.Record) {
			// Swap record 0's certificate with record 1's (both valid
			// aggregates, wrong block).
			b0, _, err := recs[0].DecodeBlock()
			if err != nil {
				t.Fatal(err)
			}
			_, sig1, err := recs[1].DecodeBlock()
			if err != nil {
				t.Fatal(err)
			}
			b0.SetCurrentCommitSig(sig1)
			raw, err := rlp.EncodeToBytes(core.BlockWithSig{Block: b0, CommitSigAndBitmap: sig1})
			if err != nil {
				t.Fatal(err)
			}
			recs[0].BlockWithSigRLP = raw
		}, false)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "certificate") {
			t.Fatalf("want certificate refusal, got %v", err)
		}
	})

	t.Run("tamperedBodyBytes", func(t *testing.T) {
		dest := freshBaseline(t, w)
		dir := tamperBundle(t, w, func(recs []*bundle.Record) {
			raw := recs[0].BlockWithSigRLP
			raw[len(raw)-3] ^= 0xff
		}, false)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil {
			t.Fatalf("tampered record bytes must refuse")
		}
	})

	t.Run("invalidVRFResigned", func(t *testing.T) {
		// Tamper the VRF of the LAST record (the target — nothing chains to
		// it), re-sign with the real committee so BOTH certificate layers
		// pass; ValidateNewBlock must still reject (the two-layer defense).
		dest := freshBaseline(t, w)
		dir := tamperBundle(t, w, func(recs []*bundle.Record) {
			resignWithMutation(t, w, recs, len(recs)-1, func(h *block.Header) {
				v := h.Vrf()
				v[0] ^= 0xff
				h.SetVrf(v)
			})
		}, true)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "ValidateNewBlock") {
			t.Fatalf("want ValidateNewBlock VRF refusal, got %v", err)
		}
	})

	t.Run("wrongStateRootResigned", func(t *testing.T) {
		dest := freshBaseline(t, w)
		dir := tamperBundle(t, w, func(recs []*bundle.Record) {
			resignWithMutation(t, w, recs, len(recs)-1, func(h *block.Header) {
				h.SetRoot(common.HexToHash("0xbadbadbad"))
			})
		}, true)
		err := replayInto(w, dest, dir, w.k.anchorPath, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "ValidateNewBlock") {
			t.Fatalf("want ValidateNewBlock root refusal, got %v", err)
		}
	})

	t.Run("sidecarAnchorMismatch", func(t *testing.T) {
		dest := freshBaseline(t, w)
		badAnchor := filepath.Join(t.TempDir(), "anchor.json")
		m, _ := anchor.Load(w.k.anchorPath)
		m.AbandonedChildHash = common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555")
		if err := writeJSONSum(badAnchor, m); err != nil {
			t.Fatal(err)
		}
		err := replayInto(w, dest, w.bundleDir, badAnchor, filepath.Join(t.TempDir(), "r.json"))
		if err == nil || !strings.Contains(err.Error(), "ABANDONED_CHILD_HASH") {
			t.Fatalf("want sidecar/anchor mismatch, got %v", err)
		}
	})
}

func headHashOf(t *testing.T, w *sharedWorld) string {
	t.Helper()
	var rep report.InspectReport
	if err := report.ReadJSONStrict(w.inspectA, &rep); err != nil {
		t.Fatal(err)
	}
	return rep.Heads[0].Hash
}

func copyBundle(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "bundle")
	if err := fixture.CopyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func bundleRecordHash(t *testing.T, dir string, idx int) common.Hash {
	t.Helper()
	recs := readAllRecords(t, dir)
	return recs[idx].Hash
}

func readAllRecords(t *testing.T, dir string) []*bundle.Record {
	t.Helper()
	manifest, _, err := bundle.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	var recs []*bundle.Record
	for _, c := range manifest.Chunks {
		f, err := os.Open(filepath.Join(dir, c.Name))
		if err != nil {
			t.Fatal(err)
		}
		r := bufio.NewReader(f)
		for {
			rec, err := bundle.ReadFrame(r)
			if err == bundle.ErrEndOfChunk {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			recs = append(recs, rec)
		}
		f.Close()
	}
	return recs
}

// tamperBundle rewrites the bundle with mutated records, re-hashing chunks
// and the manifest so the checksum gates pass and the fault surfaces at the
// intended layer. When rehashOrdered is true the ordered-hash digest is
// recomputed from the (possibly re-hashed) records.
func tamperBundle(t *testing.T, w *sharedWorld, mutate func([]*bundle.Record), rehashOrdered bool) string {
	t.Helper()
	dir := copyBundle(t, w.bundleDir)
	manifest, _, err := bundle.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	recs := readAllRecords(t, dir)
	mutate(recs)

	// Rewrite everything into a single chunk for simplicity.
	for _, c := range manifest.Chunks {
		os.Remove(filepath.Join(dir, c.Name))
	}
	chunkPath := filepath.Join(dir, bundle.ChunkName(0))
	f, err := os.Create(chunkPath)
	if err != nil {
		t.Fatal(err)
	}
	wtr := bufio.NewWriter(f)
	var total uint64
	first, last := recs[0].Height, recs[len(recs)-1].Height
	for _, rec := range recs {
		n, err := bundle.WriteFrame(wtr, rec)
		if err != nil {
			t.Fatal(err)
		}
		total += uint64(n)
	}
	wtr.Flush()
	f.Close()
	sum, err := integrity.FileSHA256(chunkPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Chunks = []bundle.ChunkInfo{{
		Name: bundle.ChunkName(0), SHA256: sum, Records: uint64(len(recs)),
		FirstHeight: first, LastHeight: last, Bytes: total,
	}}
	if rehashOrdered {
		h := report.NewHasher("bundle.orderedHashes")
		for _, rec := range recs {
			h.Add(rec.Hash.Bytes())
		}
		manifest.OrderedHashDigest = h.Digest().SHA256
	}
	if _, err := report.WriteJSON(bundle.ManifestPath(dir), manifest); err != nil {
		t.Fatal(err)
	}
	return dir
}

// committeeSigners loads the shard-0 committee secrets for the given epoch
// from the donor (dev keys + persisted fixture-validator key).
func committeeSigners(t *testing.T, w *sharedWorld, epoch *big.Int) []bls2.PrivateKeyWrapper {
	t.Helper()
	db, ro, err := dbopen.OpenSourceDatabase(w.k.donorDir)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	ss, err := rawdb.ReadShardState(db, epoch)
	if err != nil {
		t.Fatal(err)
	}
	comm, err := ss.FindCommitteeByID(0)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := fixture.SlotKeys(comm.Slots, fixture.RepoKeysDir(), fixture.ExtraKeysDir(w.k.donorDir))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// resignWithMutation mutates a record's header, recomputes its hash and
// re-signs it with the real fixture committee, so certificate verification
// passes and only full validation can catch the defect.
func resignWithMutation(t *testing.T, w *sharedWorld, recs []*bundle.Record, idx int, mutate func(*block.Header)) {
	t.Helper()
	blk, _, err := recs[idx].DecodeBlock()
	if err != nil {
		t.Fatal(err)
	}
	hdr := blk.Header()
	mutate(hdr)
	newBlk := types.NewBlockWithHeader(hdr).WithBody(
		blk.Transactions(), blk.StakingTransactions(), blk.Uncles(), blk.IncomingReceipts(),
	)

	// Re-sign with the committee of the BLOCK'S epoch (the staking-era
	// committee differs from the baseline committee captured at world
	// build: elected slots, round 13 finding 9).
	signers := committeeSigners(t, w, blk.Epoch())
	pubs := make([]bls2.PublicKeyWrapper, len(signers))
	for i, kwr := range signers {
		pubs[i] = *kwr.Pub
	}
	mask := bls2.NewMask(pubs)
	chainCfg, _ := harness.ChainConfig("localnet", 0)
	payload := consensus_sig.ConstructCommitPayload(chainCfg, newBlk.Epoch(), newBlk.Hash(), newBlk.NumberU64(), newBlk.Header().ViewID().Uint64())
	var agg bls_core.Sign
	for i, kwr := range signers {
		if err := mask.SetBit(i, true); err != nil {
			t.Fatal(err)
		}
		agg.Add(kwr.Pri.SignHash(payload))
	}
	sigAndBitmap := append(agg.Serialize(), mask.Mask()...)
	newBlk.SetCurrentCommitSig(sigAndBitmap)
	raw, err := rlp.EncodeToBytes(core.BlockWithSig{Block: newBlk, CommitSigAndBitmap: sigAndBitmap})
	if err != nil {
		t.Fatal(err)
	}
	recs[idx].BlockWithSigRLP = raw
	recs[idx].Hash = newBlk.Hash()
	recs[idx].StateRoot = newBlk.Header().Root()
	// Later records now chain to the OLD hash; only record idx is tested.
}

// ---- verify-db seeded-defect matrix (plan WS6 acceptance) ----

func TestVerifySeededDefects(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)

	runVerify := func(t *testing.T, dir string, compactRep *report.CompactReport, manifestPath string) *verify.Result {
		t.Helper()
		roDB, ro, err := dbopen.OpenSourceDatabase(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		anc, err := anchor.Load(w.k.anchorPath)
		if err != nil {
			t.Fatal(err)
		}
		chainCfg, _ := harness.ChainConfig("localnet", 0)
		sched, _ := harness.Schedule("localnet")
		res, err := verify.Run(roDB, verify.Params{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			Anchor: anc, AnchorSHA256: fileSHA(t, w.k.anchorPath),
			Compact:                       compactRep,
			MetadataReferenceManifestPath: manifestPath,
			Window:                        w.window,
			TargetIsEpochLast:             sched.EpochLastBlock(w.window.Epoch) == w.window.Target,
			TempDir:                       t.TempDir(),
		})
		if err != nil {
			t.Fatalf("verify run: %v", err)
		}
		return res
	}

	seeded := func(t *testing.T, plant func(db interface {
		Put([]byte, []byte) error
		Delete([]byte) error
		Get([]byte) ([]byte, error)
	}), wantFail string) {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.compactDir, dir); err != nil {
			t.Fatal(err)
		}
		wdb, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
		if err != nil {
			t.Fatal(err)
		}
		plant(wdb)
		wdb.Close()
		res := runVerify(t, dir, w.compactRep, "")
		if res.Passed {
			t.Fatalf("seeded defect not detected (want %s to fail)", wantFail)
		}
		for _, c := range res.Checks {
			if c.ID == wantFail && !c.OK {
				return
			}
		}
		t.Fatalf("check %s did not fail; failures: %v", wantFail, failedIDs(res))
	}

	mid := (w.window.RetainFrom + w.window.Target) / 2

	t.Run("unmodifiedPasses", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.compactDir, dir); err != nil {
			t.Fatal(err)
		}
		res := runVerify(t, dir, w.compactRep, "")
		if !res.Passed {
			t.Fatalf("unmodified fixture must pass: %v", failedIDs(res))
		}
	})
	t.Run("staleFallbackOnlyCert", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			sig, _ := db.Get(keys.BlockSigKey(mid))
			db.Delete(keys.BlockSigKey(mid))
			db.Put(keys.LastCommitsKey, sig)
		}, verify.CheckWindowCerts)
	})
	t.Run("missingInverseMapping", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Delete(keys.HeaderNumberKey(w.k.targetHash))
		}, verify.CheckCanonicalTarget)
	})
	t.Run("plantedFutureLookup", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			entry := rawdb.LegacyTxLookupEntry{BlockHash: common.HexToHash("0x99"), BlockIndex: targetHeight + 5, Index: 0}
			raw, _ := rlp.EncodeToBytes(entry)
			db.Put(keys.TxLookupKey(common.HexToHash("0x1234")), raw)
		}, verify.CheckWindowLookups)
	})
	t.Run("plantedAbandonedChildEntry", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.HeaderNumberKey(w.k.childHash), keys.Uint64BE(targetHeight+1))
		}, verify.CheckAbandonedChild)
	})
	t.Run("stalePivot", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.LastPivotKey, []byte{1})
		}, verify.CheckRuntimeMarkers)
	})
	t.Run("forgedCXSpent", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.CxSpentKey(1, 12345), []byte{1})
		}, verify.CheckDigestMatch)
	})
	t.Run("plantedPendingQueue", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.PendingCrosslinkKey, []byte{1})
		}, verify.CheckPendingQueues)
	})
	t.Run("plantedValidatorStats", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.ValidatorStatsKey(common.HexToAddress("0x77")), []byte{1})
		}, verify.CheckValidatorStats)
	})
	t.Run("epochPlusOneShardState", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			// window.Epoch+2 is beyond even the epoch-last allowance.
			db.Put(keys.ShardStateKey(new(big.Int).SetUint64(w.window.Epoch+2)), []byte{1})
		}, verify.CheckAboveTarget)
	})
	t.Run("forkBlock", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			// A header at a legal height under a non-canonical hash.
			hdrRaw, _ := db.Get(keys.HeaderKey(mid, canonicalAt(t, w, mid)))
			db.Put(keys.HeaderKey(mid, common.HexToHash("0xfeedface")), hdrRaw)
		}, verify.CheckForks)
	})
	t.Run("corruptedMidWindowSig", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			sig, _ := db.Get(keys.BlockSigKey(mid))
			sig[3] ^= 0xff
			db.Put(keys.BlockSigKey(mid), sig)
		}, verify.CheckWindowCerts)
	})
	t.Run("unresolvedBareHash32Fatal", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(common.HexToHash("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes(), []byte{1})
		}, verify.CheckBareHash32)
	})
	t.Run("orphanPrefixedCode", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.CodeKey(common.HexToHash("0x4242")), []byte{0xde, 0xad})
		}, verify.CheckCodeOrphans)
	})
	t.Run("wrongHeads", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Put(keys.HeadFinalizedKey, common.HexToHash("0x1").Bytes())
		}, verify.CheckHeads)
	})
	// Half preimage-marker pairs (round 16 finding 2): write the valid
	// stock pair, then delete one half — the survivor alone must be
	// refused even though its value is individually correct.
	preimagePair := func(db interface {
		Put([]byte, []byte) error
		Delete([]byte) error
		Get([]byte) ([]byte, error)
	}) {
		start := make([]byte, 8)
		end := make([]byte, 8)
		binary.BigEndian.PutUint64(start, w.window.Target+1)
		binary.BigEndian.PutUint64(end, w.window.Target)
		if err := db.Put(keys.PreimageGenStartKey, start); err != nil {
			t.Fatal(err)
		}
		if err := db.Put(keys.PreimageGenEndKey, end); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("halfPreimagePairStartOnly", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			preimagePair(db)
			if err := db.Delete(keys.PreimageGenEndKey); err != nil {
				t.Fatal(err)
			}
		}, verify.CheckRuntimeMarkers)
	})
	t.Run("halfPreimagePairEndOnly", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			preimagePair(db)
			if err := db.Delete(keys.PreimageGenStartKey); err != nil {
				t.Fatal(err)
			}
		}, verify.CheckRuntimeMarkers)
	})
	t.Run("missingMarker", func(t *testing.T) {
		seeded(t, func(db interface {
			Put([]byte, []byte) error
			Delete([]byte) error
			Get([]byte) ([]byte, error)
		}) {
			db.Delete(keys.RecoveryMarkerKey)
		}, verify.CheckMarkerPresent)
	})
	t.Run("markerWrongReference", func(t *testing.T) {
		seeded(t, tamperMarker(t, w, func(m *verify.Marker) {
			m.MetadataReferenceDigest = "deadbeef"
		}), verify.CheckMarkerReference)
	})
	t.Run("markerSelfReferenceDigest", func(t *testing.T) {
		seeded(t, tamperMarker(t, w, func(m *verify.Marker) {
			m.LogicalKVDigest = strings.Repeat("ab", 32) // ≠ marker-excluded recomputation
		}), verify.CheckMarkerLogical)
	})
	t.Run("markerWrongToolVersion", func(t *testing.T) {
		seeded(t, tamperMarker(t, w, func(m *verify.Marker) {
			m.ToolVersion = "rogue-tool/1"
		}), verify.CheckMarkerToolIdentity)
	})
	t.Run("markerWrongBinarySHA", func(t *testing.T) {
		seeded(t, tamperMarker(t, w, func(m *verify.Marker) {
			m.ToolBinarySHA256 = strings.Repeat("cd", 32)
		}), verify.CheckMarkerToolIdentity)
	})
	t.Run("modeMismatchManifestSupplied", func(t *testing.T) {
		// Internal-mode build verified WITH a manifest: fatal.
		dir := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.compactDir, dir); err != nil {
			t.Fatal(err)
		}
		manifestPath := writeReferenceManifest(t, w, nil)
		res := runVerify(t, dir, w.compactRep, manifestPath)
		if res.Passed {
			t.Fatal("mode mismatch must fail")
		}
		assertFailed(t, res, verify.CheckMarkerReference)
	})
}

func tamperMarker(t *testing.T, w *sharedWorld, mutate func(*verify.Marker)) func(db interface {
	Put([]byte, []byte) error
	Delete([]byte) error
	Get([]byte) ([]byte, error)
}) {
	return func(db interface {
		Put([]byte, []byte) error
		Delete([]byte) error
		Get([]byte) ([]byte, error)
	}) {
		raw, err := db.Get(keys.RecoveryMarkerKey)
		if err != nil {
			t.Fatal(err)
		}
		var m verify.Marker
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		mutate(&m)
		out, err := json.Marshal(&m)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put(keys.RecoveryMarkerKey, out); err != nil {
			t.Fatal(err)
		}
	}
}

func failedIDs(res *verify.Result) []string {
	var out []string
	for _, c := range res.Checks {
		if !c.OK {
			out = append(out, c.ID)
		}
	}
	return out
}

func assertFailed(t *testing.T, res *verify.Result, id string) {
	t.Helper()
	for _, c := range res.Checks {
		if c.ID == id && !c.OK {
			return
		}
	}
	t.Fatalf("check %s did not fail; failures: %v", id, failedIDs(res))
}

func canonicalAt(t *testing.T, w *sharedWorld, n uint64) common.Hash {
	t.Helper()
	db, ro, err := dbopen.OpenSourceDatabase(w.compactDir)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	return rawdb.ReadCanonicalHash(db, n)
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	sum, err := integrity.FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

// ---- reference-mode legs (plan WS5/WS6, round 8 finding 1) ----

func writeReferenceManifest(t *testing.T, w *sharedWorld, mutate func(map[string]string)) string {
	t.Helper()
	sections := map[string]string{}
	for k, v := range w.compactRep.NormalizedSections {
		sections[k] = v
	}
	if mutate != nil {
		mutate(sections)
	}
	m := verify.MetadataReferenceManifest{
		SchemaVersion: verify.MetadataReferenceSchemaV1,
		Sections:      sections,
	}
	path := filepath.Join(t.TempDir(), "metadata-reference.json")
	if err := writeJSONSum(path, &m); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReferenceMode(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)
	chainCfg, _ := harness.ChainConfig("localnet", 0)

	t.Run("buildAndVerify", func(t *testing.T) {
		manifestPath := writeReferenceManifest(t, w, nil)
		dest := filepath.Join(t.TempDir(), "harmony_db_0")
		out := filepath.Join(t.TempDir(), "compact-ref.json")
		rep, err := compact.Run(compact.Config{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			SourceDB: w.replayed, DestinationDB: dest,
			AnchorPath: w.k.anchorPath, SourceReferencePath: w.replayJSON,
			MetadataReferenceManifestPath: manifestPath,
			TargetHeight:                  targetHeight, ToolVersion: toolVersion, OutputPath: out,
		}, w.window)
		if err != nil {
			t.Fatalf("reference-mode compact: %v", err)
		}
		if rep.Mode != report.ModeReference || rep.MetadataReferenceDigest == verify.MetadataReferenceInternalNone {
			t.Fatalf("mode not recorded: %+v", rep.Mode)
		}
		// Verify WITH the manifest passes.
		roDB, ro, err := dbopen.OpenSourceDatabase(dest)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		anc, _ := anchor.Load(w.k.anchorPath)
		sched, _ := harness.Schedule("localnet")
		res, err := verify.Run(roDB, verify.Params{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			Anchor: anc, AnchorSHA256: fileSHA(t, w.k.anchorPath),
			Compact: rep, MetadataReferenceManifestPath: manifestPath,
			Window: w.window, TargetIsEpochLast: sched.EpochLastBlock(w.window.Epoch) == w.window.Target,
			TempDir: t.TempDir(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Passed {
			t.Fatalf("reference-mode verify failed: %v", failedIDs(res))
		}
		// Mode mismatch: verified WITHOUT the manifest is fatal.
		res2, err := verify.Run(roDB, verify.Params{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			Anchor: anc, AnchorSHA256: fileSHA(t, w.k.anchorPath),
			Compact: rep, Window: w.window,
			TargetIsEpochLast: sched.EpochLastBlock(w.window.Epoch) == w.window.Target,
			TempDir:           t.TempDir(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if res2.Passed {
			t.Fatal("reference-mode build verified without manifest must fail")
		}
		assertFailed(t, res2, verify.CheckMarkerReference)
	})

	t.Run("divergedMetadataRefusesBeforeHeads", func(t *testing.T) {
		manifestPath := writeReferenceManifest(t, w, func(s map[string]string) {
			s["validator-list"] = strings.Repeat("ee", 32)
		})
		dest := filepath.Join(t.TempDir(), "harmony_db_0")
		out := filepath.Join(t.TempDir(), "compact-div.json")
		_, err := compact.Run(compact.Config{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			SourceDB: w.replayed, DestinationDB: dest,
			AnchorPath: w.k.anchorPath, SourceReferencePath: w.replayJSON,
			MetadataReferenceManifestPath: manifestPath,
			TargetHeight:                  targetHeight, ToolVersion: toolVersion, OutputPath: out,
		}, w.window)
		if err == nil || !strings.Contains(err.Error(), "convergence failed") {
			t.Fatalf("diverged manifest must refuse, got %v", err)
		}
		// Refused BEFORE writing any head key.
		wdb, err := rawdb.NewLevelDBDatabase(dest, 16, 64, "", false)
		if err != nil {
			t.Fatal(err)
		}
		defer wdb.Close()
		if ok, _ := wdb.Has(keys.HeadBlockKey); ok {
			t.Fatal("diverged convergence must refuse with NO head key written")
		}
	})
}

// ---- two-build logical-digest equality (plan WS8) ----

func TestTwoBuildDigestEquality(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)
	chainCfg, _ := harness.ChainConfig("localnet", 0)
	digests := []string{}
	for i, batch := range []int{compact.DefaultBatchBytes, 4096} {
		dest := filepath.Join(t.TempDir(), fmt.Sprintf("build-%d", i), "harmony_db_0")
		out := filepath.Join(t.TempDir(), fmt.Sprintf("compact-%d.json", i))
		rep, err := compact.Run(compact.Config{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			SourceDB: w.replayed, DestinationDB: dest,
			AnchorPath: w.k.anchorPath, SourceReferencePath: w.replayJSON,
			TargetHeight: targetHeight, BatchBytes: batch, ToolVersion: toolVersion, OutputPath: out,
		}, w.window)
		if err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
		digests = append(digests, rep.LogicalKVDigest)
	}
	if digests[0] != digests[1] {
		t.Fatalf("two independent builds must produce identical logical KV digests: %s vs %s", digests[0], digests[1])
	}
	if digests[0] != w.compactRep.LogicalKVDigest {
		t.Fatalf("rebuilds must match the original build's digest")
	}
}

// ---- export faults (plan WS3 acceptance) ----

func TestExportFaults(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)
	chainCfg, _ := harness.ChainConfig("localnet", 0)

	t.Run("donorGapMidRange", func(t *testing.T) {
		gapDonor := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.k.donorDir, gapDonor); err != nil {
			t.Fatal(err)
		}
		mid := baselineHeight + 2
		wdb, err := rawdb.NewLevelDBDatabase(gapDonor, 16, 64, "", false)
		if err != nil {
			t.Fatal(err)
		}
		ch := rawdb.ReadCanonicalHash(wdb, uint64(mid))
		if err := wdb.Delete(keys.BodyKey(uint64(mid), ch)); err != nil {
			t.Fatal(err)
		}
		wdb.Close()
		db, ro, err := dbopen.OpenSourceDatabase(gapDonor)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		pre, err := bundle.Preflight(db, bundle.ExportConfig{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
			BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
			Donor: "gap", ToolVersion: toolVersion,
		})
		if err != nil {
			t.Fatal(err)
		}
		if pre.Passed || pre.GapCount == 0 {
			t.Fatalf("gapped donor must fail preflight block-accurately: %+v", pre)
		}
		found := false
		for _, g := range pre.Gaps {
			if strings.Contains(g, fmt.Sprintf("body missing at %d", mid)) {
				found = true
			}
		}
		if !found {
			t.Fatalf("gap must name the block: %v", pre.Gaps)
		}
		if _, err := bundle.Export(db, bundle.ExportConfig{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
			BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
			OutputDir: filepath.Join(t.TempDir(), "out"), Donor: "gap", ToolVersion: toolVersion,
		}); err == nil {
			t.Fatal("a gapped donor is refused by export")
		}
	})

	t.Run("zeroedCertificateShape", func(t *testing.T) {
		// A child header whose last-commit fields are empty carries NO
		// certificate for its parent; the mechanical preflight must refuse
		// (round 13 finding 13), not defer the failure to export.
		badDonor := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.k.donorDir, badDonor); err != nil {
			t.Fatal(err)
		}
		n := uint64(baselineHeight + 2)
		wdb, err := rawdb.NewLevelDBDatabase(badDonor, 16, 64, "", false)
		if err != nil {
			t.Fatal(err)
		}
		ch := rawdb.ReadCanonicalHash(wdb, n)
		hdr := rawdb.ReadHeader(wdb, ch, n)
		hdr.SetLastCommitSignature([96]byte{})
		hdr.SetLastCommitBitmap(nil)
		raw, err := rlp.EncodeToBytes(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if err := wdb.Put(keys.HeaderKey(n, ch), raw); err != nil {
			t.Fatal(err)
		}
		wdb.Close()
		db, ro, err := dbopen.OpenSourceDatabase(badDonor)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		pre, err := bundle.Preflight(db, bundle.ExportConfig{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
			BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
			Donor: "zeroed-cert", ToolVersion: toolVersion,
		})
		if err != nil {
			t.Fatal(err)
		}
		if pre.Passed || pre.CertsPresent {
			t.Fatalf("zeroed certificate must fail preflight: %+v", pre)
		}
		found := false
		for _, g := range pre.Gaps {
			if strings.Contains(g, fmt.Sprintf("zero last-commit signature in header %d", n)) {
				found = true
			}
		}
		if !found {
			t.Fatalf("gap must name the header: %v", pre.Gaps)
		}
	})

	t.Run("wrongFromHeight", func(t *testing.T) {
		db, ro, err := dbopen.OpenSourceDatabase(w.k.donorDir)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		_, err = bundle.Export(db, bundle.ExportConfig{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			FromHeight: baselineHeight + 2, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
			BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
			OutputDir: filepath.Join(t.TempDir(), "out"), Donor: "x", ToolVersion: toolVersion,
		})
		if err == nil || !strings.Contains(err.Error(), "disagrees with baseline") {
			t.Fatalf("wrong from-height must refuse, got %v", err)
		}
	})

	t.Run("deterministicChunks", func(t *testing.T) {
		db, ro, err := dbopen.OpenSourceDatabase(w.k.donorDir)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		shas := [][]string{}
		for i := 0; i < 2; i++ {
			out := filepath.Join(t.TempDir(), fmt.Sprintf("exp-%d", i))
			m, err := bundle.Export(db, bundle.ExportConfig{
				Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
				FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
				BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
				OutputDir: out, Donor: "x", ToolVersion: toolVersion,
			})
			if err != nil {
				t.Fatal(err)
			}
			var s []string
			for _, c := range m.Chunks {
				s = append(s, c.SHA256)
			}
			shas = append(shas, s)
		}
		if fmt.Sprint(shas[0]) != fmt.Sprint(shas[1]) {
			t.Fatalf("two exports of the same donor must be byte-identical: %v vs %v", shas[0], shas[1])
		}
	})

	t.Run("compareBundles", func(t *testing.T) {
		db, ro, err := dbopen.OpenSourceDatabase(w.k.donorDir)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		left := filepath.Join(t.TempDir(), "left")
		if _, err := bundle.Export(db, bundle.ExportConfig{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
			BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(headHashOf(t, w)),
			OutputDir: left, Donor: "x", ToolVersion: toolVersion,
		}); err != nil {
			t.Fatal(err)
		}
		res, err := bundle.Compare(left, w.bundleDir, "localnet", 0, toolVersion)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Identical {
			t.Fatalf("identical chains must compare identical: %s", res.FirstDifference)
		}
	})
}

// ---- archival preflight deleted-record fixtures (round 13 finding 6) ----

// TestPreflightDeletedRecords deletes/corrupts individual records inside the
// retention window of a pristine baseline and asserts the full-archival
// replay preflight refuses block-accurately (plan WS2; round 13 finding 6).
func TestPreflightDeletedRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)

	mutated := func(t *testing.T, plant func(db ethdb.Database)) *report.InspectReport {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.k.baseB, dir); err != nil {
			t.Fatal(err)
		}
		wdb, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
		if err != nil {
			t.Fatal(err)
		}
		plant(wdb)
		wdb.Close()
		rep, _, err := inspect.Run(inspect.Params{
			Network: "localnet", ShardID: 0, DBPath: dir,
			TargetHeight: targetHeight, AnchorPath: w.k.anchorPath,
			Output: filepath.Join(t.TempDir(), "inspect.json"), ToolVersion: toolVersion,
		})
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}

	requireRefusal := func(t *testing.T, rep *report.InspectReport, want string) {
		t.Helper()
		if !rep.ReplayPreflight.Ran {
			t.Fatal("preflight did not run")
		}
		if rep.ReplayPreflight.FullArchival {
			t.Fatalf("mutilated baseline must fail full-archival preflight (want %q)", want)
		}
		for _, f := range rep.ReplayPreflight.Failures {
			if strings.Contains(f, want) {
				return
			}
		}
		t.Fatalf("no preflight failure contains %q: %v", want, rep.ReplayPreflight.Failures)
	}

	mid := uint64(baselineHeight - 2)

	t.Run("deletedHeader", func(t *testing.T) {
		rep := mutated(t, func(db ethdb.Database) {
			ch := rawdb.ReadCanonicalHash(db, mid)
			if err := db.Delete(keys.HeaderKey(mid, ch)); err != nil {
				t.Fatal(err)
			}
		})
		requireRefusal(t, rep, fmt.Sprintf("header missing at %d", mid))
	})
	t.Run("deletedCanonicalHash", func(t *testing.T) {
		rep := mutated(t, func(db ethdb.Database) {
			if err := db.Delete(keys.CanonicalHashKey(mid)); err != nil {
				t.Fatal(err)
			}
		})
		requireRefusal(t, rep, fmt.Sprintf("canonical hash missing at %d", mid))
	})
	t.Run("deletedBody", func(t *testing.T) {
		rep := mutated(t, func(db ethdb.Database) {
			ch := rawdb.ReadCanonicalHash(db, mid)
			if err := db.Delete(keys.BodyKey(mid, ch)); err != nil {
				t.Fatal(err)
			}
		})
		requireRefusal(t, rep, fmt.Sprintf("body missing at %d", mid))
	})
	t.Run("deletedShardState", func(t *testing.T) {
		var epoch uint64
		rep := mutated(t, func(db ethdb.Database) {
			ch := rawdb.ReadCanonicalHash(db, baselineHeight)
			epoch = rawdb.ReadHeader(db, ch, baselineHeight).Epoch().Uint64()
			if err := db.Delete(keys.ShardStateKey(new(big.Int).SetUint64(epoch))); err != nil {
				t.Fatal(err)
			}
		})
		requireRefusal(t, rep, fmt.Sprintf("shard state for epoch %d unreadable", epoch))
	})
	t.Run("undecodableEpochVRF", func(t *testing.T) {
		var epoch uint64
		rep := mutated(t, func(db ethdb.Database) {
			ch := rawdb.ReadCanonicalHash(db, baselineHeight)
			epoch = rawdb.ReadHeader(db, ch, baselineHeight).Epoch().Uint64()
			if err := db.Put(keys.EpochVrfKey(new(big.Int).SetUint64(epoch)), []byte{0xff, 0x00, 0x13}); err != nil {
				t.Fatal(err)
			}
		})
		requireRefusal(t, rep, fmt.Sprintf("undecodable epoch-%d VRF record", epoch))
	})
}

// ---- package-db crash-window reconciliation (plan WS7 acceptance) ----

func TestPackageCrashWindows(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)

	buildRelease := func(t *testing.T) (release.Config, string, *report.PackageReport) {
		t.Helper()
		// A verification report for the good compact artifact.
		verifOut := filepath.Join(t.TempDir(), "verification.json")
		meta, err := report.NewMeta(report.VerificationSchemaV1, "verify-db", "localnet", 0, toolVersion,
			[]integrity.InputRef{{Name: "anchor-manifest", Path: w.k.anchorPath, SHA256: fileSHA(t, w.k.anchorPath)}})
		if err != nil {
			t.Fatal(err)
		}
		verifRep := &report.VerificationReport{
			Meta: meta, DBPath: w.compactDir, Mode: w.compactRep.Mode, Passed: true,
			DigestSet:               w.compactRep.DigestSet,
			LogicalKVDigest:         w.compactRep.LogicalKVDigest,
			NormalizedOutputDigest:  w.compactRep.NormalizedOutputDigest,
			MetadataReferenceDigest: w.compactRep.MetadataReferenceDigest,
			JournalState:            report.StateCompleteVerified,
		}
		if _, err := report.WriteJSON(verifOut, verifRep); err != nil {
			t.Fatal(err)
		}
		cfg := release.Config{
			Network: "localnet", ShardID: 0,
			DBPath: w.compactDir, AnchorPath: w.k.anchorPath, TargetHeight: targetHeight,
			VerificationReportPath: verifOut,
			ReleaseRoot:            t.TempDir(),
			ToolVersion:            toolVersion,
		}
		rep, finalDir, err := release.Run(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return cfg, finalDir, rep
	}

	truncateJournal := func(t *testing.T, finalDir string, keep int) {
		t.Helper()
		journalPath := filepath.Join(filepath.Dir(finalDir), filepath.Base(finalDir)+".journal")
		raw, err := os.ReadFile(journalPath)
		if err != nil {
			t.Fatal(err)
		}
		lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
		if len(lines) < keep {
			t.Fatalf("journal too short: %d", len(lines))
		}
		out := bytes.Join(lines[:keep], []byte("\n"))
		out = append(out, '\n')
		if err := os.WriteFile(journalPath, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("killBetweenRenameAndREADY", func(t *testing.T) {
		cfg, finalDir, rep := buildRelease(t)
		// Simulate the crash window: READY missing, journal ends at PROMOTED.
		os.Remove(filepath.Join(finalDir, "READY"))
		truncateJournal(t, finalDir, 3) // IN_PROGRESS, PROMOTING, PROMOTED
		rep2, dir2, err := release.Run(cfg)
		if err != nil {
			t.Fatalf("rerun must reconcile: %v", err)
		}
		if dir2 != finalDir || rep2.ReleaseID != rep.ReleaseID {
			t.Fatalf("reconciliation must complete the SAME release")
		}
		ready, err := os.ReadFile(filepath.Join(finalDir, "READY"))
		if err != nil || strings.TrimSpace(string(ready)) != rep.ReleaseID {
			t.Fatalf("rerun must seal with READY: %v %q", err, ready)
		}
	})

	t.Run("killBetweenREADYAndTerminal", func(t *testing.T) {
		cfg, finalDir, rep := buildRelease(t)
		truncateJournal(t, finalDir, 4) // ..., SEALED (terminal record dropped)
		rep2, _, err := release.Run(cfg)
		if err != nil {
			t.Fatalf("rerun must fully re-verify and complete: %v", err)
		}
		if rep2.ReleaseID != rep.ReleaseID {
			t.Fatalf("release ID changed across reconciliation")
		}
	})

	t.Run("tamperedSealedTreeQuarantined", func(t *testing.T) {
		cfg, finalDir, _ := buildRelease(t)
		os.Remove(filepath.Join(finalDir, "READY"))
		truncateJournal(t, finalDir, 3)
		// Tamper a staged file inside the promoted-but-unsealed tree.
		if err := os.WriteFile(filepath.Join(finalDir, "INSTALL.md"), []byte("tampered"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err := release.Run(cfg)
		if err == nil || !strings.Contains(err.Error(), "quarantined") {
			t.Fatalf("failed re-verify must quarantine, got %v", err)
		}
	})

	t.Run("unjournaledDirQuarantined", func(t *testing.T) {
		cfg, finalDir, _ := buildRelease(t)
		journalPath := filepath.Join(filepath.Dir(finalDir), filepath.Base(finalDir)+".journal")
		os.Remove(journalPath)
		_, _, err := release.Run(cfg)
		if err == nil || !strings.Contains(err.Error(), "quarantined") {
			t.Fatalf("unjournaled release dir must be quarantined, got %v", err)
		}
	})

	t.Run("refusesUnreleasableBuild", func(t *testing.T) {
		// A destination whose journal says COMPLETE_UNRELEASABLE.
		dir := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := fixture.CopyDir(w.compactDir, dir); err != nil {
			t.Fatal(err)
		}
		os.Remove(report.JournalPath(dir))
		j, err := report.CreateJournal(report.JournalPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		j.Complete(report.StateCompleteUnreleasable, "size gate")
		j.Close()
		cfg, _, _ := buildRelease(t)
		cfg.DBPath = dir
		cfg.ReleaseRoot = t.TempDir()
		_, _, err = release.Run(cfg)
		if err == nil || !strings.Contains(err.Error(), "COMPLETE_UNRELEASABLE") {
			t.Fatalf("unreleasable build must have its own refusal, got %v", err)
		}
	})
}
