package audit

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/hmr"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/scan"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
	"github.com/harmony-one/harmony/internal/recovery/report"
	"github.com/harmony-one/harmony/internal/recovery/strictdb"
)

// PassSection summarizes one pass for the report. The full per-block
// outcome list (every height: executed / root-matched / validity failures)
// is digest-bound via OutcomesSHA256 so the report carries complete
// per-block evidence even though only failed outcomes are itemized.
type PassSection struct {
	Label            string               `json:"label"`
	Authoritative    bool                 `json:"authoritative"`
	Seed             *SeedSpec            `json:"seed"`
	ExecutedBlocks   int                  `json:"executed_blocks"`
	RootsMatched     int                  `json:"roots_matched"`
	ValidityFailures int                  `json:"validity_failures"`
	OutcomeCount     int                  `json:"outcome_count"`
	OutcomesSHA256   string               `json:"outcomes_sha256"`
	// LegacyBitmapsRestored counts incoming-receipt proofs whose Copy-bug
	// corrupted CommitBitmap was verifiably restored from stored crosslinks
	// (legacybitmap.go); restoration evidence is itemized in Findings.
	LegacyBitmapsRestored int             `json:"legacy_bitmaps_restored"`
	FailedOutcomes   []blockOutcome       `json:"failed_outcomes,omitempty"`
	Findings         scan.FindingsSection `json:"findings"`
	Fatal            bool                 `json:"fatal,omitempty"`
	FatalHeight      uint64               `json:"fatal_height,omitempty"`
	FatalReason      string               `json:"fatal_reason,omitempty"`
}

// Anomaly is one reconciliation gap (exit 24 when any exist).
type Anomaly struct {
	Kind   string `json:"kind"`
	Key    string `json:"key,omitempty"`
	Detail string `json:"detail"`
}

// WriteClassCounts is the fixed-shape post-barrier write census.
type WriteClassCounts struct {
	PlanReconciled   int `json:"plan_reconciled"`
	MetadataNoOp     int `json:"metadata_noop_rewrites"`
	ChainRecords     int `json:"chain_records"`
	StateMaterial    int `json:"state_material"`
	CrossLinkSubset  int `json:"crosslink_subset"`
	SpentSubset      int `json:"spent_subset"`
	Pointers         int `json:"pointers"`
	Stats            int `json:"validator_stats"`
	Heads            int `json:"head_pointers"`
	PendingQueues    int `json:"pending_queues"`
	Anomalous        int `json:"anomalous"`
}

// ReconciliationSection is §4.6 output 4.
type ReconciliationSection struct {
	PlanKeys        int              `json:"plan_keys"`
	Reproduced      int              `json:"reproduced"`
	ByteEqual       int              `json:"byte_equal"`
	Excluded        int              `json:"excluded"`
	ExcludedReasons []string         `json:"excluded_reasons,omitempty"`
	Writes          WriteClassCounts `json:"writes"`
	Anomalies       []Anomaly        `json:"anomalies,omitempty"`
	AnomaliesOmitted int             `json:"anomalies_omitted"`
	AnomalyCount    int              `json:"anomaly_count"`
}

// DelegateClass classifies one observed delegation op (§4.6 output 5).
type DelegateClass struct {
	Source            string `json:"source"` // native | precompile
	Block             uint64 `json:"block"`
	Delegator         string `json:"delegator"`
	Validator         string `json:"validator"`
	Attempted         bool   `json:"attempted"`
	StakeMsgsVisible  bool   `json:"stake_msgs_visible"`
	MetadataProducing bool   `json:"metadata_producing"`
	FrameFailed       bool   `json:"frame_failed,omitempty"`
	EnclosingReverted bool   `json:"enclosing_reverted,omitempty"`
}

// StakingSection is §4.6 outputs 2 and 5.
type StakingSection struct {
	NativeByDirective map[string]int  `json:"native_by_directive"`
	PrecompileByKind  map[string]int  `json:"precompile_by_kind"`
	Native            []NativeOp      `json:"native_ops,omitempty"`
	NativeOmitted     int             `json:"native_omitted"`
	Precompile        []FCOp          `json:"precompile_ops,omitempty"`
	PrecompileOmitted int             `json:"precompile_omitted"`
	Delegations       []DelegateClass `json:"delegations,omitempty"`
	DelegationsOmitted int            `json:"delegations_omitted"`
	CreatedValidators []string        `json:"created_validators"`
	RemovedValidators []string        `json:"removed_validators"`
}

// EpochTransitionSection is §4.6 output 3.
type EpochTransitionSection struct {
	Observed          bool   `json:"observed"`
	NextEpoch         uint64 `json:"next_epoch,omitempty"`
	ShardStateEqual   bool   `json:"shard_state_byte_equal"`
	SnapshotBatch     int    `json:"snapshot_batch_records"`
	SnapshotBatchEqual int   `json:"snapshot_batch_byte_equal"`
}

// ShardSubset is §4.6 output 6 (B4 consumes these).
type ShardSubset struct {
	ShardID        uint32   `json:"shard_id"`
	CrossLinkNums  []uint64 `json:"crosslink_block_nums"`
	CrossLinkKeys  []string `json:"crosslink_keys"`
	SpentNums      []uint64 `json:"spent_block_nums"`
	SpentKeys      []string `json:"spent_keys"`
	PointerWritten bool     `json:"pointer_written"`
}

// PointerResult is the per-shard solver outcome.
type PointerResult struct {
	ShardID              uint32   `json:"shard_id"`
	StoredBlockNum       uint64   `json:"stored_block_num"`
	StoredValueSHA       string   `json:"stored_value_sha256"`
	PreTargetRecords     int      `json:"pre_target_records"`
	BranchWrittenRecords int      `json:"branch_written_records"`
	Derived              bool     `json:"derived"`
	DerivedBlockNum      uint64   `json:"derived_block_num,omitempty"`
	Ambiguous            bool     `json:"ambiguous"`
	Candidates           []uint64 `json:"candidates,omitempty"`
	TrustedUsed          bool     `json:"trusted_used,omitempty"`
	TrustedProvenance    string   `json:"trusted_provenance,omitempty"`
	Pass2EndEqualsStored bool     `json:"pass2_end_equals_stored"`
}

// Report is abandoned-branch-audit.json.
type Report struct {
	Tool   string `json:"tool"`
	Schema string `json:"schema"`

	Network string `json:"network"`
	Shard   uint32 `json:"shard"`
	DBPath  string `json:"db_path"`

	AnchorConfigSHA string          `json:"anchor_config_sha256"`
	Anchor          hmr.AnchorTuple `json:"anchor"`

	RangeStart uint64 `json:"range_start"`
	RangeEnd   uint64 `json:"range_end"`
	Blocks     uint64 `json:"blocks"`

	NonAuthoritative bool `json:"non_authoritative,omitempty"` // --single-pass

	KnownBadBlocks       []uint64 `json:"known_bad_blocks"`
	FirstValidityFailure uint64   `json:"first_validity_failure,omitempty"`
	KnownBadCrossChecked bool     `json:"known_bad_cross_checked"`

	Pass1 *PassSection `json:"pass1,omitempty"` // provenance, non-authoritative
	Pass2 *PassSection `json:"pass2,omitempty"` // authoritative

	EpochTransition EpochTransitionSection `json:"epoch_transition"`
	Staking         StakingSection         `json:"staking"`
	Reconciliation  ReconciliationSection  `json:"reconciliation"`
	ShardSubsets    []ShardSubset          `json:"shard_subsets"`
	Pointers        []PointerResult        `json:"pointers"`

	// Optional --reference cross-check (nil when not supplied).
	Reference *ReferenceSection `json:"reference,omitempty"`

	// Source-immutability evidence: fingerprints compared before/after
	// (device/inode included); inequality is exit 14.
	FingerprintBefore *dbopen.Fingerprint `json:"db_fingerprint_before"`
	FingerprintAfter  *dbopen.Fingerprint `json:"db_fingerprint_after"`
	SourceUnchanged   bool                `json:"source_unchanged"`

	StartedAt string  `json:"started_at"`
	DurationS float64 `json:"duration_s"`

	Verdict  string `json:"verdict"`
	ExitCode int    `json:"exit_code"`
}

// ReferenceSection binds the audit to the exported reference manifest: the
// manifest must carry the same anchor config SHA and anchor tuple AND its
// normalized-content digest must equal the manifest the audit rebuilds from
// its OWN normalization result (package/section/wrapper/diagnostics/
// assertion digests). The hash chain seals anchor → reference digest →
// per-pass outcome digests.
type ReferenceSection struct {
	File                string `json:"file"`
	ManifestSHA         string `json:"manifest_sha256"` // THE reference digest (supplied file)
	AnchorConfigOK      bool   `json:"anchor_config_sha256_match"`
	AnchorTupleOK       bool   `json:"anchor_tuple_match"`
	ContentMatch        bool   `json:"content_match"`
	ExpectedManifestSHA string `json:"expected_manifest_sha256"` // rebuilt from the audit's normalization
	HashChain           string `json:"hash_chain_sha256"`
}

// loadReference reads, strictly decodes and cross-checks the reference
// manifest against the resolved anchor AND against the manifest the audit
// rebuilds from its own normalization result. Any mismatch — anchor tuple,
// config SHA, or normalized content (a forged package/section/wrapper/
// diagnostics/assertion digest) — is an invocation error (exit 15): the
// supplied reference does not belong to this source under this anchor.
func loadReference(rep *Report, path string, res *anchor.Resolved, normA norm.Anchor, nres *norm.Result, stderr io.Writer) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: read reference manifest: %v\n", err)
		return report.ExitIO
	}
	man, err := hmr.DecodeManifest(raw)
	if err != nil {
		fmt.Fprintf(stderr, "invalid invocation: reference manifest: %v\n", err)
		return report.ExitBadInvocation
	}
	sec := &ReferenceSection{File: filepath.Base(path), ManifestSHA: report.SHA256Hex(raw)}
	sec.AnchorConfigOK = man.AnchorConfigSHA == res.ConfigSHAHex()
	sec.AnchorTupleOK = man.Anchor == rep.Anchor && man.Network == rep.Network && man.Shard == rep.Shard

	// Rebuild the expected manifest from the audit's own normalization and
	// compare byte-for-byte: this binds the reference to the normalized
	// CONTENT, not just the anchor identity.
	pkg, err := hmr.Encode(nres.Normalized, res.ConfigSHA)
	if err != nil {
		fmt.Fprintf(stderr, "error: rebuild reference package: %v\n", err)
		return report.ExitIO
	}
	expected, err := hmr.EncodeManifest(hmr.BuildManifest(normA, nres, pkg))
	if err != nil {
		fmt.Fprintf(stderr, "error: rebuild reference manifest: %v\n", err)
		return report.ExitIO
	}
	sec.ExpectedManifestSHA = report.SHA256Hex(expected)
	sec.ContentMatch = bytes.Equal(expected, raw)
	rep.Reference = sec
	if !sec.AnchorConfigOK || !sec.AnchorTupleOK || !sec.ContentMatch {
		fmt.Fprintf(stderr, "invalid invocation: reference manifest does not match this source under the resolved anchor (config match %v, tuple match %v, content match %v)\n",
			sec.AnchorConfigOK, sec.AnchorTupleOK, sec.ContentMatch)
		return report.ExitBadInvocation
	}
	return 0
}

func newReport(res *anchor.Resolved, open *source.Open, opts Options, endHeight uint64, started time.Time) *Report {
	return &Report{
		Tool:            Tool,
		Schema:          Schema,
		Network:         res.Config.Network,
		Shard:           res.Config.Shard,
		DBPath:          open.DB.Path(),
		AnchorConfigSHA: res.ConfigSHAHex(),
		Anchor: hmr.AnchorTuple{
			TargetHeight:       open.NormA.TargetHeight,
			TargetHash:         open.NormA.TargetHash.Hex(),
			TargetRoot:         open.NormA.TargetRoot.Hex(),
			Epoch:              open.NormA.Epoch,
			EpochFirstBlock:    open.NormA.EpochFirst,
			EpochLastBlock:     open.NormA.EpochLast,
			SnapshotBaseHeight: open.NormA.SnapshotBase,
			AbandonedChildHash: open.NormA.AbandonedChildHash.Hex(),
		},
		RangeStart:     res.Config.TargetHeight + 1,
		RangeEnd:       endHeight,
		Blocks:         endHeight - res.Config.TargetHeight,
		KnownBadBlocks: res.Config.KnownBadBlocks,
		StartedAt:      started.UTC().Format(time.RFC3339),
	}
}

func passSection(label string, authoritative bool, pr *passResult) *PassSection {
	if pr == nil {
		return nil
	}
	sec := &PassSection{
		Label:                 label,
		Authoritative:         authoritative,
		Seed:                  pr.SeedSpec,
		Fatal:                 pr.Fatal,
		FatalHeight:           pr.FatalHeight,
		FatalReason:           pr.FatalReason,
		LegacyBitmapsRestored: pr.LegacyBitmapsRestored,
	}
	for _, o := range pr.Outcomes {
		if o.Executed {
			sec.ExecutedBlocks++
		}
		if o.RootMatched {
			sec.RootsMatched++
		}
		if len(o.ValidityFails) > 0 {
			sec.ValidityFailures++
			if len(sec.FailedOutcomes) < scan.ItemLimit {
				sec.FailedOutcomes = append(sec.FailedOutcomes, o)
			}
		}
	}
	sec.OutcomeCount = len(pr.Outcomes)
	sec.OutcomesSHA256, _ = report.DigestCanonicalJSON(pr.Outcomes)
	sec.Findings, _ = scan.BuildFindingsSection(pr.Findings)
	return sec
}

// shardSubsets extracted from a pass write log.
type shardSubsets struct {
	byShard map[uint32]*ShardSubset
}

func (s *shardSubsets) allKeys() [][]byte {
	var out [][]byte
	ids := make([]uint32, 0, len(s.byShard))
	for id := range s.byShard {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		sub := s.byShard[id]
		for _, k := range sub.CrossLinkKeys {
			out = append(out, common.FromHex(k))
		}
		for _, k := range sub.SpentKeys {
			out = append(out, common.FromHex(k))
		}
	}
	return out
}

func (s *shardSubsets) sorted() []ShardSubset {
	ids := make([]uint32, 0, len(s.byShard))
	for id := range s.byShard {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]ShardSubset, 0, len(ids))
	for _, id := range ids {
		out = append(out, *s.byShard[id])
	}
	return out
}

func extractShardSubsets(log map[string]WriteLogEntry) *shardSubsets {
	s := &shardSubsets{byShard: map[uint32]*ShardSubset{}}
	get := func(sid uint32) *ShardSubset {
		sub, ok := s.byShard[sid]
		if !ok {
			sub = &ShardSubset{ShardID: sid}
			s.byShard[sid] = sub
		}
		return sub
	}
	keys := make([]string, 0, len(log))
	for k := range log {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ns, meta := strictdb.Classify([]byte(k))
		switch ns {
		case strictdb.NsCrossLink:
			sub := get(meta.ShardID)
			sub.CrossLinkNums = append(sub.CrossLinkNums, meta.Number)
			sub.CrossLinkKeys = append(sub.CrossLinkKeys, fmt.Sprintf("%x", k))
		case strictdb.NsCXReceiptSpent:
			sub := get(meta.ShardID)
			sub.SpentNums = append(sub.SpentNums, meta.Number)
			sub.SpentKeys = append(sub.SpentKeys, fmt.Sprintf("%x", k))
		case strictdb.NsCrossLinkPointer:
			get(meta.ShardID).PointerWritten = true
		}
	}
	return s
}

// pointerPlan carries solver outputs into pass 2 and the report.
type pointerPlan struct {
	results []PointerResult
	seeds   map[string][]byte // pointer key -> derived record value
}

// solvePointers runs the §4.4 invariant solver per shard.
func solvePointers(sd *side, nres *norm.Result, subsets *shardSubsets, opts Options, rep *Report, stderr io.Writer) (*pointerPlan, int) {
	// Source crosslink record sets + pointers per shard.
	type srcShard struct {
		nums    map[uint64]bool
		values  map[uint64][]byte
		pointer []byte
	}
	shards := map[uint32]*srcShard{}
	get := func(sid uint32) *srcShard {
		sh, ok := shards[sid]
		if !ok {
			sh = &srcShard{nums: map[uint64]bool{}, values: map[uint64][]byte{}}
			shards[sid] = sh
		}
		return sh
	}
	err := strictdb.ForEach(sd.kv, []byte("cl"), func(key, value []byte) error {
		ns, meta := strictdb.Classify(key)
		switch ns {
		case strictdb.NsCrossLink:
			sh := get(meta.ShardID)
			sh.nums[meta.Number] = true
			sh.values[meta.Number] = append([]byte(nil), value...)
		case strictdb.NsCrossLinkPointer:
			get(meta.ShardID).pointer = append([]byte(nil), value...)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: read source crosslinks: %v\n", err)
		return nil, report.ExitIO
	}

	trustedShard, trustedNum, trustedSet, terr := parseTrustedPointer(opts.TrustedShard1Pointer)
	if terr != nil {
		fmt.Fprintf(stderr, "invalid --trusted-shard1-pointer: %v\n", terr)
		return nil, report.ExitBadInvocation
	}

	plan := &pointerPlan{seeds: map[string][]byte{}}
	anyAmbiguous := false
	ids := make([]uint32, 0, len(shards))
	for id := range shards {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, sid := range ids {
		sh := shards[sid]
		if sh.pointer == nil {
			continue // shard never crosslinked; nothing to solve
		}
		prr := PointerResult{ShardID: sid, StoredValueSHA: report.SHA256Hex(sh.pointer)}
		if cl, err := types.DeserializeCrossLink(sh.pointer); err == nil {
			prr.StoredBlockNum = cl.BlockNum()
		} else {
			fmt.Fprintf(stderr, "error: stored pointer for shard %d undecodable: %v\n", sid, err)
			return nil, report.ExitIO
		}

		branch := map[uint64]bool{}
		if sub, ok := subsets.byShard[sid]; ok {
			for _, n := range sub.CrossLinkNums {
				branch[n] = true
			}
		}
		sPre := map[uint64]bool{}
		for n := range sh.nums {
			if !branch[n] {
				sPre[n] = true
			}
		}
		prr.PreTargetRecords = len(sPre)
		prr.BranchWrittenRecords = len(branch)

		q := prr.StoredBlockNum
		if trustedSet && trustedShard == sid {
			if err := ValidateTrustedPointer(trustedNum, sPre, branch, q); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return nil, report.ExitBadInvocation
			}
			prr.Derived, prr.DerivedBlockNum, prr.TrustedUsed = true, trustedNum, true
			prr.TrustedProvenance = opts.TrustedProvenance
		} else {
			p, ok, candidates := SolvePointer(sPre, branch, q)
			prr.Candidates = candidates
			if ok {
				prr.Derived, prr.DerivedBlockNum = true, p
			} else {
				prr.Ambiguous = true
				anyAmbiguous = true
				fmt.Fprintf(stderr, "POINTER_AMBIGUOUS: shard %d has %d gap-maximal candidates %v (supply --trusted-shard1-pointer)\n",
					sid, len(candidates), candidates)
			}
		}
		if prr.Derived {
			val, ok := sh.values[prr.DerivedBlockNum]
			if !ok {
				fmt.Fprintf(stderr, "error: derived pointer %d for shard %d has no retained record\n", prr.DerivedBlockNum, sid)
				return nil, report.ExitIO
			}
			key := append([]byte("cl"), u32be4(sid)...)
			plan.seeds[string(key)] = val
		}
		plan.results = append(plan.results, prr)
	}
	if anyAmbiguous {
		return plan, report.ExitAuditAnomaly
	}
	return plan, 0
}

func parseTrustedPointer(s string) (uint32, uint64, bool, error) {
	if s == "" {
		return 0, 0, false, nil
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("want <shardID>:<blockNum>, got %q", s)
	}
	sid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, 0, false, err
	}
	num, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false, err
	}
	return uint32(sid), num, true, nil
}

func writeFatalReport(rep *Report, p1, p2 *passResult, opts Options, stderr io.Writer, code int) {
	rep.Pass1 = passSection("pass-1", false, p1)
	rep.Pass2 = passSection("pass-2", !opts.SinglePass, p2)
	rep.ExitCode = code
	rep.Verdict = scan.Verdict(code)
	path := filepath.Join(opts.OutDir, "abandoned-branch-audit.json")
	if err := report.WriteJSONAtomic(path, rep); err != nil {
		fmt.Fprintf(stderr, "error: write audit report: %v\n", err)
		return
	}
	fmt.Fprintf(stderr, "audit report (fatal) written to %s; scratch preserved at %s\n", path, opts.Scratch)
}

// crossCheckKnownBad evaluates the known-bad gate (§4.6 output 1) over the
// authoritative pass outcomes. It is a pure function of the observed validity
// failures and the anchored known-bad list so the gate can be unit-tested
// exhaustively (the only deterministic way to exercise the gate-satisfied
// path locally — see the note below).
//
// The anchored known-bad list is the expected set of validity failures on the
// branch. On mainnet the exploit block MUST fail the PATCHED incoming-receipts
// validation specifically — that is the signature the exploit detector gates
// on. The known-bad entry excuses EXACTLY that one receipt failure and nothing
// else: a seal/VRF/shard-state/header failure is a DIFFERENT defect, and even
// when it coincides with the expected receipt failure at the same height it
// remains anomalous (the anchor did not anticipate it). Anomaly conditions:
//
//	(a) any validity failure at a height NOT in the anchored known-bad set
//	    (an unanticipated invalid block);
//	(b) at a known-bad height, any failure OTHER than incoming-receipts (an
//	    unexpected additional defect the anchor does not excuse);
//	(c) the first anchored known-bad block produced no validity failure at
//	    all (the expected exploit failure is absent); and
//	(d) the first anchored known-bad block failed, but NOT with an
//	    incoming-receipts failure (wrong defect for the exploit gate).
//
// crossChecked is true iff the first known-bad block reproduced the expected
// incoming-receipts failure; it is independent of conditions (a)/(b), which
// record their own gating anomalies (an extra defect at the exploit height
// still fails the audit via the anomaly, even though the exploit signature
// itself was reproduced).
func crossCheckKnownBad(outcomes []blockOutcome, knownBadBlocks []uint64) (crossChecked bool, firstFail uint64, anomalies []Anomaly) {
	add := func(kind, key, detail string) {
		anomalies = append(anomalies, Anomaly{Kind: kind, Key: key, Detail: detail})
	}
	knownBad := map[uint64]bool{}
	for _, h := range knownBadBlocks {
		knownBad[h] = true
	}
	// isIncomingReceiptsFail reports whether a single failure label is the
	// patched incoming-receipts check (recordModeChecks prefixes it with
	// "incoming-receipts: ").
	isIncomingReceiptsFail := func(f string) bool {
		return strings.HasPrefix(f, "incoming-receipts:")
	}
	failsByHeight := map[uint64][]string{}
	for _, o := range outcomes {
		if len(o.ValidityFails) == 0 {
			continue
		}
		if firstFail == 0 {
			firstFail = o.Height
		}
		failsByHeight[o.Height] = o.ValidityFails
		if !knownBad[o.Height] {
			add("unexpected-validity-failure", fmt.Sprintf("%d", o.Height),
				fmt.Sprintf("block %d failed validity checks (%v) but is not in the anchor's known-bad list", o.Height, o.ValidityFails))
			continue
		}
		// Known-bad height: the anchor excuses ONLY the incoming-receipts
		// failure. Any other failure at the same height is an additional,
		// unanticipated defect and must still surface as an anomaly (condition
		// (b)) — the known-bad entry is not a blanket amnesty for the height.
		for _, f := range o.ValidityFails {
			if isIncomingReceiptsFail(f) {
				continue
			}
			add("known-bad-extra-failure", fmt.Sprintf("%d", o.Height),
				fmt.Sprintf("known-bad block %d additionally failed %q; the anchor only excuses the incoming-receipts exploit failure, not unrelated seal/VRF/shard-state/header defects", o.Height, f))
		}
	}
	if len(knownBadBlocks) > 0 {
		kb := knownBadBlocks[0]
		fails, failed := failsByHeight[kb]
		var sawReceipts bool
		for _, f := range fails {
			if isIncomingReceiptsFail(f) {
				sawReceipts = true
			}
		}
		switch {
		case !failed:
			add("known-bad-failure-absent", fmt.Sprintf("%d", kb),
				fmt.Sprintf("anchor expects a validity failure at known-bad block %d, none observed — the expected receipt-validation failure was not reproduced", kb))
		case !sawReceipts:
			add("known-bad-wrong-failure", fmt.Sprintf("%d", kb),
				fmt.Sprintf("known-bad block %d failed validity checks %v but NOT incoming-receipts; the exploit-detector gate requires the patched incoming-receipts failure and rejects an unrelated seal/VRF/shard-state defect", kb, fails))
		default:
			crossChecked = true
		}
	}
	return crossChecked, firstFail, anomalies
}

// assembleReport runs the bidirectional reconciliation (§4.6 outputs 3-6)
// over the authoritative pass and fills the report. Returns the exit code.
func assembleReport(rep *Report, nres *norm.Result, sd *side, open *source.Open,
	pass1, auth *passResult, subsets *shardSubsets, pointer *pointerPlan, pointerAmbiguous bool) int {

	rep.Pass1 = passSection("pass-1", auth == pass1, pass1)
	if auth != pass1 {
		rep.Pass2 = passSection("pass-2", true, auth)
	}

	var anomalies []Anomaly
	addAnomaly := func(kind, key, detail string) {
		anomalies = append(anomalies, Anomaly{Kind: kind, Key: key, Detail: detail})
	}

	// srcGet wraps side.Get, latching genuine I/O failures: a read error is
	// exit 14 (I/O), never re-classified as absence or an audit anomaly.
	var ioErr error
	srcGet := func(key []byte) ([]byte, bool) {
		v, ok, err := sd.Get(key)
		if err != nil {
			if ioErr == nil {
				ioErr = fmt.Errorf("source read %x: %w", key, err)
			}
			addAnomaly("source-io-error", fmt.Sprintf("%x", key), err.Error())
			return nil, false
		}
		return v, ok
	}

	// ---- Output 1: known-bad cross-check — GATING, not informational. ----
	crossChecked, firstFail, kbAnomalies := crossCheckKnownBad(auth.Outcomes, rep.KnownBadBlocks)
	rep.FirstValidityFailure = firstFail
	rep.KnownBadCrossChecked = crossChecked
	anomalies = append(anomalies, kbAnomalies...)

	// ---- Output 4: bidirectional plan/write reconciliation. ----
	planKeys := map[string]string{} // raw key -> reason
	for _, d := range nres.Deletions.Deletions() {
		planKeys[string(common.FromHex(d.Key))] = d.Reason
	}
	for _, rw := range nres.Deletions.Rewrites() {
		planKeys[string(common.FromHex(rw.Key))] = rw.Reason
	}
	rec := &rep.Reconciliation
	rec.PlanKeys = len(planKeys)

	excludedReason := func(key []byte) string {
		ns, _ := strictdb.Classify(key)
		switch ns {
		case strictdb.NsPendingCrossLink, strictdb.NsPendingSlashing:
			return "node-local queue (never reproduced by re-execution)"
		case strictdb.NsLastCommits:
			return "dead legacy key (nothing writes it)"
		case strictdb.NsEpochBlockNumber, strictdb.NsEpochVRF, strictdb.NsEpochVDF:
			return "dead-writer namespace (§2.1)"
		default:
			return ""
		}
	}

	planKeyList := make([]string, 0, len(planKeys))
	for k := range planKeys {
		planKeyList = append(planKeyList, k)
	}
	sort.Strings(planKeyList)
	excludedSeen := map[string]bool{}
	for _, k := range planKeyList {
		key := []byte(k)
		if reason := excludedReason(key); reason != "" {
			rec.Excluded++
			if !excludedSeen[reason] {
				excludedSeen[reason] = true
				rec.ExcludedReasons = append(rec.ExcludedReasons, reason)
			}
			continue
		}
		entry, written := auth.Log[k]
		if !written || entry.Puts == 0 {
			addAnomaly("plan-key-not-reproduced", fmt.Sprintf("%x", k),
				fmt.Sprintf("deletion-plan key (reason %s) was never re-written by the branch re-execution", planKeys[k]))
			continue
		}
		rec.Reproduced++
		// Chain-deterministic namespaces byte-compare final overlay value
		// against the source (non-reverted source expectation).
		final, ferr := auth.overlay.Get(key)
		srcVal, ok := srcGet(key)
		if ferr != nil || !ok {
			addAnomaly("plan-key-compare-unreadable", fmt.Sprintf("%x", k),
				"final or source value unreadable for byte comparison")
			continue
		}
		if !bytes.Equal(final, srcVal) {
			addAnomaly("plan-key-byte-mismatch", fmt.Sprintf("%x", k),
				"re-executed value differs from the source's record (non-reverted source expected byte equality)")
			continue
		}
		rec.ByteEqual++
	}

	// Every post-barrier post-target write must be in the plan, the
	// shard-1/B4 cleanup sets, or the stats namespace — anything else is
	// an anomaly.
	logKeys := make([]string, 0, len(auth.Log))
	for k := range auth.Log {
		logKeys = append(logKeys, k)
	}
	sort.Strings(logKeys)
	target := rep.Anchor.TargetHeight
	for _, k := range logKeys {
		key := []byte(k)
		if _, inPlan := planKeys[k]; inPlan {
			rec.Writes.PlanReconciled++
			continue
		}
		ns, meta := strictdb.Classify(key)
		switch ns {
		case strictdb.NsCrossLink:
			rec.Writes.CrossLinkSubset++
		case strictdb.NsCXReceiptSpent:
			rec.Writes.SpentSubset++
		case strictdb.NsCrossLinkPointer:
			rec.Writes.Pointers++
		case strictdb.NsValidatorStats:
			rec.Writes.Stats++ // kept namespace: inventoried, never anomalous (§8 Q4)
		case strictdb.NsHead:
			rec.Writes.Heads++
		case strictdb.NsPendingCrossLink, strictdb.NsPendingSlashing:
			rec.Writes.PendingQueues++
		case strictdb.NsHeader, strictdb.NsHeaderTD, strictdb.NsCanonicalHash, strictdb.NsBody,
			strictdb.NsReceipts, strictdb.NsBlockCommitSig, strictdb.NsSkeleton:
			if meta.Number != 0 && meta.Number <= target {
				addAnomaly("chain-record-below-target", fmt.Sprintf("%x", k),
					fmt.Sprintf("%s write at height %d <= target", ns, meta.Number))
				rec.Writes.Anomalous++
				continue
			}
			rec.Writes.ChainRecords++
		case strictdb.NsHeaderNumber, strictdb.NsTxLookup, strictdb.NsCXLookup, strictdb.NsCXReceipt:
			rec.Writes.ChainRecords++ // hash-keyed lookup/cx material (B4 cleanup domain)
		case strictdb.NsStateNode, strictdb.NsCode, strictdb.NsValidatorCode, strictdb.NsPreimage, strictdb.NsBloom:
			rec.Writes.StateMaterial++
		case strictdb.NsValidatorList, strictdb.NsDVL, strictdb.NsValidatorSnapshot,
			strictdb.NsShardState, strictdb.NsBlockRewardAccum:
			// Metadata write outside the plan: acceptable only as a no-op
			// rewrite (e.g. a top-up delegate rewriting identical dvl
			// bytes).
			final, ferr := auth.overlay.Get(key)
			srcVal, ok := srcGet(key)
			if ferr == nil && ok && bytes.Equal(final, srcVal) {
				rec.Writes.MetadataNoOp++
				continue
			}
			addAnomaly("unplanned-metadata-write", fmt.Sprintf("%x", k),
				fmt.Sprintf("%s write outside the deletion plan whose value does not match the source", ns))
			rec.Writes.Anomalous++
		default:
			addAnomaly("unclassified-write", fmt.Sprintf("%x", k),
				fmt.Sprintf("post-barrier write to unexpected namespace %s", ns))
			rec.Writes.Anomalous++
		}
	}

	// ---- Output 5: staking-metadata reconciliation. ----
	exitStaking := stakingReconciliation(rep, nres, auth, addAnomaly)

	// ---- Output 3: epoch-transition byte-equality record. ----
	epochTransition(rep, nres, auth, srcGet)

	// ---- Output 6: shard subsets + pointer end-state check. ----
	rep.ShardSubsets = subsets.sorted()
	for i := range pointer.results {
		prr := &pointer.results[i]
		key := append([]byte("cl"), u32be4(prr.ShardID)...)
		final, err := auth.overlay.Get(key)
		srcVal, ok := srcGet(key)
		if err == nil && ok && bytes.Equal(final, srcVal) {
			prr.Pass2EndEqualsStored = true
		} else if prr.Derived {
			addAnomaly("pointer-end-state-mismatch", fmt.Sprintf("%x", key),
				fmt.Sprintf("pass-2 replay seeded with %d did not end byte-equal to the stored pointer", prr.DerivedBlockNum))
		}
	}
	rep.Pointers = pointer.results

	// Bounded anomaly section.
	rec.AnomalyCount = len(anomalies)
	if len(anomalies) > scan.ItemLimit {
		rec.Anomalies = anomalies[:scan.ItemLimit]
		rec.AnomaliesOmitted = len(anomalies) - scan.ItemLimit
	} else {
		rec.Anomalies = anomalies
	}

	exit := report.ExitOK
	if rec.AnomalyCount > 0 || pointerAmbiguous {
		exit = report.ExitAuditAnomaly
	}
	exit = report.ResolveExit(exit, exitStaking)
	if ioErr != nil {
		// Genuine source read failures during reconciliation are I/O (exit
		// 14), not audit anomalies (14 outranks 24 in the precedence table).
		exit = report.ResolveExit(exit, report.ExitIO)
	}
	rep.Verdict = scan.Verdict(exit)
	return exit
}

// stakingReconciliation binds operations to metadata effects (§4.6 output
// 5): CreateValidator addresses <-> removals bidirectionally; delegations
// classified attempted / StakeMsgs-visible / metadata-producing with the
// binding requirement bidirectional between metadata-producing delegations
// and removed dvl entries.
func stakingReconciliation(rep *Report, nres *norm.Result, auth *passResult, addAnomaly func(kind, key, detail string)) int {
	sec := &rep.Staking
	sec.NativeByDirective = map[string]int{}
	sec.PrecompileByKind = map[string]int{}

	removedVals := map[string]bool{}
	for _, a := range nres.RemovedValidators {
		removedVals[a.Hex()] = true
		sec.RemovedValidators = append(sec.RemovedValidators, a.Hex())
	}
	sort.Strings(sec.RemovedValidators)

	// Removed dvl entries keyed by the full effect tuple
	// {delegator, validator, blockNum}: a delegation is metadata-producing
	// only if a removed entry exists for that exact pair AT that exact
	// block (the dvl index BlockNum is the block the index was appended
	// at), and each removed entry is consumed exactly once. Subsequent
	// delegations to an already-indexed pair (top-ups) match no tuple and
	// are correctly NOT metadata-producing (addDelegationIndex appends
	// only when the pair is absent, §2.1).
	tupleKey := func(delegator, validator string, block uint64) string {
		return delegator + "|" + validator + "|" + strconv.FormatUint(block, 10)
	}
	removedTuples := map[string]bool{} // tuple -> consumed
	for _, e := range nres.RemovedDVLEntries {
		removedTuples[tupleKey(e.Delegator.Hex(), e.Validator.Hex(), e.BlockNum)] = false
	}
	consume := func(delegator, validator string, block uint64) bool {
		k := tupleKey(delegator, validator, block)
		consumed, ok := removedTuples[k]
		if !ok || consumed {
			return false
		}
		removedTuples[k] = true
		return true
	}

	created := map[string]bool{}
	var delegations []DelegateClass
	for _, op := range auth.NativeOps {
		sec.NativeByDirective[op.Directive]++
		switch op.Directive {
		case "CreateValidator":
			created[op.Address] = true
			// CreateValidator implies a self-delegation index
			// {validator, 0, blockNum} (prepareStakingMetaData); it
			// reconciles the removed self-delegation dvl tuple at the
			// create block.
			consume(op.Address, op.Address, op.Block)
		case "Delegate":
			dc := DelegateClass{
				Source: "native", Block: op.Block,
				Delegator: op.Address, Validator: op.Validator,
				Attempted: true, StakeMsgsVisible: false,
			}
			dc.MetadataProducing = consume(op.Address, op.Validator, op.Block)
			delegations = append(delegations, dc)
		}
	}
	sec.CreatedValidators = make([]string, 0, len(created))
	for a := range created {
		sec.CreatedValidators = append(sec.CreatedValidators, a)
	}
	sort.Strings(sec.CreatedValidators)

	for _, op := range auth.FCOps {
		sec.PrecompileByKind[op.Kind]++
		if op.Kind != "Delegate" {
			continue
		}
		dc := DelegateClass{
			Source: "precompile", Block: op.Block,
			Delegator: op.Delegator, Validator: op.Validator,
			Attempted:         true,
			StakeMsgsVisible:  !op.FrameFailed, // appended to evm.StakeMsgs on frame success, survives enclosing reverts
			FrameFailed:       op.FrameFailed,
			EnclosingReverted: op.EnclosingReverted,
		}
		if dc.StakeMsgsVisible && !op.EnclosingReverted {
			dc.MetadataProducing = consume(op.Delegator, op.Validator, op.Block)
		}
		delegations = append(delegations, dc)
	}

	// Bidirectional checks.
	for _, a := range sec.RemovedValidators {
		if !created[a] {
			addAnomaly("removed-validator-without-create", a,
				"validator removed from the normalized list has no observed post-target CreateValidator")
		}
	}
	for _, a := range sec.CreatedValidators {
		if !removedVals[a] {
			addAnomaly("created-validator-without-removal", a,
				"post-target CreateValidator has no matching validator-list removal")
		}
	}
	tupleList := make([]string, 0, len(removedTuples))
	for p := range removedTuples {
		tupleList = append(tupleList, p)
	}
	sort.Strings(tupleList)
	for _, p := range tupleList {
		if !removedTuples[p] {
			addAnomaly("removed-dvl-entry-without-delegation", p,
				"removed dvl entry (delegator|validator|blockNum) has no observed metadata-producing delegation at that block")
		}
	}

	sort.SliceStable(delegations, func(i, j int) bool {
		if delegations[i].Block != delegations[j].Block {
			return delegations[i].Block < delegations[j].Block
		}
		return delegations[i].Delegator < delegations[j].Delegator
	})
	if len(delegations) > scan.ItemLimit {
		sec.Delegations = delegations[:scan.ItemLimit]
		sec.DelegationsOmitted = len(delegations) - scan.ItemLimit
	} else {
		sec.Delegations = delegations
	}
	if len(auth.NativeOps) > scan.ItemLimit {
		sec.Native = auth.NativeOps[:scan.ItemLimit]
		sec.NativeOmitted = len(auth.NativeOps) - scan.ItemLimit
	} else {
		sec.Native = auth.NativeOps
	}
	if len(auth.FCOps) > scan.ItemLimit {
		sec.Precompile = auth.FCOps[:scan.ItemLimit]
		sec.PrecompileOmitted = len(auth.FCOps) - scan.ItemLimit
	} else {
		sec.Precompile = auth.FCOps
	}
	return report.ExitOK
}

// epochTransition records the §4.6 output-3 byte-equality evidence: the
// reproduced next-epoch ss and snapshot batch byte-compared to the
// source's to-be-deleted records (the main compensating evidence for low
// snapshot-reconstruction coverage, §8 Q1). The per-key comparisons are
// already part of output 4; this section aggregates them.
func epochTransition(rep *Report, nres *norm.Result, auth *passResult, srcGet func([]byte) ([]byte, bool)) {
	sec := &rep.EpochTransition
	nextEpoch := rep.Anchor.Epoch + 1
	ssKey := append([]byte("ss"), newBig(nextEpoch)...)
	if entry, ok := auth.Log[string(ssKey)]; ok && entry.Puts > 0 {
		sec.Observed = true
		sec.NextEpoch = nextEpoch
		final, err := auth.overlay.Get(ssKey)
		srcVal, ok := srcGet(ssKey)
		sec.ShardStateEqual = err == nil && ok && bytes.Equal(final, srcVal)
	}
	// Snapshot batch: future-epoch snapshot deletions rewritten by the
	// replay.
	for _, d := range nres.Deletions.Deletions() {
		if d.Reason != "future-epoch" {
			continue
		}
		key := common.FromHex(d.Key)
		ns, _ := strictdb.Classify(key)
		if ns != strictdb.NsValidatorSnapshot {
			continue
		}
		sec.SnapshotBatch++
		if entry, ok := auth.Log[string(key)]; ok && entry.Puts > 0 {
			final, err := auth.overlay.Get(key)
			srcVal, ok2 := srcGet(key)
			if err == nil && ok2 && bytes.Equal(final, srcVal) {
				sec.SnapshotBatchEqual++
			}
		}
	}
}

func newBig(n uint64) []byte {
	if n == 0 {
		return nil
	}
	var buf [8]byte
	i := 8
	for n > 0 {
		i--
		buf[i] = byte(n)
		n >>= 8
	}
	return buf[i:]
}
