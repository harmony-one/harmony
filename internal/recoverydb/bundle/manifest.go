package bundle

import (
	"fmt"
	"path/filepath"

	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

// ManifestSchemaV1 is the bundle manifest schema.
const ManifestSchemaV1 = "hmy-recovery-bundle-v1"

// ChunkInfo describes one chunk file.
type ChunkInfo struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	Records     uint64 `json:"records"`
	FirstHeight uint64 `json:"first_height"`
	LastHeight  uint64 `json:"last_height"`
	Bytes       uint64 `json:"bytes"`
}

// SidecarInfo describes the abandoned-child certificate sidecar.
type SidecarInfo struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	ChildHeight uint64 `json:"child_height"`
	ChildHash   string `json:"child_hash"`
	ParentHash  string `json:"parent_hash"` // must equal the pinned target hash
	SigSHA256   string `json:"sig_sha256"`  // SHA-256 over the extracted sig+bitmap bytes
}

// Manifest is manifest.json.
type Manifest struct {
	report.Meta

	BaselineHeight uint64 `json:"baseline_height"`
	BaselineHash   string `json:"baseline_hash"`
	FromHeight     uint64 `json:"from_height"`
	ToHeight       uint64 `json:"to_height"`
	TargetHash     string `json:"target_hash"`

	RecordCount       uint64      `json:"record_count"`
	OrderedHashDigest string      `json:"ordered_hash_digest"`
	Chunks            []ChunkInfo `json:"chunks"`
	Sidecar           SidecarInfo `json:"sidecar"`

	Donor string `json:"donor"`
}

// ManifestPath / SidecarPath / SumsPath fix the bundle layout.
func ManifestPath(dir string) string { return filepath.Join(dir, "manifest.json") }

// SidecarPath is the raw RLP header of the certificate child.
func SidecarPath(dir string) string { return filepath.Join(dir, "target-cert-header.rlp") }

// SumsPath is the directory-level SHA256SUMS.
func SumsPath(dir string) string { return filepath.Join(dir, "SHA256SUMS") }

// ChunkName formats the nth chunk file name.
func ChunkName(n int) string { return fmt.Sprintf("bundle-%06d.rec", n) }

// LoadManifest reads and validates manifest.json (with its .sha256 sidecar).
func LoadManifest(dir string) (*Manifest, string, error) {
	path := ManifestPath(dir)
	sum, err := integrity.VerifyChecksumFile(path)
	if err != nil {
		return nil, "", err
	}
	var m Manifest
	if err := report.ReadJSONStrict(path, &m); err != nil {
		return nil, "", err
	}
	if m.SchemaVersion != ManifestSchemaV1 {
		return nil, "", fmt.Errorf("bundle: unsupported manifest schema %q", m.SchemaVersion)
	}
	if m.FromHeight > m.ToHeight {
		return nil, "", fmt.Errorf("bundle: manifest range [%d,%d] inverted", m.FromHeight, m.ToHeight)
	}
	want := m.ToHeight - m.FromHeight + 1
	if m.RecordCount != want {
		return nil, "", fmt.Errorf("bundle: manifest record count %d != range size %d", m.RecordCount, want)
	}
	var total uint64
	for _, c := range m.Chunks {
		total += c.Records
	}
	if total != m.RecordCount {
		return nil, "", fmt.Errorf("bundle: chunk record counts sum to %d, manifest says %d", total, m.RecordCount)
	}
	return &m, sum, nil
}

// VerifyChunks recomputes every chunk hash against the manifest (checksum
// gate; plan WS4 step 1).
func (m *Manifest) VerifyChunks(dir string) error {
	for _, c := range m.Chunks {
		if err := integrity.VerifyRecorded(filepath.Join(dir, c.Name), c.SHA256); err != nil {
			return fmt.Errorf("bundle: chunk %s: %w", c.Name, err)
		}
	}
	if err := integrity.VerifyRecorded(SidecarPath(dir), m.Sidecar.SHA256); err != nil {
		return fmt.Errorf("bundle: sidecar: %w", err)
	}
	return nil
}
