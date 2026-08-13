package hmr

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// ManifestSchema identifies the reference-manifest document.
const ManifestSchema = "hmr-reference-v1"

// AnchorTuple is the chain-invariant anchor embedded in the manifest.
type AnchorTuple struct {
	TargetHeight       uint64 `json:"target_height"`
	TargetHash         string `json:"target_hash"`
	TargetRoot         string `json:"target_root"`
	Epoch              uint64 `json:"epoch"`
	EpochFirstBlock    uint64 `json:"epoch_first_block"`
	EpochLastBlock     uint64 `json:"epoch_last_block"`
	SnapshotBaseHeight uint64 `json:"snapshot_base_height"`
	AbandonedChildHash string `json:"abandoned_child_hash"`
}

// ManifestAssertion is the reference form of an absence assertion: only
// namespace + predicate + expected_remaining (planned-deletion counts are
// source-specific run evidence and are excluded, plan §4.5).
type ManifestAssertion struct {
	Namespace         string `json:"namespace"`
	Predicate         string `json:"predicate"`
	ExpectedRemaining uint64 `json:"expected_remaining"`
}

// ManifestSection mirrors a section digest.
type ManifestSection struct {
	Name        string `json:"name"`
	RecordCount uint64 `json:"record_count"`
	SHA256      string `json:"sha256"`
}

// Manifest is the canonical reference manifest
// (metadata-<target>.reference.json): strictly canonical JSON, only
// chain-invariant, timestamp-free material. Its SHA-256 is the reference
// digest the release notes publish and B4's pre-mutation check, the
// COMPLETE marker, and the D1 startup gate bind.
type Manifest struct {
	Schema           string              `json:"schema"`
	Network          string              `json:"network"`
	Shard            uint32              `json:"shard"`
	Anchor           AnchorTuple         `json:"anchor"`
	AnchorConfigSHA  string              `json:"anchor_config_sha256"`
	RulesetVersion   string              `json:"ruleset_version"`
	PackageSHA256    string              `json:"package_sha256"` // SHA-256 of the .hmr file bytes
	RecordCount      uint64              `json:"record_count"`
	Sections         []ManifestSection   `json:"sections"`
	WrapperSetSHA256 string              `json:"wrapper_set_sha256"`
	DiagnosticsSHA   string              `json:"diagnostics_sha256"`
	Assertions       []ManifestAssertion `json:"absence_assertions"`
}

// BuildManifest assembles the manifest from a normalization result and the
// encoded package bytes.
func BuildManifest(a norm.Anchor, res *norm.Result, packageBytes []byte) *Manifest {
	sections := make([]ManifestSection, 0, len(res.Digests.Sections))
	var recordCount uint64
	for _, s := range res.Digests.Sections {
		sections = append(sections, ManifestSection{Name: s.Name, RecordCount: s.RecordCount, SHA256: s.SHA256})
		recordCount += s.RecordCount
	}
	assertions := make([]ManifestAssertion, 0, len(res.Assertions))
	for _, as := range res.Assertions {
		assertions = append(assertions, ManifestAssertion{
			Namespace: as.Namespace, Predicate: as.Predicate, ExpectedRemaining: as.ExpectedRemaining,
		})
	}
	// Assertions stay order-stable (assembly order is fixed in norm).
	return &Manifest{
		Schema:  ManifestSchema,
		Network: a.Network,
		Shard:   a.Shard,
		Anchor: AnchorTuple{
			TargetHeight:       a.TargetHeight,
			TargetHash:         a.TargetHash.Hex(),
			TargetRoot:         a.TargetRoot.Hex(),
			Epoch:              a.Epoch,
			EpochFirstBlock:    a.EpochFirst,
			EpochLastBlock:     a.EpochLast,
			SnapshotBaseHeight: a.SnapshotBase,
			AbandonedChildHash: a.AbandonedChildHash.Hex(),
		},
		AnchorConfigSHA:  a.ConfigSHA256Hex,
		RulesetVersion:   norm.RulesetVersion,
		PackageSHA256:    report.SHA256Hex(packageBytes),
		RecordCount:      recordCount,
		Sections:         sections,
		WrapperSetSHA256: res.Digests.WrapperSet,
		DiagnosticsSHA:   res.Digests.Diagnostics,
		Assertions:       assertions,
	}
}

// EncodeManifest renders the strictly canonical JSON bytes whose SHA-256 is
// the reference digest.
func EncodeManifest(m *Manifest) ([]byte, error) {
	return report.CanonicalJSON(m)
}

// DecodeManifest strictly parses manifest bytes: unknown fields rejected,
// every required field validated (Validate), and the bytes must be the
// exact canonical re-encoding (which also rejects duplicate keys,
// reordered keys and whitespace variants — the manifest's SHA-256 is the
// reference digest, so only the one canonical byte form is a manifest).
func DecodeManifest(data []byte) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("hmr: strict manifest parse: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("hmr: trailing data after manifest document")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	reenc, err := EncodeManifest(&m)
	if err != nil {
		return nil, fmt.Errorf("hmr: manifest re-encode: %w", err)
	}
	if !bytes.Equal(reenc, data) {
		return nil, fmt.Errorf("hmr: manifest bytes are not the canonical encoding (duplicate keys, ordering or formatting differ)")
	}
	return &m, nil
}

// Validate checks the complete manifest contract: schema, required
// fields, digest shapes, the exact canonical section list for the target
// epoch (names, order, count), the record-count sum, and assertions.
func (m *Manifest) Validate() error {
	if m.Schema != ManifestSchema {
		return fmt.Errorf("hmr: manifest schema %q, want %q", m.Schema, ManifestSchema)
	}
	if m.Network == "" {
		return fmt.Errorf("hmr: manifest network is empty")
	}
	if m.RulesetVersion != norm.RulesetVersion {
		return fmt.Errorf("hmr: manifest ruleset %q, want %q", m.RulesetVersion, norm.RulesetVersion)
	}
	for name, v := range map[string]string{
		"anchor_config_sha256": m.AnchorConfigSHA,
		"package_sha256":       m.PackageSHA256,
		"wrapper_set_sha256":   m.WrapperSetSHA256,
		"diagnostics_sha256":   m.DiagnosticsSHA,
	} {
		if !isHexDigest(v, 64) {
			return fmt.Errorf("hmr: manifest %s %q is not a 64-char lowercase hex SHA-256", name, v)
		}
	}
	for name, v := range map[string]string{
		"target_hash":          m.Anchor.TargetHash,
		"target_root":          m.Anchor.TargetRoot,
		"abandoned_child_hash": m.Anchor.AbandonedChildHash,
	} {
		if len(v) != 66 || v[:2] != "0x" || !isHexDigest(v[2:], 64) {
			return fmt.Errorf("hmr: manifest anchor %s %q is not a 0x-prefixed 32-byte hash", name, v)
		}
	}
	want := norm.SectionNames(m.Anchor.Epoch)
	if len(m.Sections) != len(want) {
		return fmt.Errorf("hmr: manifest has %d sections, want exactly %d", len(m.Sections), len(want))
	}
	// Fixed section cardinalities (plan §4.5): validator-list, shard-state
	// and reward-accumulator are exactly one record each; the snapshot
	// section must carry at least the target-epoch snapshot; dvl is the
	// only variable-and-possibly-empty section. SectionNames order is
	// [validator-list, dvl, <snapshots>, <shard-state>, reward-accumulator].
	exactlyOne := map[int]bool{0: true, 3: true, 4: true}
	var sum uint64
	for i, s := range m.Sections {
		if s.Name != want[i] {
			return fmt.Errorf("hmr: manifest section %d is %q, want %q (canonical order)", i, s.Name, want[i])
		}
		if !isHexDigest(s.SHA256, 64) {
			return fmt.Errorf("hmr: manifest section %q sha256 %q is not a 64-char lowercase hex SHA-256", s.Name, s.SHA256)
		}
		if exactlyOne[i] && s.RecordCount != 1 {
			return fmt.Errorf("hmr: manifest section %q record_count %d, want exactly 1", s.Name, s.RecordCount)
		}
		if i == 2 && s.RecordCount == 0 {
			return fmt.Errorf("hmr: manifest snapshot section %q has zero records (the target-epoch snapshot is mandatory)", s.Name)
		}
		sum += s.RecordCount
	}
	if m.RecordCount != sum {
		return fmt.Errorf("hmr: manifest record_count %d != section sum %d", m.RecordCount, sum)
	}
	// Absence assertions must be exactly the canonical ordered set for the
	// anchor's (epoch, target height), each with expected_remaining == 0.
	// This rejects reordering, duplicates, extra/missing assertions and any
	// nonzero end-state count.
	specs := norm.CanonicalAssertionSpecs(m.Anchor.Epoch, m.Anchor.TargetHeight)
	if len(m.Assertions) != len(specs) {
		return fmt.Errorf("hmr: manifest has %d absence assertions, want exactly %d (canonical set)", len(m.Assertions), len(specs))
	}
	for i, a := range m.Assertions {
		if a.Namespace != specs[i].Namespace || a.Predicate != specs[i].Predicate {
			return fmt.Errorf("hmr: manifest assertion %d is {%q,%q}, want {%q,%q} (canonical set/order)",
				i, a.Namespace, a.Predicate, specs[i].Namespace, specs[i].Predicate)
		}
		if a.ExpectedRemaining != 0 {
			return fmt.Errorf("hmr: manifest assertion %d (%s) expected_remaining=%d, must be 0 (post-apply end state)", i, a.Namespace, a.ExpectedRemaining)
		}
	}
	return nil
}

func isHexDigest(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
