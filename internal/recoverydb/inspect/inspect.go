// Package inspect implements inspect-db and the two-copy agreement (plan
// WS2): baseline tuple pinning, full state and off-chain digest passes with
// preimage-coverage enforcement, the full-archival replay preflight, and the
// baseline gate.
package inspect

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/rawdb"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

// Params configures an inspect run.
type Params struct {
	Network string
	ShardID uint32

	DBPath           string
	FullState        bool
	FullOffchain     bool
	RequirePreimages bool
	TargetHeight     uint64
	AnchorPath       string
	Output           string
	ToolVersion      string
}

// Run performs the inspection, writes the report (with .sha256 sidecar), and
// returns it together with its SHA-256. Check failures are returned in the
// report; the error return is for environmental problems.
func Run(p Params) (*report.InspectReport, string, error) {
	db, ro, err := dbopen.OpenSourceDatabase(p.DBPath)
	if err != nil {
		return nil, "", err
	}
	defer ro.Close()

	var anc *anchor.Manifest
	inputs := []integrity.InputRef{}
	if p.AnchorPath != "" {
		if _, err := integrity.VerifyChecksumFile(p.AnchorPath); err != nil {
			return nil, "", err
		}
		ref, err := integrity.NewInputRef("anchor-manifest", p.AnchorPath)
		if err != nil {
			return nil, "", err
		}
		inputs = append(inputs, ref)
		if anc, err = anchor.Load(p.AnchorPath); err != nil {
			return nil, "", err
		}
		if p.TargetHeight != 0 {
			if err := anc.RequireTargetHeight(p.TargetHeight); err != nil {
				return nil, "", err
			}
		}
	}

	meta, err := report.NewMeta(report.InspectSchemaV1, "inspect-db", p.Network, p.ShardID, p.ToolVersion, inputs)
	if err != nil {
		return nil, "", err
	}
	rep := &report.InspectReport{Meta: meta, LayoutOK: true, TargetHeight: p.TargetHeight}
	rep.MarkerPresence = map[string]bool{}

	fail := func(id, format string, args ...interface{}) {
		rep.Checks = append(rep.Checks, report.Check{ID: id, OK: false, Detail: fmt.Sprintf(format, args...)})
	}
	pass := func(id string) { rep.Checks = append(rep.Checks, report.Check{ID: id, OK: true}) }

	// Source identity.
	files, bytesUsed, err := dirStats(p.DBPath)
	if err != nil {
		return nil, "", err
	}
	var st syscall.Stat_t
	if err := syscall.Stat(p.DBPath, &st); err != nil {
		return nil, "", fmt.Errorf("inspect: stat %s: %w", p.DBPath, err)
	}
	rep.Source = report.SourceIdentity{
		AbsolutePath: p.DBPath,
		DeviceID:     uint64(st.Dev),
		FileCount:    files,
		TotalBytes:   bytesUsed,
	}

	// Heads: raw values resolved to (height, hash, epoch, viewID, stateRoot).
	names := []struct {
		name string
		key  []byte
	}{{"LastBlock", keys.HeadBlockKey}, {"LastHeader", keys.HeadHeaderKey}, {"LastFast", keys.HeadFastBlockKey}}
	rep.HeadsAgree = true
	for _, n := range names {
		ht, err := resolveHead(db, n.key, n.name)
		if err != nil {
			return nil, "", err
		}
		rep.Heads = append(rep.Heads, ht)
		if ht.Hash != rep.Heads[0].Hash {
			rep.HeadsAgree = false
		}
	}
	head := rep.Heads[0]
	headHash := common.HexToHash(head.Hash)
	if ch := rawdb.ReadCanonicalHash(db, head.Height); ch == headHash {
		rep.CanonicalHeadMatch = true
		pass("inspect.canonical-head")
	} else {
		fail("inspect.canonical-head", "canonical(%d) = %s != head %s", head.Height, ch.Hex(), head.Hash)
	}

	// Genesis + config presence.
	genHash := rawdb.ReadCanonicalHash(db, 0)
	rep.GenesisHash = genHash.Hex()
	if ok, err := db.Has(keys.ConfigKey(genHash)); err == nil {
		rep.ChainConfigPresent = ok
	}
	if v := rawdb.ReadDatabaseVersion(db); v != nil {
		rep.DatabaseVersion = v
	}

	// Marker presence (plan §2.1 fact 10 + WS2 list).
	for _, m := range []struct {
		name string
		key  []byte
	}{
		{"LastFinalized", keys.HeadFinalizedKey},
		{"LastPivot", keys.LastPivotKey},
		{"SnapdbInfo", keys.SnapdbInfoKey},
		{"SnapshotJournal", keys.SnapshotJournalKey},
		{"SnapshotRoot", keys.SnapshotRootKey},
		{"SnapshotGenerator", keys.SnapshotGeneratorKey},
		{"SnapshotRecovery", keys.SnapshotRecoveryKey},
		{"SnapshotSyncStatus", keys.SnapshotSyncStatusKey},
		{"SkeletonSyncStatus", keys.SkeletonSyncStatusKey},
		{"unclean-shutdown", keys.UncleanShutdownKey},
		{"InvalidBlock", keys.BadBlockKey},
		{"LastCommits", keys.LastCommitsKey},
		{"TrieSync", keys.TrieSyncKey},
	} {
		ok, err := db.Has(m.key)
		if err != nil {
			return nil, "", fmt.Errorf("inspect: probe %s: %w", m.name, err)
		}
		rep.MarkerPresence[m.name] = ok
	}

	// Digest passes.
	rep.FullStateCheck = p.FullState
	rep.FullOffchainCheck = p.FullOffchain
	rep.Preimages.Checked = p.FullState
	rep.Preimages.Required = p.RequirePreimages
	sched, err := harness.Schedule(p.Network)
	if err != nil {
		return nil, "", err
	}
	win, err := anchor.ComputeWindow(sched, head.Height, 0)
	if err != nil {
		return nil, "", err
	}
	window := report.DigestWindow{RetainFrom: win.RetainFrom, Target: head.Height}

	var walk *verify.StateWalkResult
	if p.FullState {
		walk, err = verify.WalkState(db, common.HexToHash(head.StateRoot), verify.StateWalkOptions{
			CheckPreimages:   true,
			RequirePreimages: p.RequirePreimages,
		})
		if err != nil {
			fail("inspect.full-state", "%v", err)
		} else {
			rep.Preimages.MissingAccountPreimages = walk.MissingAccountPreimages
			rep.Preimages.MissingStoragePreimages = walk.MissingStoragePreimages
			pass("inspect.full-state")
		}
	}
	if p.FullState && p.FullOffchain && walk != nil {
		off, err := verify.ComputeOffchainDigests(db, window)
		if err != nil {
			fail("inspect.full-offchain", "%v", err)
		} else {
			rep.DigestSet = verify.BuildDigestSet(p.Network, p.ShardID, head.Height, headHash,
				common.HexToHash(head.StateRoot), window, walk, off)
			pass("inspect.full-offchain")
		}
	} else if p.FullOffchain && !p.FullState {
		fail("inspect.full-offchain", "--full-offchain-check without --full-state-check produces no DigestSet (both halves required)")
	}

	// Full-archival replay preflight + baseline gate (need a target).
	if p.TargetHeight != 0 {
		runPreflight(p, rep, db, head, sched, win.RetainFrom, fail, pass)
		runBaselineGate(p, rep, db, ro, anc, head, headHash, fail, pass)
	}

	sum, err := report.WriteJSON(p.Output, rep)
	if err != nil {
		return nil, "", err
	}
	return rep, sum, nil
}

func runPreflight(p Params, rep *report.InspectReport, db ethdb.Database, head report.HeadTuple,
	sched shardingconfig.Schedule, retainFrom uint64,
	fail func(string, string, ...interface{}), pass func(string)) {
	rep.ReplayPreflight.Ran = true
	rep.ReplayPreflight.FullArchival = true
	refuse := func(format string, args ...interface{}) {
		rep.ReplayPreflight.FullArchival = false
		rep.ReplayPreflight.Failures = append(rep.ReplayPreflight.Failures, fmt.Sprintf(format, args...))
	}
	if !rep.HeadsAgree {
		refuse("heads disagree")
	}
	if rep.MarkerPresence["SnapdbInfo"] {
		refuse("SnapdbInfo resume marker present (stock-dumpdb-shaped source; full-archival input only — operator answer 7)")
	}
	// Checked probe (round 14 finding 2): a Has error is a refusal in its
	// own right, never collapsed into absence.
	lastFast, lfErr := db.Has(keys.HeadFastBlockKey)
	if lfErr != nil {
		refuse("LastFast probe failed: %v", lfErr)
	} else if !lastFast {
		refuse("LastFast missing (stock-dumpdb-shaped source)")
	}
	if rep.MarkerPresence["unclean-shutdown"] || rep.MarkerPresence["InvalidBlock"] {
		refuse("unclean-shutdown/InvalidBlock marker present")
	}
	// Range checks over the retention window feeding the replay interval's
	// committee lookups (round 13 finding 6; plan WS2 full-archival replay
	// preflight): continuous canonical/header/body records, shard states for
	// every epoch in the window (plus the next epoch when the head closes
	// its own), and decodable epoch-VRF records.
	prev := common.Hash{}
	for n := retainFrom; n <= head.Height; n++ {
		ch := rawdb.ReadCanonicalHash(db, n)
		if ch == (common.Hash{}) {
			refuse("canonical hash missing at %d", n)
			break
		}
		hdr := rawdb.ReadHeader(db, ch, n)
		if hdr == nil {
			refuse("header missing at %d", n)
			break
		}
		if n > retainFrom && hdr.ParentHash() != prev {
			refuse("header chain break at %d: parent %s != canonical(%d) %s",
				n, hdr.ParentHash().Hex(), n-1, prev.Hex())
			break
		}
		if body := rawdb.ReadBody(db, ch, n); body == nil {
			refuse("body missing at %d", n)
			break
		}
		prev = ch
	}
	firstEpoch := sched.CalcEpochNumber(retainFrom).Uint64()
	lastEpoch := head.Epoch
	if sched.IsLastBlock(head.Height) {
		// The last block of an epoch commits the NEXT epoch's shard state;
		// replay of head+1 looks its committee up immediately.
		lastEpoch++
	}
	for e := firstEpoch; e <= lastEpoch; e++ {
		epoch := new(big.Int).SetUint64(e)
		if _, err := rawdb.ReadShardState(db, epoch); err != nil {
			refuse("shard state for epoch %d unreadable: %v", e, err)
		}
		// Epoch-VRF records are legitimately ABSENT (the production write
		// path is disabled, core/offchain.go:70-96 — round 13 finding 7's
		// evidence); any record that IS present must decode, and a probe
		// error is a refusal (round 14 finding 2).
		vrfPresent, vrfErr := db.Has(keys.EpochVrfKey(epoch))
		if vrfErr != nil {
			refuse("epoch-%d VRF probe failed: %v", e, vrfErr)
		} else if vrfPresent {
			raw, err := db.Get(keys.EpochVrfKey(epoch))
			if err != nil {
				refuse("epoch-%d VRF record unreadable: %v", e, err)
			} else {
				nums := []uint64{}
				if err := rlp.DecodeBytes(raw, &nums); err != nil {
					refuse("undecodable epoch-%d VRF record: %v", e, err)
				}
			}
		}
	}
	if p.FullState {
		// The full walk already ran; its failure is recorded separately.
	} else if _, err := verify.WalkStateProbe(db, common.HexToHash(head.StateRoot)); err != nil {
		refuse("head state does not open: %v", err)
	}
	if rep.ReplayPreflight.FullArchival {
		pass("inspect.replay-preflight")
	} else {
		fail("inspect.replay-preflight", "%v", rep.ReplayPreflight.Failures)
	}
}

func runBaselineGate(p Params, rep *report.InspectReport, db ethdb.Database, ro *dbopen.ReadOnlyDB,
	anc *anchor.Manifest, head report.HeadTuple, headHash common.Hash,
	fail func(string, string, ...interface{}), pass func(string)) {
	rep.BaselineGate.Ran = true
	rep.BaselineGate.Passed = true
	gateFail := func(format string, args ...interface{}) {
		rep.BaselineGate.Passed = false
		rep.BaselineGate.Failures = append(rep.BaselineGate.Failures, fmt.Sprintf(format, args...))
	}
	if head.Height >= p.TargetHeight {
		gateFail("head %d not below target %d", head.Height, p.TargetHeight)
	}
	if anc != nil {
		bad := []struct {
			name string
			hash common.Hash
		}{
			{"abandoned-child", anc.AbandonedChildHash},
			{"rejected-shard1", anc.RejectedShard1Hash},
		}
		for _, kb := range anc.KnownBad {
			bad = append(bad, struct {
				name string
				hash common.Hash
			}{fmt.Sprintf("known-bad-%d", kb.Height), kb.Hash})
		}
		for _, b := range bad {
			if b.hash == (common.Hash{}) {
				continue
			}
			if num := rawdb.ReadHeaderNumber(db, b.hash); num != nil {
				gateFail("%s hash %s present (height %d)", b.name, b.hash.Hex(), *num)
			}
		}
	}
	// Open/close causes no repair/rewind: raw head keys compared before and
	// after a harness open over the idempotent-write probe (the stock chain
	// re-writes LastHeader with the SAME value at every open; the probe
	// swallows exactly those no-op writes and refuses any value-CHANGING
	// write — i.e. an actual repair or rewind fails the open, and the
	// directory stays byte-untouched either way).
	before, err := rawHeads(db)
	if err != nil {
		gateFail("read raw heads: %v", err)
	}
	probe := dbopen.NewProbe(ro)
	probeDB := rawdb.NewDatabase(probe)
	bc, err := harness.OpenChain(probeDB, p.Network, p.ShardID, harness.ModeReadOnly)
	if err != nil {
		gateFail("read-only harness open failed (repair/rewind attempted?): %v", err)
	} else if bc.CurrentBlock().Hash() != headHash {
		gateFail("harness resolves head %s, raw head is %s", bc.CurrentBlock().Hash().Hex(), head.Hash)
	}
	after, err := rawHeads(db)
	if err != nil {
		gateFail("re-read raw heads: %v", err)
	}
	if before != after {
		gateFail("raw head keys changed across a read-only open: %s -> %s", before, after)
	}
	if rep.BaselineGate.Passed {
		pass("inspect.baseline-gate")
	} else {
		fail("inspect.baseline-gate", "%v", rep.BaselineGate.Failures)
	}
}

// Agreement compares two inspect reports (this run's, by path) and writes
// the baseline-agreement verdict naming both by SHA-256 (plan WS2).
func Agreement(network string, shardID uint32, toolVersion, myReportPath, otherPath, output string) (*report.AgreementVerdict, error) {
	if _, err := integrity.VerifyChecksumFile(myReportPath); err != nil {
		return nil, err
	}
	mySHA, err := integrity.FileSHA256(myReportPath)
	if err != nil {
		return nil, err
	}
	var mine report.InspectReport
	if err := report.ReadJSONStrict(myReportPath, &mine); err != nil {
		return nil, err
	}
	if _, err := integrity.VerifyChecksumFile(otherPath); err != nil {
		return nil, err
	}
	otherSHA, err := integrity.FileSHA256(otherPath)
	if err != nil {
		return nil, err
	}
	var other report.InspectReport
	if err := report.ReadJSONStrict(otherPath, &other); err != nil {
		return nil, err
	}

	var diffs []string
	if mine.DigestSet == nil || other.DigestSet == nil {
		diffs = append(diffs, "agreement requires complete DigestSets on both copies (both --full-state-check and --full-offchain-check); refused otherwise")
	} else {
		if len(mine.Heads) != len(other.Heads) {
			diffs = append(diffs, "head tuple counts differ")
		} else {
			for i := range mine.Heads {
				if mine.Heads[i] != other.Heads[i] {
					diffs = append(diffs, fmt.Sprintf("head %s: %+v vs %+v", mine.Heads[i].Key, mine.Heads[i], other.Heads[i]))
				}
			}
		}
		diffs = append(diffs, mine.DigestSet.Diff(other.DigestSet)...)
	}

	meta, err := report.NewMeta(report.AgreementSchemaV1, "inspect-db --compare-with", network, shardID, toolVersion, nil)
	if err != nil {
		return nil, err
	}
	verdict := &report.AgreementVerdict{
		Meta:        meta,
		LeftReport:  mySHA,
		RightReport: otherSHA,
		Agreed:      len(diffs) == 0,
		Differences: diffs,
	}
	if _, err := report.WriteJSON(output, verdict); err != nil {
		return nil, err
	}
	return verdict, nil
}

func resolveHead(db ethdb.Database, key []byte, name string) (report.HeadTuple, error) {
	raw, err := db.Get(key)
	if err != nil {
		return report.HeadTuple{}, fmt.Errorf("inspect: read head %s: %w", name, err)
	}
	hash := common.BytesToHash(raw)
	numPtr := rawdb.ReadHeaderNumber(db, hash)
	if numPtr == nil {
		return report.HeadTuple{}, fmt.Errorf("inspect: head %s (%s) has no header-number entry", name, hash.Hex())
	}
	hdr := rawdb.ReadHeader(db, hash, *numPtr)
	if hdr == nil {
		return report.HeadTuple{}, fmt.Errorf("inspect: head %s header unreadable", name)
	}
	return report.HeadTuple{
		Key: name, Hash: hash.Hex(), Height: *numPtr,
		Epoch: hdr.Epoch().Uint64(), ViewID: hdr.ViewID().Uint64(), StateRoot: hdr.Root().Hex(),
	}, nil
}

func rawHeads(db ethdb.Database) (string, error) {
	out := ""
	for _, k := range [][]byte{keys.HeadBlockKey, keys.HeadHeaderKey, keys.HeadFastBlockKey} {
		v, err := db.Get(k)
		if err != nil {
			return "", fmt.Errorf("read raw head %s: %w", k, err)
		}
		out += common.BytesToHash(v).Hex() + ";"
	}
	return out, nil
}

func dirStats(root string) (uint64, uint64, error) {
	var files, bytesUsed uint64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files++
			bytesUsed += uint64(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("inspect: walk %s: %w", root, err)
	}
	return files, bytesUsed, nil
}

// Failed reports whether any check in the report failed.
func Failed(rep *report.InspectReport) bool {
	for _, c := range rep.Checks {
		if !c.OK {
			return true
		}
	}
	return false
}
