package hmr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// fakeSHA is a syntactically valid 64-hex digest generator for the sample
// (Validate enforces digest shapes; the values are otherwise arbitrary).
func fakeSHA(seed byte) string {
	return report.SHA256Hex([]byte{seed})
}

func sampleResult() (norm.Anchor, *norm.Result, []byte) {
	a := norm.Anchor{
		Network: "mainnet", Shard: 0, TargetHeight: 92730034,
		TargetHash:      common.HexToHash("0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d"),
		TargetRoot:      common.HexToHash("0xdeadbeef"),
		Epoch:           3002,
		EpochFirst:      92700672,
		EpochLast:       92733439,
		SnapshotBase:    92700670,
		ConfigSHA256Hex: fakeSHA(0xA0),
	}
	// The complete canonical section list for epoch 3002 (Validate
	// enforces the exact names, order and cardinalities: validator-list,
	// shard-state and reward-accumulator are exactly one record; the
	// snapshot section is nonzero; dvl is variable).
	names := norm.SectionNames(a.Epoch)
	counts := []uint64{1, 7, 3, 1, 1} // [validator-list, dvl, snapshots, shard-state, reward-accumulator]
	sections := make([]norm.SectionDigest, 0, len(names))
	for i, name := range names {
		sections = append(sections, norm.SectionDigest{
			Name: name, RecordCount: counts[i], SHA256: fakeSHA(byte(i)),
		})
	}
	// The full canonical absence-assertion set for (epoch, target), each
	// with expected_remaining 0 and an arbitrary per-run planned count.
	specs := norm.CanonicalAssertionSpecs(a.Epoch, a.TargetHeight)
	assertions := make([]norm.AbsenceAssertion, 0, len(specs))
	for i, s := range specs {
		assertions = append(assertions, norm.AbsenceAssertion{
			Namespace: s.Namespace, Predicate: s.Predicate,
			PlannedDeletions: uint64(i * 3), ExpectedRemaining: 0,
		})
	}
	res := &norm.Result{
		Digests: norm.DigestSet{
			Sections:    sections,
			WrapperSet:  fakeSHA(0xB0),
			Diagnostics: fakeSHA(0xC0),
		},
		Assertions: assertions,
	}
	pkg := []byte("fake-hmr-bytes")
	return a, res, pkg
}

func TestManifestRoundTrip(t *testing.T) {
	a, res, pkg := sampleResult()
	m := BuildManifest(a, res, pkg)
	enc, err := EncodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecodeManifest(enc)
	if err != nil {
		t.Fatal(err)
	}
	// Re-encode must be byte-identical (canonical).
	enc2, _ := EncodeManifest(dec)
	if !bytes.Equal(enc, enc2) {
		t.Fatal("manifest re-encode not byte-stable")
	}
}

func TestManifestExcludesPlannedDeletionsAndTimestamps(t *testing.T) {
	a, res, pkg := sampleResult()
	m := BuildManifest(a, res, pkg)
	enc, err := EncodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(enc)
	if strings.Contains(s, "planned_deletions") {
		t.Fatal("reference manifest must not carry planned_deletions (source-specific run evidence)")
	}
	for _, ts := range []string{"created_at", "timestamp", "started_at", "duration"} {
		if strings.Contains(s, ts) {
			t.Fatalf("reference manifest must be timestamp-free, found %q", ts)
		}
	}
	// It must carry the required end-state predicate + expected_remaining.
	if !strings.Contains(s, "expected_remaining") || !strings.Contains(s, "epoch>3002") {
		t.Fatal("reference manifest missing absence-assertion predicate/end-state")
	}
}

func TestReferenceDigestBindings(t *testing.T) {
	a, res, pkg := sampleResult()
	base := func() string {
		m := BuildManifest(a, res, pkg)
		enc, _ := EncodeManifest(m)
		return report.SHA256Hex(enc)
	}
	baseline := base()

	// Flipping a payload byte changes the package digest -> reference
	// digest.
	pkg2 := append([]byte(nil), pkg...)
	pkg2[0] ^= 0xff
	m2 := BuildManifest(a, res, pkg2)
	e2, _ := EncodeManifest(m2)
	if report.SHA256Hex(e2) == baseline {
		t.Fatal("changing the package bytes must change the reference digest")
	}

	// Flipping a section digest changes the reference digest.
	res3 := *res
	res3.Digests.Sections = append([]norm.SectionDigest(nil), res.Digests.Sections...)
	res3.Digests.Sections[0].SHA256 = "ffff"
	m3 := BuildManifest(a, &res3, pkg)
	e3, _ := EncodeManifest(m3)
	if report.SHA256Hex(e3) == baseline {
		t.Fatal("changing a section digest must change the reference digest")
	}

	// Varying only per-run planned-deletion counts does NOT change the
	// reference digest (junk-insensitivity at the manifest level).
	res4 := *res
	res4.Assertions = append([]norm.AbsenceAssertion(nil), res.Assertions...)
	for i := range res4.Assertions {
		res4.Assertions[i].PlannedDeletions += 777 // different junk
	}
	m4 := BuildManifest(a, &res4, pkg)
	e4, _ := EncodeManifest(m4)
	if report.SHA256Hex(e4) != baseline {
		t.Fatal("per-run planned-deletion counts must NOT affect the reference digest")
	}
}

func TestManifestRejectsUnknownFields(t *testing.T) {
	bad := []byte(`{"schema":"hmr-reference-v1","surprise":1}`)
	if _, err := DecodeManifest(bad); err == nil {
		t.Fatal("unknown fields must be rejected")
	}
}

// TestManifestValidationRejectsIncomplete pins the strict-consumer
// contract: schema-only documents, wrong digest shapes, section
// order/count violations, record-count mismatches, missing assertions and
// non-canonical encodings are all rejected.
func TestManifestValidationRejectsIncomplete(t *testing.T) {
	// Schema alone is NOT a manifest.
	if _, err := DecodeManifest([]byte(`{"schema":"hmr-reference-v1"}`)); err == nil {
		t.Fatal("a schema-only document must be rejected")
	}

	a, res, pkg := sampleResult()
	valid := BuildManifest(a, res, pkg)
	enc, err := EncodeManifest(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeManifest(enc); err != nil {
		t.Fatalf("the canonical sample must decode: %v", err)
	}

	mutate := func(name string, f func(m *Manifest)) {
		t.Helper()
		m := BuildManifest(a, res, pkg)
		f(m)
		bad, err := EncodeManifest(m)
		if err != nil {
			t.Fatalf("%s: encode: %v", name, err)
		}
		if _, err := DecodeManifest(bad); err == nil {
			t.Fatalf("%s: must be rejected", name)
		}
	}
	mutate("short package digest", func(m *Manifest) { m.PackageSHA256 = "abcd" })
	mutate("uppercase digest", func(m *Manifest) { m.WrapperSetSHA256 = strings.ToUpper(m.WrapperSetSHA256) })
	mutate("section order swapped", func(m *Manifest) {
		m.Sections[0], m.Sections[1] = m.Sections[1], m.Sections[0]
	})
	mutate("section dropped", func(m *Manifest) { m.Sections = m.Sections[:len(m.Sections)-1] })
	mutate("record-count mismatch", func(m *Manifest) { m.RecordCount++ })
	mutate("assertions dropped", func(m *Manifest) { m.Assertions = nil })
	mutate("ruleset drift", func(m *Manifest) { m.RulesetVersion = "hmr-norm-v0" })
	mutate("bad anchor hash", func(m *Manifest) { m.Anchor.TargetHash = "0x1234" })
	// Section cardinality: validator-list / shard-state / reward-accumulator
	// must be exactly one record; the snapshot section must be nonzero.
	mutate("validator-list count != 1", func(m *Manifest) { m.Sections[0].RecordCount = 2; m.RecordCount++ })
	mutate("shard-state count != 1", func(m *Manifest) { m.Sections[3].RecordCount = 0; m.RecordCount-- })
	mutate("reward-accumulator count != 1", func(m *Manifest) { m.Sections[4].RecordCount = 3; m.RecordCount += 2 })
	mutate("snapshot count zero", func(m *Manifest) { m.RecordCount -= m.Sections[2].RecordCount; m.Sections[2].RecordCount = 0 })
	// Absence assertions: exact canonical set, order and zero end-state.
	mutate("assertion reordered", func(m *Manifest) {
		m.Assertions[0], m.Assertions[1] = m.Assertions[1], m.Assertions[0]
	})
	mutate("assertion extra", func(m *Manifest) {
		m.Assertions = append(m.Assertions, ManifestAssertion{Namespace: "x", Predicate: "present"})
	})
	mutate("assertion nonzero expected_remaining", func(m *Manifest) { m.Assertions[0].ExpectedRemaining = 1 })
	mutate("assertion wrong predicate", func(m *Manifest) { m.Assertions[0].Predicate = "epoch>1" })

	// Non-canonical byte forms of a valid manifest are rejected (the
	// reference digest binds exactly one byte form).
	withSpace := append([]byte(nil), enc...)
	withSpace = bytes.Replace(withSpace, []byte(`{"absence_assertions"`), []byte(`{ "absence_assertions"`), 1)
	if _, err := DecodeManifest(withSpace); err == nil {
		t.Fatal("whitespace variant must be rejected (non-canonical)")
	}
}
