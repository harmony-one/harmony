// Package norm is THE shared, deterministic validator-metadata
// normalization library (plan §4.3/§4.4): one implementation consumed by
// `metadata scan`, `metadata export-reference`, `metadata audit-branch`
// (mask construction) and — as the documented interface — B4 apply and
// verify. Divergent implementations are the failure mode handoff §6 bans.
//
// Behavior is fixed by the anchor plus RulesetVersion (no policy knobs, §8
// Q4/Q5: validator-stats kept untouched, the target reward accumulator
// included). Determinism rules: iteration strictly in ascending raw-key
// order, no Go map iteration reaches any output, no wall-clock content in
// digested material, and Normalize performs zero writes (it only receives
// read interfaces; tests additionally run it over a write-refusing
// wrapper).
package norm

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// RulesetVersion is embedded in the reference manifest so B4 can refuse on
// rule drift (plan §4.3).
const RulesetVersion = "hmr-norm-v1"

// Anchor is the resolved, cross-checked anchor tuple the rules run under.
type Anchor struct {
	Network            string
	Shard              uint32
	TargetHeight       uint64
	TargetHash         common.Hash
	TargetRoot         common.Hash
	Epoch              uint64 // 3002 on mainnet
	EpochFirst         uint64
	EpochLast          uint64
	SnapshotBase       uint64 // EpochLastBlock(epoch-1) - 1 = 92,700,670
	BoundaryHeight     uint64 // EpochLastBlock(epoch-1)     = 92,700,671 (carries ss<epoch>)
	AbandonedChildHash common.Hash
	AuditEndHeight     uint64
	ConfigSHA256Hex    string
}

// RawKV is the strict read-only surface Normalize consumes: exact-key gets
// plus ordered iteration. It is intentionally write-free.
type RawKV interface {
	ethdb.KeyValueReader
	ethdb.Iteratee
}

// HistoricalStateOpener resolves best-effort historical state at a
// pre-target height for snapshot content reconstruction (plan §4.4). A nil
// state with a nil error means "root unavailable" (structural-only).
type HistoricalStateOpener interface {
	StateAt(height uint64) (*state.DB, error)
}

// HeaderReader reads a canonical pre-target header (the boundary header
// whose ShardState field the ss<epoch> record must byte-equal).
type HeaderReader interface {
	HeaderByNumber(height uint64) (*block.Header, error)
}

// Sources bundles the inputs (plan §4.3).
type Sources struct {
	Raw     RawKV
	Target  *state.DB
	Hist    HistoricalStateOpener // may be nil: pure structural-only mode
	Headers HeaderReader

	// Ctx, when non-nil, makes every long raw iteration observe
	// cancellation (the mainnet blk-rwd walk alone visits ~92.7M keys —
	// SIGINT must not be ignored for hours). Cancellation surfaces as an
	// error wrapping context.Canceled; callers map it to exit 130/16.
	Ctx context.Context
}

// Record is one raw key/value pair of the normalized set.
type Record struct {
	Key   []byte
	Value []byte
}

// NormalizedSet is the logical .hmr content (plan §4.5 sections). Stats
// are never included; target wrapper/state blobs are never included.
type NormalizedSet struct {
	ValidatorList     Record   // 1 record: RLP of the normalized ordered list
	DVL               []Record // every corrected non-empty record, key order
	Snapshots         []Record // retained target-epoch snapshots, original validated bytes, key order
	ShardState        Record   // ss<epoch>, exact bytes
	RewardAccumulator Record   // blk-rwd-<target> (§8 Q5)
}

// SectionNames returns the .hmr section names in canonical order for the
// given target epoch (on mainnet: validator-snapshot-3002 etc., plan §4.5).
func SectionNames(epoch uint64) []string {
	return []string{
		"validator-list",
		"dvl",
		sectionSnapshots(epoch),
		sectionShardState(epoch),
		"reward-accumulator",
	}
}

// DeletionPlan phase names — exactly B4's journal phases (handoff B4; no
// stats phase, §8 Q4).
const (
	PhaseDVL     = "DVL_SANITIZING"
	PhaseSnap    = "SNAPSHOT_SANITIZING"
	PhaseEpoch   = "EPOCH_SANITIZING"
	PhaseCleanup = "LOOKUP_AND_CANONICAL_CLEANUP"
)

// PlannedDeletion is one raw key to delete with its reason code.
type PlannedDeletion struct {
	Key    string `json:"key"` // hex
	Reason string `json:"reason"`
}

// PlannedRewrite is one raw key whose value is replaced; only the hash of
// the new value is carried (apply re-derives, the plan authenticates).
type PlannedRewrite struct {
	Key            string `json:"key"` // hex
	NewValueSHA256 string `json:"new_value_sha256"`
	Reason         string `json:"reason"`
}

// Placeholder marks audit-input-required cleanup (shard-1 subsets and the
// derived pointer) and B4-owned canonical cleanup — never computed here.
type Placeholder struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

// Phase groups the plan by B4 journal phase.
type Phase struct {
	Name         string            `json:"name"`
	Deletions    []PlannedDeletion `json:"deletions,omitempty"`
	Rewrites     []PlannedRewrite  `json:"rewrites,omitempty"`
	Placeholders []Placeholder     `json:"placeholders,omitempty"`
}

// DeletionPlan is the per-phase raw-key mutation plan. The validator-list
// rewrite is carried in DVL_SANITIZING (the validator/delegator metadata
// correction family); B4 physically applies the corrected list in its
// final synchronous batch per handoff B4 — the plan is the logical
// artifact, the journal owns physical sequencing.
type DeletionPlan struct {
	Phases []Phase `json:"phases"`
}

// Deletions flattens all planned deletions in phase order.
func (p *DeletionPlan) Deletions() []PlannedDeletion {
	var out []PlannedDeletion
	for _, ph := range p.Phases {
		out = append(out, ph.Deletions...)
	}
	return out
}

// Rewrites flattens all planned rewrites in phase order.
func (p *DeletionPlan) Rewrites() []PlannedRewrite {
	var out []PlannedRewrite
	for _, ph := range p.Phases {
		out = append(out, ph.Rewrites...)
	}
	return out
}

// AbsenceAssertion is the post-apply end-state contract (plan §4.5):
// PlannedDeletions is report-only evidence (source-specific); the
// reference manifest embeds only namespace + predicate +
// expected_remaining.
type AbsenceAssertion struct {
	Namespace         string `json:"namespace"`
	Predicate         string `json:"predicate"`
	PlannedDeletions  uint64 `json:"planned_deletions"` // stripped from the reference
	ExpectedRemaining uint64 `json:"expected_remaining"`
}

// SectionCounts are per-section record counters.
type SectionCounts struct {
	Retained  uint64 `json:"retained"`
	Removed   uint64 `json:"removed"`
	Invalid   uint64 `json:"invalid"`
	Missing   uint64 `json:"missing"`
	Duplicate uint64 `json:"duplicate"`
}

// Counts is the fixed-shape per-section counter block (no map iteration).
type Counts struct {
	ValidatorList SectionCounts `json:"validator_list"`
	DVL           SectionCounts `json:"dvl"`
	Snapshots     SectionCounts `json:"validator_snapshot_target_epoch"`
	ShardState    SectionCounts `json:"shard_state"`
	EpochAux      SectionCounts `json:"epoch_aux"`
	RewardAccum   SectionCounts `json:"reward_accumulator"`
	Pending       SectionCounts `json:"pending_queues"`
}

// SnapshotCoverage reports reconstructed-vs-structural verification (plan
// §4.4): structural-only cannot detect corruption that stays well-formed
// RLP with the right embedded address.
type SnapshotCoverage struct {
	ReconstructedVerified uint64 `json:"reconstructed_verified"`
	StructuralOnly        uint64 `json:"structural_only"`
}

// DigestSet holds the pre-registered digests (plan §4.5). Package digest
// is added at export time (it hashes the .hmr file bytes).
type DigestSet struct {
	Sections    []SectionDigest `json:"sections"` // canonical section order
	WrapperSet  string          `json:"wrapper_set"`
	Diagnostics string          `json:"diagnostics"`
}

// SectionDigest is one named section digest with its record count.
type SectionDigest struct {
	Name        string `json:"name"`
	RecordCount uint64 `json:"record_count"`
	SHA256      string `json:"sha256"`
}

// KeyDigestInventory is a bounded inventory row: count + digest over the
// record frames of a namespace subset.
type KeyDigestInventory struct {
	Count     uint64 `json:"count"`
	FrameSHA  string `json:"frame_sha256,omitempty"`
	MinNumber uint64 `json:"min_number,omitempty"`
	MaxNumber uint64 `json:"max_number,omitempty"`
}

// ShardInventory is the per-shard crosslink/spent freeze input (plan §4.4:
// the post-target subsets to delete come from the audit).
type ShardInventory struct {
	ShardID          uint32             `json:"shard_id"`
	CrossLinks       KeyDigestInventory `json:"crosslinks"`
	PointerPresent   bool               `json:"pointer_present"`
	PointerValueSHA  string             `json:"pointer_value_sha256,omitempty"`
	PointerBlockNum  uint64             `json:"pointer_block_num,omitempty"`
	CXReceiptsSpent  KeyDigestInventory `json:"cx_receipts_spent"`
}

// SyncEraKey is one observed sync-era/legacy key with its value digest
// (reported for B4's cleanup; never in the .hmr or the deletion plan).
type SyncEraKey struct {
	Key      string `json:"key"`
	ValueSHA string `json:"value_sha256"`
}

// Inventory is the informational (non-normative) survey.
type Inventory struct {
	Stats               KeyDigestInventory `json:"validator_stats"` // kept untouched (§8 Q4)
	SnapshotsPriorEpoch KeyDigestInventory `json:"snapshots_prior_epochs"`
	ShardStatesPrior    KeyDigestInventory `json:"shard_states_prior_epochs"`
	RewardAccumPrior    KeyDigestInventory `json:"reward_accum_prior"`
	SyncEra             []SyncEraKey       `json:"sync_era_keys"`
	LeaderContinuous    *SyncEraKey        `json:"leader_continuous,omitempty"`
	Shards              []ShardInventory   `json:"shards"`
	DVLReverseMapBytes  uint64             `json:"dvl_reverse_map_bytes"` // memory-footprint report (plan §4.4)
}

// RemovedDVLEntry is one filtered-out (post-target) delegation index tuple;
// the audit binds these bidirectionally to metadata-producing delegations
// (plan §4.6 output 5).
type RemovedDVLEntry struct {
	Delegator common.Address `json:"delegator"`
	Validator common.Address `json:"validator"`
	Index     uint64         `json:"index"`
	BlockNum  uint64         `json:"block_num"`
}

// Result is the full normalization output (plan §4.3).
type Result struct {
	Normalized *NormalizedSet
	Deletions  *DeletionPlan
	Findings   []report.Finding
	Digests    DigestSet
	Assertions []AbsenceAssertion
	Coverage   SnapshotCoverage
	Counts     Counts
	Inventory  Inventory
	// NormalizedListLength is printed prominently by scan for the manual,
	// informational preflight comparison (§8 Q2 — no receipt coupling).
	NormalizedListLength int

	// Audit inputs (plan §4.6 output 5): the removed entities the branch
	// re-execution must account for bidirectionally.
	RemovedValidators []common.Address
	RemovedDVLEntries []RemovedDVLEntry
}

// ExitCode resolves the §4.5 exit implied by the findings.
func (r *Result) ExitCode() int { return report.ExitForFindings(r.Findings) }

// HasFatalOrMissing reports whether export must refuse (plan WS5: any
// Fatal or MissingRequired finding refuses export; ReviewItems allowed).
func (r *Result) HasFatalOrMissing() bool {
	for _, f := range r.Findings {
		if f.Severity == report.SeverityFatal {
			return true
		}
	}
	return false
}

func epochBig(e uint64) *big.Int { return new(big.Int).SetUint64(e) }
