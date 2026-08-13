package verify

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
)

// MetadataReferenceInternalNone is the defined sentinel for internal builds
// run without the optional in-place metadata-reference manifest (plan
// §2.2.4, revision 11).
const MetadataReferenceInternalNone = "internal:none"

// MarkerSchemaV1 is this plan's documented recovery-marker schema (an
// integration item for the in-place agents if/when they adopt the tool —
// plan §8).
const MarkerSchemaV1 = "hmy-recovery-marker-v1"

// Marker is the recovery-completion marker written by compact-db under
// keys.RecoveryMarkerKey after all four heads (plan §2.2.4, WS5 step 4):
// (a) anchor-manifest digest; (b) metadata-reference digest or the
// internal:none sentinel; (c) producing-tool version + binary SHA-256;
// (d) the normalized-output digest and the marker-excluded artifact logical
// KV digest. Deterministic (no timestamps) so two identical builds produce
// identical markers.
type Marker struct {
	SchemaVersion string `json:"schema_version"`
	Network       string `json:"network"`
	ShardID       uint32 `json:"shard_id"`
	TargetHeight  uint64 `json:"target_height"`
	TargetHash    string `json:"target_hash"`

	AnchorManifestSHA256    string `json:"anchor_manifest_sha256"`
	MetadataReferenceDigest string `json:"metadata_reference_digest"` // or "internal:none"
	ToolVersion             string `json:"tool_version"`
	ToolBinarySHA256        string `json:"tool_binary_sha256"`
	NormalizedOutputDigest  string `json:"normalized_output_digest"`
	LogicalKVDigest         string `json:"logical_kv_digest"` // marker-excluded by definition
}

// Encode serializes the marker canonically.
func (m *Marker) Encode() ([]byte, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("verify: marshal recovery marker: %w", err)
	}
	return raw, nil
}

// ReadMarker loads and strictly decodes the recovery marker from a database.
func ReadMarker(db ethdb.KeyValueReader) (*Marker, error) {
	ok, err := db.Has(keys.RecoveryMarkerKey)
	if err != nil {
		return nil, fmt.Errorf("verify: probe recovery marker: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("verify: recovery-completion marker is absent")
	}
	raw, err := db.Get(keys.RecoveryMarkerKey)
	if err != nil {
		return nil, fmt.Errorf("verify: read recovery marker: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Marker
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("verify: decode recovery marker: %w", err)
	}
	if m.SchemaVersion != MarkerSchemaV1 {
		return nil, fmt.Errorf("verify: unsupported recovery-marker schema %q", m.SchemaVersion)
	}
	return &m, nil
}
