// Package anchor loads and cross-verifies the plain-JSON anchor config
// (recovery-anchor.json, plan §4.7) that drives every metadata subcommand.
// There is no --network flag on the metadata commands: network, shard and
// the frozen incident constants come from this file, are cross-checked
// against the network schedule (via the preflight-owned
// internal/recovery/inplace/anchor.Resolve, which also installs the
// process-global shard.Schedule) and against the source DB at run time.
// A typo'd config cannot survive first contact with the DB.
package anchor

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	inplaceanchor "github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
)

// Schema is the anchor config schema identifier.
const Schema = "recovery-anchor-v1"

// Config is the strict-parsed anchor file (unknown fields rejected).
// All §1 frozen numbers live here; everything is re-verified at run time
// against the schedule and the DB.
type Config struct {
	Schema             string   `json:"schema"`
	Network            string   `json:"network"`
	Shard              uint32   `json:"shard"`
	TargetHeight       uint64   `json:"target_height"`
	TargetHash         string   `json:"target_hash"`
	AbandonedChildHash string   `json:"abandoned_child_hash"`
	Epoch              uint64   `json:"epoch"`
	EpochFirstBlock    uint64   `json:"epoch_first_block"`
	EpochLastBlock     uint64   `json:"epoch_last_block"`
	SnapshotBaseHeight uint64   `json:"snapshot_base_height"`
	AuditEndHeight     uint64   `json:"audit_end_height"`
	KnownBadBlocks     []uint64 `json:"known_bad_blocks"`
}

// Resolved is the run-time anchor: the parsed config, its exact file hash
// (bound into the .hmr header and the reference manifest), and the
// schedule-derived inplace anchor (whose Resolve installed the
// process-global shard.Schedule).
type Resolved struct {
	Config     Config
	ConfigSHA  [32]byte
	FileBytes  []byte
	Inplace    *inplaceanchor.Anchor
	TargetHash common.Hash
	ChildHash  common.Hash
}

// ConfigSHAHex returns the anchor file hash in hex.
func (r *Resolved) ConfigSHAHex() string { return fmt.Sprintf("%x", r.ConfigSHA[:]) }

// Load strictly parses the anchor config file and records its SHA-256.
func Load(path string) (*Config, []byte, [32]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, [32]byte{}, fmt.Errorf("anchor: read config: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, nil, [32]byte{}, fmt.Errorf("anchor: strict parse %s: %w", path, err)
	}
	// Exactly one JSON document.
	if dec.More() {
		return nil, nil, [32]byte{}, fmt.Errorf("anchor: trailing data after config document in %s", path)
	}
	return &cfg, raw, sha256.Sum256(raw), nil
}

// Resolve validates the config, installs the schedule via the
// preflight-owned inplace anchor (read-only import; its files are never
// edited) and cross-checks every derivable identity (plan §4.1/§4.7):
//
//   - mainnet: Resolve refuses overrides, then the JSON constants must
//     equal the compiled anchor.MainnetTargetHeight/MainnetTargetHash
//     (runtime drift check);
//   - non-mainnet fixture networks: the JSON target height/hash are passed
//     as the test-only overrides Resolve requires;
//   - schedule identities: epoch, epoch first/last blocks, snapshot base
//     height (= EpochLastBlock(epoch-1) - 1);
//   - audit range sanity: audit_end_height > target_height.
func Resolve(path string) (*Resolved, error) {
	cfg, raw, sum, err := Load(path)
	if err != nil {
		return nil, err
	}
	if cfg.Schema != Schema {
		return nil, fmt.Errorf("anchor: schema %q, want %q", cfg.Schema, Schema)
	}
	if len(common.FromHex(cfg.TargetHash)) != common.HashLength {
		return nil, fmt.Errorf("anchor: target_hash %q is not a 32-byte hex hash", cfg.TargetHash)
	}
	if len(common.FromHex(cfg.AbandonedChildHash)) != common.HashLength {
		return nil, fmt.Errorf("anchor: abandoned_child_hash %q is not a 32-byte hex hash", cfg.AbandonedChildHash)
	}

	var ov inplaceanchor.Overrides
	if nodeconfig.NetworkType(cfg.Network) != nodeconfig.Mainnet {
		ov = inplaceanchor.Overrides{TargetHeight: cfg.TargetHeight, TargetHash: cfg.TargetHash}
	}
	ip, err := inplaceanchor.Resolve(cfg.Network, cfg.Shard, ov)
	if err != nil {
		return nil, fmt.Errorf("anchor: schedule resolve: %w", err)
	}
	if nodeconfig.NetworkType(cfg.Network) == nodeconfig.Mainnet {
		// Compiled-constant drift check: the shipped JSON must match the
		// binary's compiled anchor exactly.
		if cfg.TargetHeight != inplaceanchor.MainnetTargetHeight {
			return nil, fmt.Errorf("anchor: config target_height %d differs from the compiled mainnet anchor %d",
				cfg.TargetHeight, inplaceanchor.MainnetTargetHeight)
		}
		if common.HexToHash(cfg.TargetHash) != inplaceanchor.MainnetTargetHash {
			return nil, fmt.Errorf("anchor: config target_hash %s differs from the compiled mainnet anchor %s",
				cfg.TargetHash, inplaceanchor.MainnetTargetHashHex)
		}
	}

	// Schedule cross-checks (a wrong epoch geometry cannot pass).
	if ip.Epoch.Uint64() != cfg.Epoch {
		return nil, fmt.Errorf("anchor: config epoch %d, schedule says CalcEpochNumber(%d) = %s",
			cfg.Epoch, cfg.TargetHeight, ip.Epoch)
	}
	if first := ip.BoundaryHeight + 1; first != cfg.EpochFirstBlock {
		return nil, fmt.Errorf("anchor: config epoch_first_block %d, schedule says %d", cfg.EpochFirstBlock, first)
	}
	if last := ip.Schedule.EpochLastBlock(cfg.Epoch); last != cfg.EpochLastBlock {
		return nil, fmt.Errorf("anchor: config epoch_last_block %d, schedule says %d", cfg.EpochLastBlock, last)
	}
	if base := ip.BoundaryHeight - 1; base != cfg.SnapshotBaseHeight {
		return nil, fmt.Errorf("anchor: config snapshot_base_height %d, schedule says EpochLastBlock(%d)-1 = %d",
			cfg.SnapshotBaseHeight, cfg.Epoch-1, base)
	}
	if cfg.AuditEndHeight <= cfg.TargetHeight {
		return nil, fmt.Errorf("anchor: audit_end_height %d must exceed target_height %d",
			cfg.AuditEndHeight, cfg.TargetHeight)
	}

	return &Resolved{
		Config:     *cfg,
		ConfigSHA:  sum,
		FileBytes:  raw,
		Inplace:    ip,
		TargetHash: common.HexToHash(cfg.TargetHash),
		ChildHash:  common.HexToHash(cfg.AbandonedChildHash),
	}, nil
}

// Getter is the minimal raw read surface the DB cross-check needs.
type Getter interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
}

// VerifyDB cross-checks the anchor against the opened source DB: the
// canonical hash at the target height must equal the anchored hash and the
// target header must exist (plan §4.7). It returns the raw target header
// bytes for the caller to decode.
func VerifyDB(kv Getter, r *Resolved) ([]byte, error) {
	canonical, err := kv.Get(canonicalHashKey(r.Config.TargetHeight))
	if err != nil {
		return nil, fmt.Errorf("anchor: canonical mapping for target height %d unreadable: %w", r.Config.TargetHeight, err)
	}
	if common.BytesToHash(canonical) != r.TargetHash {
		return nil, fmt.Errorf("anchor: canonical hash at height %d is %x, config says %s — refusing (wrong DB or wrong config)",
			r.Config.TargetHeight, canonical, r.Config.TargetHash)
	}
	hdr, err := kv.Get(headerKey(r.Config.TargetHeight, r.TargetHash))
	if err != nil {
		return nil, fmt.Errorf("anchor: target header %d %s unreadable: %w", r.Config.TargetHeight, r.Config.TargetHash, err)
	}
	return hdr, nil
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		enc[i] = byte(number)
		number >>= 8
	}
	return enc
}

func canonicalHashKey(number uint64) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), 'n')
}

func headerKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), hash.Bytes()...)
}
