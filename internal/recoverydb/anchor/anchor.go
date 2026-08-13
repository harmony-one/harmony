// Package anchor defines the recovery anchor manifest: the frozen incident
// values every mutating command of harmony-recovery-db validates against.
//
// The manifest is a plain JSON file with a recorded SHA-256 sidecar (no
// signing ceremony — operator decision, plan §3). The field layout mirrors
// the in-place recovery handoff §3.1 so both recovery efforts share one set
// of frozen values. Fields the in-place handoff marks FILL_AND_VERIFY
// (target state root, target header ViewID, target certificate hash,
// ss<epoch> digest) are produced by inspect-db / export-bundle runs and may
// legitimately be absent (zero) in early manifests; when present they are
// validated like every other field.
package anchor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
)

// Pinned incident values (operator-confirmed 2026-08-12, plan §1).
// The tools re-verify all of them at run time; they are never trusted blindly.
var (
	// MainnetTargetHeight is the pinned shard-0 target block height.
	MainnetTargetHeight = uint64(92730034)
	// MainnetTargetHash is the pinned shard-0 target block hash.
	MainnetTargetHash = common.HexToHash("0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d")
	// MainnetTargetParentHash is the pinned parent hash of the target block.
	MainnetTargetParentHash = common.HexToHash("0x14e2bcbb4aba7e04e13fd6fdb8427632e942403e58dcc9f0c412bb0c7a38951e")
	// MainnetAbandonedChildHeight is the first abandoned block above the target.
	MainnetAbandonedChildHeight = uint64(92730035)
	// MainnetAbandonedChildHash carries the certificate over the target block.
	MainnetAbandonedChildHash = common.HexToHash("0x5de06979a333f20afb8b245a8cf44472dc5bfc7383a57ddee48e1809bcee7c5d")
	// MainnetRejectedShard1Height is the known-bad shard-1 tuple height.
	MainnetRejectedShard1Height = uint64(94978279)
	// MainnetRejectedShard1Hash is the known-bad shard-1 block hash.
	MainnetRejectedShard1Hash = common.HexToHash("0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878")
	// MainnetPresumedBaselineHeight is the presumed Aug 8 baseline head.
	// Contingent: inspect-db must read the actual head (plan §1).
	MainnetPresumedBaselineHeight = uint64(92591097)
)

// KnownBadEntry is one entry of the anchor known-bad list: a (shard, height,
// hash) tuple that must have no record of any kind in a produced artifact.
type KnownBadEntry struct {
	ShardID uint32      `json:"shard_id"`
	Height  uint64      `json:"height"`
	Hash    common.Hash `json:"hash"`
}

// Manifest is the recovery anchor manifest. Strict JSON: unknown fields are
// rejected, decoding is all-or-nothing.
type Manifest struct {
	SchemaVersion string `json:"schema_version"` // "hmy-recovery-anchor-v1"
	Network       string `json:"network"`        // "mainnet", "localnet", ...
	ShardID       uint32 `json:"shard_id"`

	GenesisHash common.Hash `json:"genesis_hash"` // FILL_AND_VERIFY; zero = not yet filled

	TargetHeight     uint64      `json:"target_height"`
	TargetHash       common.Hash `json:"target_hash"`
	TargetParentHash common.Hash `json:"target_parent_hash"`
	TargetStateRoot  common.Hash `json:"target_state_root"` // FILL_AND_VERIFY
	TargetEpoch      uint64      `json:"target_epoch"`
	TargetViewID     uint64      `json:"target_view_id"` // FILL_AND_VERIFY; 0 = not yet filled

	// Baseline tuple: the head of the trusted pre-incident copy the replay
	// starts from. Presumed in the plan; pinned after the first inspect-db.
	BaselineHeight uint64      `json:"baseline_height"`
	BaselineHash   common.Hash `json:"baseline_hash"` // zero = not yet pinned

	AbandonedChildHeight uint64      `json:"abandoned_child_height"`
	AbandonedChildHash   common.Hash `json:"abandoned_child_hash"`

	RejectedShard1Height uint64      `json:"rejected_shard1_height"`
	RejectedShard1Hash   common.Hash `json:"rejected_shard1_hash"`

	// TargetCertificateSHA256 is the SHA-256 over the exact commit
	// sig+bitmap bytes for the target block (FILL_AND_VERIFY, produced by
	// export-bundle). Empty = not yet filled.
	TargetCertificateSHA256 string `json:"target_certificate_sha256"`

	// ShardStateDigestSHA256 is the digest of the target-epoch shard state
	// record (ss<epoch>), FILL_AND_VERIFY. Empty = not yet filled.
	ShardStateDigestSHA256 string `json:"shard_state_digest_sha256"`

	// KnownBad lists identifiers that must not appear at or below the
	// baseline (inspect gate) nor anywhere in produced artifacts.
	KnownBad []KnownBadEntry `json:"known_bad"`
}

// SchemaVersionV1 is the only schema this build accepts.
const SchemaVersionV1 = "hmy-recovery-anchor-v1"

// Load reads, strictly decodes and validates an anchor manifest from an
// absolute path. It does NOT verify the checksum sidecar; callers that
// require the checksum gate use integrity.VerifyChecksumFile first.
func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("anchor: read %s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("anchor: strict decode %s: %w", path, err)
	}
	// Reject trailing garbage after the JSON document.
	if dec.More() {
		return nil, fmt.Errorf("anchor: trailing data after JSON document in %s", path)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("anchor: validate %s: %w", path, err)
	}
	return &m, nil
}

// Validate performs the all-or-nothing structural and pinned-value checks.
func (m *Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("unsupported schema_version %q (want %q)", m.SchemaVersion, SchemaVersionV1)
	}
	if m.Network == "" {
		return fmt.Errorf("network is required")
	}
	if m.TargetHeight == 0 {
		return fmt.Errorf("target_height is required")
	}
	if m.TargetHash == (common.Hash{}) {
		return fmt.Errorf("target_hash is required")
	}
	if m.TargetParentHash == (common.Hash{}) {
		return fmt.Errorf("target_parent_hash is required")
	}
	if m.AbandonedChildHeight != m.TargetHeight+1 {
		return fmt.Errorf("abandoned_child_height %d must equal target_height+1 (%d)",
			m.AbandonedChildHeight, m.TargetHeight+1)
	}
	if m.AbandonedChildHash == (common.Hash{}) {
		return fmt.Errorf("abandoned_child_hash is required")
	}
	if m.BaselineHeight >= m.TargetHeight {
		return fmt.Errorf("baseline_height %d must be below target_height %d", m.BaselineHeight, m.TargetHeight)
	}
	// Pinned-incident cross-check: a mainnet shard-0 manifest for the pinned
	// target height must carry exactly the pinned hashes (plan §1). This is
	// the "tool re-verifies the operator-confirmed values" rule.
	if m.Network == "mainnet" && m.ShardID == 0 && m.TargetHeight == MainnetTargetHeight {
		if m.TargetHash != MainnetTargetHash {
			return fmt.Errorf("pinned target hash mismatch: manifest %s, pinned %s",
				m.TargetHash.Hex(), MainnetTargetHash.Hex())
		}
		if m.TargetParentHash != MainnetTargetParentHash {
			return fmt.Errorf("pinned target parent hash mismatch: manifest %s, pinned %s",
				m.TargetParentHash.Hex(), MainnetTargetParentHash.Hex())
		}
		if m.AbandonedChildHash != MainnetAbandonedChildHash {
			return fmt.Errorf("pinned abandoned child hash mismatch: manifest %s, pinned %s",
				m.AbandonedChildHash.Hex(), MainnetAbandonedChildHash.Hex())
		}
		if m.RejectedShard1Height != MainnetRejectedShard1Height ||
			m.RejectedShard1Hash != MainnetRejectedShard1Hash {
			return fmt.Errorf("pinned rejected shard-1 tuple mismatch: manifest (%d, %s)",
				m.RejectedShard1Height, m.RejectedShard1Hash.Hex())
		}
	}
	return nil
}

// RequireTargetHeight refuses a CLI --target-height that disagrees with the
// manifest (plan §4 "Target selection").
func (m *Manifest) RequireTargetHeight(cliHeight uint64) error {
	if cliHeight != m.TargetHeight {
		return fmt.Errorf("anchor: --target-height %d disagrees with anchor target_height %d",
			cliHeight, m.TargetHeight)
	}
	return nil
}

// Window is the canonical retention window for a target block.
type Window struct {
	RetainFrom uint64 // first retained block (inclusive)
	Target     uint64 // last retained block (inclusive) == target height
	Epoch      uint64 // epoch of the target block
}

// Blocks returns the number of retained blocks.
func (w Window) Blocks() uint64 { return w.Target - w.RetainFrom + 1 }

// ComputeWindow derives the retention window from the shard schedule:
// retainFrom = EpochLastBlock(CalcEpochNumber(target) - 1). The schedule
// comes from --network; the window is never hard-coded (plan §4).
// retainFromOverride (--retain-from-height) may only extend retention
// (i.e. lower the start); 0 means no override.
func ComputeWindow(schedule shardingconfig.Schedule, target uint64, retainFromOverride uint64) (Window, error) {
	epoch := schedule.CalcEpochNumber(target)
	if epoch.Sign() < 0 {
		return Window{}, fmt.Errorf("anchor: negative epoch for target %d", target)
	}
	var retainFrom uint64
	if epoch.Cmp(big.NewInt(0)) == 0 {
		retainFrom = 0 // genesis epoch: retain from genesis
	} else {
		prev := new(big.Int).Sub(epoch, big.NewInt(1))
		retainFrom = schedule.EpochLastBlock(prev.Uint64())
	}
	if retainFrom > target {
		return Window{}, fmt.Errorf("anchor: computed retainFrom %d above target %d", retainFrom, target)
	}
	if retainFromOverride != 0 {
		if retainFromOverride > retainFrom {
			return Window{}, fmt.Errorf(
				"anchor: --retain-from-height %d would shrink the window (schedule start %d); it may only extend retention",
				retainFromOverride, retainFrom)
		}
		retainFrom = retainFromOverride
	}
	return Window{RetainFrom: retainFrom, Target: target, Epoch: epoch.Uint64()}, nil
}

// BloomSectionSize is the chain-indexer section size (params.BloomBitsBlocks).
const BloomSectionSize = 4096

// BloomCheckpoint computes the bloom chain-indexer checkpoint for a compact
// artifact (round 13 finding 4). The stored section count must be chosen so
// the NEXT section the indexer processes needs no headers below retainFrom:
// count = retainFrom/4096 + 1, i.e. section `count` starts at or after
// retainFrom+1 and every header it needs is either retained (<= target) or
// produced after restart. The recorded section head (count*4096-1) must
// itself be a retained block or the checkpoint would reference a header the
// artifact does not carry.
//
// ok=false means no checkpoint is written: either the window starts at
// genesis (full archival below target — the indexer can regenerate every
// section from real headers), or the window is smaller than a section
// boundary span (tiny test windows only; never the mainnet window of ~29k
// blocks). Historical sections below the checkpoint are marked done without
// bloom-bits data — the same degradation stock `dumpdb` snapshots ship with
// (cmd/config/dumpdb.go indexerDataDump).
func BloomCheckpoint(w Window) (count uint64, headBlock uint64, ok bool) {
	if w.RetainFrom == 0 {
		return 0, 0, false
	}
	count = w.RetainFrom/BloomSectionSize + 1
	headBlock = count*BloomSectionSize - 1
	if headBlock > w.Target || headBlock < w.RetainFrom {
		return 0, 0, false
	}
	return count, headBlock, true
}
