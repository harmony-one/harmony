package norm

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/recovery/report"
	staking "github.com/harmony-one/harmony/staking/types"
)

// wrapperEntry caches one listed validator's target-root wrapper.
type wrapperEntry struct {
	addr    common.Address
	code    []byte // raw stored bytes at the target root (never re-encoded)
	wrapper *staking.ValidatorWrapper
}

// normalizer carries the working state of one Normalize run.
type normalizer struct {
	a Anchor
	s Sources

	findings []report.Finding

	// validator list
	normalizedList []common.Address
	removedList    []common.Address
	wrappers       []wrapperEntry         // normalized list order
	wrapperByAddr  map[common.Address]int // index into wrappers

	// deletion plan accumulation (ordered)
	dvlDeletions   []PlannedDeletion
	dvlRewrites    []PlannedRewrite
	snapDeletions  []PlannedDeletion
	epochDeletions []PlannedDeletion

	// normalized set accumulation
	set NormalizedSet

	counts    Counts
	coverage  SnapshotCoverage
	inventory Inventory

	// retained dvl reverse map: delegator -> validator -> index (for the
	// reverse-completeness pass); memory bounded by total delegations.
	retained map[common.Address]map[common.Address]uint64

	// per-assertion planned-deletion counters
	dvlEntriesRemoved uint64
	removedDVL        []RemovedDVLEntry
}

func (n *normalizer) addFinding(sev report.Severity, class report.Class, code string, key []byte, det string, chainDet bool) {
	f := report.Finding{
		Severity: sev, Class: class, Code: code, Detail: det,
		ChainDeterministic: chainDet,
	}
	if key != nil {
		f.Key = hexKey(key)
	}
	n.findings = append(n.findings, f)
}

func (n *normalizer) fatal(class report.Class, code string, key []byte, det string) {
	n.addFinding(report.SeverityFatal, class, code, key, det, false)
}

// Normalize runs the full §4.4 ruleset. Infrastructure failures (I/O)
// return an error (exit 14 at the CLI); semantic outcomes are Findings.
// When Sources.Ctx is set, cancellation interrupts the raw iterations and
// returns an error wrapping context.Canceled (exit 130/16 at the CLI).
func Normalize(a Anchor, s Sources) (*Result, error) {
	s.Raw = withCtx(s.Raw, s.Ctx)
	n := &normalizer{
		a: a, s: s,
		wrapperByAddr: map[common.Address]int{},
		retained:      map[common.Address]map[common.Address]uint64{},
	}
	if err := n.validatorList(); err != nil {
		return nil, err
	}
	if err := n.snapshots(); err != nil {
		return nil, err
	}
	if err := n.dvl(); err != nil {
		return nil, err
	}
	n.dvlReverseCompleteness()
	if err := n.shardStates(); err != nil {
		return nil, err
	}
	if err := n.epochAux(); err != nil {
		return nil, err
	}
	if err := n.rewardAccumulator(); err != nil {
		return nil, err
	}
	if err := n.pendingAndLegacy(); err != nil {
		return nil, err
	}
	if err := n.surveyInventory(); err != nil {
		return nil, err
	}
	// A latched target-state read error means the target trie is not fully
	// available: everything derived from it is unreliable (exit 22).
	if err := s.Target.Error(); err != nil {
		n.fatal(report.ClassTargetStateUnavailable, "target-state-read-error", nil, err.Error())
	}

	plan := n.assemblePlan()
	assertions := n.assembleAssertions(plan)
	report.SortFindings(n.findings)
	digests, err := digestSet(a, &n.set, n.wrappers, n.findings)
	if err != nil {
		return nil, err
	}
	return &Result{
		Normalized:           &n.set,
		Deletions:            plan,
		Findings:             n.findings,
		Digests:              digests,
		Assertions:           assertions,
		Coverage:             n.coverage,
		Counts:               n.counts,
		Inventory:            n.inventory,
		NormalizedListLength: len(n.normalizedList),
		RemovedValidators:    n.removedList,
		RemovedDVLEntries:    n.removedDVL,
	}, nil
}

// validatorList implements the §4.4 validator-list rules over the raw key
// (never the fail-open rawdb.ReadValidatorList).
func (n *normalizer) validatorList() error {
	has, err := n.s.Raw.Has(prefixValidatorList)
	if err != nil {
		return fmt.Errorf("norm: probe validator-list: %w", err)
	}
	if !has {
		n.counts.ValidatorList.Missing++
		n.fatal(report.ClassMissingRequired, "validator-list-missing", prefixValidatorList,
			"validator-list key absent (clean-DB fallback signal, handoff §2.4)")
		// Continue with an empty list: downstream sections still report.
		n.set.ValidatorList = Record{Key: append([]byte(nil), prefixValidatorList...), Value: mustRLP([]common.Address{})}
		return nil
	}
	raw, err := n.s.Raw.Get(prefixValidatorList)
	if err != nil {
		return fmt.Errorf("norm: read validator-list: %w", err)
	}
	var list []common.Address
	if err := rlp.DecodeBytes(raw, &list); err != nil {
		n.counts.ValidatorList.Invalid++
		n.fatal(report.ClassInvalidRetained, "validator-list-undecodable", prefixValidatorList,
			fmt.Sprintf("strict RLP decode failed: %v", err))
		n.set.ValidatorList = Record{Key: append([]byte(nil), prefixValidatorList...), Value: mustRLP([]common.Address{})}
		return nil
	}

	seen := map[common.Address]bool{}
	for _, addr := range list {
		if seen[addr] {
			// The writer cannot produce duplicates (§2.1): keep first
			// occurrence, ReviewItem (chain-deterministic: retained content).
			n.counts.ValidatorList.Duplicate++
			n.addFinding(report.SeverityReviewItem, report.ClassDiagnostic,
				"validator-list-duplicate", prefixValidatorList,
				fmt.Sprintf("duplicate address %s keeps first occurrence", addr.Hex()), true)
			continue
		}
		seen[addr] = true

		flag := n.s.Target.IsValidator(addr)
		code := n.s.Target.GetCode(addr)
		if !flag && len(code) == 0 {
			// Absent from target state entirely: validators are never
			// deleted from state, so absence proves post-target origin.
			// The audit reconciles each removal with an observed
			// post-target CreateValidator.
			n.counts.ValidatorList.Removed++
			n.removedList = append(n.removedList, addr)
			n.addFinding(report.SeverityInfo, report.ClassDiagnostic,
				"validator-post-target-creation", nil,
				fmt.Sprintf("listed validator %s absent from target state (no flag, no code): removed", addr.Hex()), false)
			continue
		}
		// Present in state: every wrapper check must hold.
		entry := wrapperEntry{addr: addr, code: append([]byte(nil), code...)}
		invalid := false
		if !flag {
			n.fatal(report.ClassInvalidRetained, "validator-flag-missing", nil,
				fmt.Sprintf("listed validator %s has wrapper code but no IsValidator flag at the target root", addr.Hex()))
			invalid = true
		}
		if len(code) == 0 {
			n.fatal(report.ClassInvalidRetained, "validator-wrapper-missing", nil,
				fmt.Sprintf("listed validator %s is flagged but has no wrapper code at the target root", addr.Hex()))
			invalid = true
		} else {
			var w staking.ValidatorWrapper
			if err := rlp.DecodeBytes(code, &w); err != nil {
				n.fatal(report.ClassInvalidRetained, "validator-wrapper-undecodable", nil,
					fmt.Sprintf("listed validator %s wrapper bytes do not decode: %v", addr.Hex(), err))
				invalid = true
			} else {
				if w.Address != addr {
					n.fatal(report.ClassInvalidRetained, "validator-wrapper-unbound", nil,
						fmt.Sprintf("listed validator %s wrapper embeds address %s", addr.Hex(), w.Address.Hex()))
					invalid = true
				}
				if w.CreationHeight == nil || !w.CreationHeight.IsUint64() || w.CreationHeight.Uint64() > n.a.TargetHeight {
					n.fatal(report.ClassInvalidRetained, "validator-creation-above-target", nil,
						fmt.Sprintf("listed validator %s CreationHeight %v exceeds target %d", addr.Hex(), w.CreationHeight, n.a.TargetHeight))
					invalid = true
				}
				// SanityCheck is diagnostic only: never a removal (§4.4).
				if err := w.SanityCheck(); err != nil {
					n.addFinding(report.SeverityReviewItem, report.ClassDiagnostic,
						"validator-sanity-check", nil,
						fmt.Sprintf("wrapper %s fails SanityCheck: %v", addr.Hex(), err), true)
				}
				entry.wrapper = &w
			}
		}
		if invalid {
			n.counts.ValidatorList.Invalid++
			// Fatal invalid entries stay in the normalized list output so
			// digests reflect the (refused) content deterministically.
		}
		n.counts.ValidatorList.Retained++
		n.normalizedList = append(n.normalizedList, addr)
		n.wrappers = append(n.wrappers, entry)
		n.wrapperByAddr[addr] = len(n.wrappers) - 1
	}

	if len(n.normalizedList) == 0 {
		n.fatal(report.ClassMissingRequired, "validator-list-empty", prefixValidatorList,
			"normalized validator list is empty (clean-DB fallback signal)")
	}

	normalized := mustRLP(n.normalizedList)
	n.set.ValidatorList = Record{Key: append([]byte(nil), prefixValidatorList...), Value: normalized}
	if !bytes.Equal(normalized, raw) {
		n.dvlRewrites = append(n.dvlRewrites, PlannedRewrite{
			Key:            hexKey(prefixValidatorList),
			NewValueSHA256: report.SHA256Hex(normalized),
			Reason:         "post-target-validators-removed",
		})
	}
	return nil
}

// nonNil returns a non-nil copy of b (an empty but present value must not
// collapse to a nil "absent" marker in the normalized set).
func nonNil(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func mustRLP(v interface{}) []byte {
	b, err := rlp.EncodeToBytes(v)
	if err != nil {
		panic(fmt.Sprintf("norm: rlp encode: %v", err))
	}
	return b
}

// assemblePlan builds the four-phase deletion plan in deterministic order.
func (n *normalizer) assemblePlan() *DeletionPlan {
	return &DeletionPlan{Phases: []Phase{
		{Name: PhaseDVL, Deletions: n.dvlDeletions, Rewrites: n.dvlRewrites},
		{Name: PhaseSnap, Deletions: n.snapDeletions},
		{Name: PhaseEpoch, Deletions: n.epochDeletions},
		{Name: PhaseCleanup, Placeholders: []Placeholder{
			{Name: "shard1-crosslink-subset", Detail: "audit-input-required: post-target crosslink keys per non-beacon shard come from abandoned-branch-audit.json"},
			{Name: "shard1-spent-subset", Detail: "audit-input-required: post-target cxReceiptSpent keys per non-beacon shard come from abandoned-branch-audit.json"},
			{Name: "shard1-last-pointer", Detail: "audit-input-required: target-time last-crosslink pointer per non-beacon shard (invariant solver, plan §4.4)"},
			{Name: "canonical-lookup-cleanup", Detail: "B4-owned: canonical/lookup/head cleanup above the target is computed by prepare --apply, never here"},
		}},
	}}
}

// AssertionSpec is the chain-invariant identity of an absence assertion
// (namespace + predicate). The canonical ordered set is a pure function of
// the target epoch and height, so a verifier can reconstruct exactly which
// assertions a genuine reference manifest must carry.
type AssertionSpec struct {
	Namespace string
	Predicate string
}

// CanonicalAssertionSpecs returns the fixed, ordered absence-assertion set
// for the given target epoch and height (mirrors assembleAssertions; every
// assertion's expected_remaining is 0 by contract, plan §4.5).
func CanonicalAssertionSpecs(epoch, targetHeight uint64) []AssertionSpec {
	epochPred := fmt.Sprintf("epoch>%d", epoch)
	heightPred := fmt.Sprintf("number>%d", targetHeight)
	return []AssertionSpec{
		{"validator-snapshot", epochPred},
		{"ss", epochPred},
		{"harmony-epoch-block-number", epochPred},
		{"epoch-vrf-block-numbers", epochPred},
		{"epoch-vdf-block-number", epochPred},
		{"blk-rwd", heightPred},
		{"dvl", fmt.Sprintf("entry.BlockNum>%d", targetHeight)},
		{"pendingCL", "present"},
		{"pendingSC", "present"},
		{"LastCommits", "present"},
	}
}

// assembleAssertions builds the §4.5 absence assertions with report-only
// planned-deletion counts.
func (n *normalizer) assembleAssertions(plan *DeletionPlan) []AbsenceAssertion {
	epochPred := fmt.Sprintf("epoch>%d", n.a.Epoch)
	heightPred := fmt.Sprintf("number>%d", n.a.TargetHeight)
	count := func(ns string, reason string) uint64 {
		var c uint64
		for _, ph := range plan.Phases {
			for _, d := range ph.Deletions {
				if d.Reason == reason && classifyAssertNS(d.Key) == ns {
					c++
				}
			}
		}
		return c
	}
	return []AbsenceAssertion{
		{Namespace: "validator-snapshot", Predicate: epochPred, PlannedDeletions: count("validator-snapshot", "future-epoch"), ExpectedRemaining: 0},
		{Namespace: "ss", Predicate: epochPred, PlannedDeletions: count("ss", "future-epoch"), ExpectedRemaining: 0},
		{Namespace: "harmony-epoch-block-number", Predicate: epochPred, PlannedDeletions: count("harmony-epoch-block-number", "future-epoch-dead-writer"), ExpectedRemaining: 0},
		{Namespace: "epoch-vrf-block-numbers", Predicate: epochPred, PlannedDeletions: count("epoch-vrf-block-numbers", "future-epoch-dead-writer"), ExpectedRemaining: 0},
		{Namespace: "epoch-vdf-block-number", Predicate: epochPred, PlannedDeletions: count("epoch-vdf-block-number", "future-epoch-dead-writer"), ExpectedRemaining: 0},
		{Namespace: "blk-rwd", Predicate: heightPred, PlannedDeletions: count("blk-rwd", "post-target"), ExpectedRemaining: 0},
		{Namespace: "dvl", Predicate: fmt.Sprintf("entry.BlockNum>%d", n.a.TargetHeight), PlannedDeletions: n.dvlEntriesRemoved, ExpectedRemaining: 0},
		{Namespace: "pendingCL", Predicate: "present", PlannedDeletions: count("pendingCL", "node-local-queue"), ExpectedRemaining: 0},
		{Namespace: "pendingSC", Predicate: "present", PlannedDeletions: count("pendingSC", "node-local-queue"), ExpectedRemaining: 0},
		{Namespace: "LastCommits", Predicate: "present", PlannedDeletions: count("LastCommits", "legacy-dead-key"), ExpectedRemaining: 0},
	}
}

// classifyAssertNS maps a hex key to its assertion namespace label.
func classifyAssertNS(hexk string) string {
	k := common.FromHex(hexk)
	switch {
	case bytes.HasPrefix(k, prefixSnapshot):
		return "validator-snapshot"
	case bytes.HasPrefix(k, prefixEpochNumber):
		return "harmony-epoch-block-number"
	case bytes.HasPrefix(k, prefixEpochVRF):
		return "epoch-vrf-block-numbers"
	case bytes.HasPrefix(k, prefixEpochVDF):
		return "epoch-vdf-block-number"
	case bytes.HasPrefix(k, prefixBlkRwd):
		return "blk-rwd"
	case bytes.Equal(k, keyLastCommits):
		return "LastCommits"
	case bytes.HasPrefix(k, prefixPendingCL):
		return "pendingCL"
	case bytes.HasPrefix(k, prefixPendingSC):
		return "pendingSC"
	case bytes.HasPrefix(k, prefixShardState):
		return "ss"
	case bytes.HasPrefix(k, prefixDVL):
		return "dvl"
	default:
		return "other"
	}
}

// sortRecords orders records by raw key (bytewise ascending).
func sortRecords(rs []Record) {
	sort.Slice(rs, func(i, j int) bool { return bytes.Compare(rs[i].Key, rs[j].Key) < 0 })
}
