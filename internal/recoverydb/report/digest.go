package report

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"reflect"
)

// DigestDomainPrefix domain-separates every digest this tool computes.
const DigestDomainPrefix = "hmy-recoverydb-digest-v1"

// Hasher accumulates a domain-separated, order-sensitive SHA-256 digest over
// a sequence of items, each item a sequence of length-prefixed chunks
// (uvarint(len) || bytes). Iteration order is the caller's contract — all
// DigestSet domains are fed in lexical key order.
type Hasher struct {
	h     hash.Hash
	count uint64
}

// NewHasher creates a Hasher for a named domain.
func NewHasher(domain string) *Hasher {
	h := sha256.New()
	h.Write([]byte(DigestDomainPrefix))
	h.Write([]byte{0})
	h.Write([]byte(domain))
	h.Write([]byte{0})
	return &Hasher{h: h}
}

// Add appends one item made of the given chunks.
func (d *Hasher) Add(chunks ...[]byte) {
	var lenbuf [binary.MaxVarintLen64]byte
	for _, c := range chunks {
		n := binary.PutUvarint(lenbuf[:], uint64(len(c)))
		d.h.Write(lenbuf[:n])
		d.h.Write(c)
	}
	d.count++
}

// Digest finalizes into a Digest record.
func (d *Hasher) Digest() Digest {
	return Digest{Count: d.count, SHA256: hex.EncodeToString(d.h.Sum(nil))}
}

// Digest is a (count, sha256) pair for one namespace.
type Digest struct {
	Count  uint64 `json:"count"`
	SHA256 string `json:"sha256"`
}

// DigestSetSchemaV1 is the only DigestSet schema this build accepts.
const DigestSetSchemaV1 = "hmy-recoverydb-digestset-v1"

// DigestWindow describes the retention window the window-scoped domains
// (reward accumulators, outgoing CX receipts) were computed over.
type DigestWindow struct {
	RetainFrom uint64 `json:"retain_from"`
	Target     uint64 `json:"target"`
}

// DigestSet is the shared contract consumed by replay-bundle, compact-db and
// verify-db (plan WS4 step 9): versioned, domain-separated counts + SHA-256
// digests for state and off-chain namespaces. Validator stats are
// intentionally NOT part of the set (omitted downstream). Pending queues are
// intentionally NOT part of the set (cleared/omitted downstream).
type DigestSet struct {
	SchemaVersion string       `json:"schema_version"`
	Network       string       `json:"network"`
	ShardID       uint32       `json:"shard_id"`
	TargetHeight  uint64       `json:"target_height"`
	TargetHash    string       `json:"target_hash"`
	StateRoot     string       `json:"state_root"`
	Window        DigestWindow `json:"window"`

	// State half (from the purpose-built traversal, plan WS2/§11.2).
	Accounts     Digest `json:"accounts"`
	StorageSlots Digest `json:"storage_slots"`
	Codes        Digest `json:"codes"` // all three locations, location-tagged

	// Off-chain half (raw prefix scans in lexical key order).
	CXSpent            Digest `json:"cx_spent"`
	CXOutgoingWindow   Digest `json:"cx_outgoing_window"`
	CrosslinkIndex     Digest `json:"crosslink_index"`
	CrosslinkShardLast Digest `json:"crosslink_shard_last"`
	ValidatorList      Digest `json:"validator_list"`
	Delegations        Digest `json:"delegations"`
	ValidatorSnapshots Digest `json:"validator_snapshots"`
	ShardStates        Digest `json:"shard_states"`
	EpochBlockNumbers  Digest `json:"epoch_block_numbers"`
	EpochVrf           Digest `json:"epoch_vrf"`
	EpochVdf           Digest `json:"epoch_vdf"`
	RewardAccumulators Digest `json:"reward_accumulators"`
}

// digestFields enumerates the digest-typed fields by JSON name, used for
// validation and comparison so a new field cannot be silently skipped.
func (s *DigestSet) digestFields() map[string]Digest {
	out := map[string]Digest{}
	v := reflect.ValueOf(*s)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type == reflect.TypeOf(Digest{}) {
			out[t.Field(i).Tag.Get("json")] = v.Field(i).Interface().(Digest)
		}
	}
	return out
}

// Validate checks the schema version and that every digest field is
// populated. A missing DigestSet field is a hard failure (plan §11.3).
func (s *DigestSet) Validate() error {
	if s.SchemaVersion != DigestSetSchemaV1 {
		return fmt.Errorf("report: unsupported DigestSet schema %q (want %q)", s.SchemaVersion, DigestSetSchemaV1)
	}
	for name, d := range s.digestFields() {
		if len(d.SHA256) != 64 {
			return fmt.Errorf("report: DigestSet field %q missing or malformed (sha256 %q)", name, d.SHA256)
		}
	}
	return nil
}

// Diff compares two digest sets field by field and returns human-readable
// per-field differences (empty = equal). Window and identity fields are
// compared too: sets computed over different windows are never equal.
func (s *DigestSet) Diff(o *DigestSet) []string {
	var diffs []string
	if s.SchemaVersion != o.SchemaVersion {
		diffs = append(diffs, fmt.Sprintf("schema_version: %q vs %q", s.SchemaVersion, o.SchemaVersion))
	}
	if s.Network != o.Network {
		diffs = append(diffs, fmt.Sprintf("network: %q vs %q", s.Network, o.Network))
	}
	if s.ShardID != o.ShardID {
		diffs = append(diffs, fmt.Sprintf("shard_id: %d vs %d", s.ShardID, o.ShardID))
	}
	if s.TargetHeight != o.TargetHeight {
		diffs = append(diffs, fmt.Sprintf("target_height: %d vs %d", s.TargetHeight, o.TargetHeight))
	}
	if s.TargetHash != o.TargetHash {
		diffs = append(diffs, fmt.Sprintf("target_hash: %s vs %s", s.TargetHash, o.TargetHash))
	}
	if s.StateRoot != o.StateRoot {
		diffs = append(diffs, fmt.Sprintf("state_root: %s vs %s", s.StateRoot, o.StateRoot))
	}
	if s.Window != o.Window {
		diffs = append(diffs, fmt.Sprintf("window: %+v vs %+v", s.Window, o.Window))
	}
	a, b := s.digestFields(), o.digestFields()
	for _, name := range sortedKeys(a) {
		if a[name] != b[name] {
			diffs = append(diffs, fmt.Sprintf("%s: {count:%d sha256:%s} vs {count:%d sha256:%s}",
				name, a[name].Count, a[name].SHA256, b[name].Count, b[name].SHA256))
		}
	}
	return diffs
}

func sortedKeys(m map[string]Digest) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
