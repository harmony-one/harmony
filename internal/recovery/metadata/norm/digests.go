package norm

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/harmony-one/harmony/internal/recovery/report"
)

// FrameRecord returns the pre-registered record frame (plan §4.5):
// u32be(len(key)) ‖ key ‖ u64be(len(value)) ‖ value.
func FrameRecord(key, value []byte) []byte {
	out := make([]byte, 0, 4+len(key)+8+len(value))
	var k4 [4]byte
	binary.BigEndian.PutUint32(k4[:], uint32(len(key)))
	out = append(out, k4[:]...)
	out = append(out, key...)
	var v8 [8]byte
	binary.BigEndian.PutUint64(v8[:], uint64(len(value)))
	out = append(out, v8[:]...)
	return append(out, value...)
}

// SectionSHA256 computes SHA-256("hmr1/section/" ‖ name ‖ 0x00 ‖
// concat(record frames in key order)).
func SectionSHA256(name string, records []Record) string {
	h := sha256.New()
	h.Write([]byte("hmr1/section/"))
	h.Write([]byte(name))
	h.Write([]byte{0})
	for _, r := range records {
		h.Write(FrameRecord(r.Key, r.Value))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// wrapperSetSHA256 computes SHA-256("hmr1/wrappers" ‖ 0x00 ‖ concat(addr(20)
// ‖ u64be(len(code)) ‖ code)) in normalized list order; code is the raw
// stored wrapper bytes at the target root, hashed unre-encoded.
func wrapperSetSHA256(ordered []wrapperEntry) string {
	h := sha256.New()
	h.Write([]byte("hmr1/wrappers"))
	h.Write([]byte{0})
	for _, w := range ordered {
		h.Write(w.addr.Bytes())
		var v8 [8]byte
		binary.BigEndian.PutUint64(v8[:], uint64(len(w.code)))
		h.Write(v8[:])
		h.Write(w.code)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// diagnosticsSHA256 hashes the canonical JSON of the chain-deterministic
// findings, sorted by (code, key) — plan §4.5.
func diagnosticsSHA256(findings []report.Finding) (string, error) {
	var chainDet []report.Finding
	for _, f := range findings {
		if f.ChainDeterministic {
			chainDet = append(chainDet, f)
		}
	}
	report.SortFindings(chainDet)
	if chainDet == nil {
		chainDet = []report.Finding{}
	}
	return report.DigestCanonicalJSON(chainDet)
}

// digestSet assembles the DigestSet from the normalized sections.
func digestSet(a Anchor, set *NormalizedSet, wrappers []wrapperEntry, findings []report.Finding) (DigestSet, error) {
	sections := []SectionDigest{
		{Name: "validator-list", RecordCount: 1,
			SHA256: SectionSHA256("validator-list", []Record{set.ValidatorList})},
		{Name: "dvl", RecordCount: uint64(len(set.DVL)),
			SHA256: SectionSHA256("dvl", set.DVL)},
		{Name: sectionSnapshots(a.Epoch), RecordCount: uint64(len(set.Snapshots)),
			SHA256: SectionSHA256(sectionSnapshots(a.Epoch), set.Snapshots)},
		{Name: sectionShardState(a.Epoch), RecordCount: 1,
			SHA256: SectionSHA256(sectionShardState(a.Epoch), []Record{set.ShardState})},
		{Name: "reward-accumulator", RecordCount: 1,
			SHA256: SectionSHA256("reward-accumulator", []Record{set.RewardAccumulator})},
	}
	diag, err := diagnosticsSHA256(findings)
	if err != nil {
		return DigestSet{}, err
	}
	return DigestSet{
		Sections:    sections,
		WrapperSet:  wrapperSetSHA256(wrappers),
		Diagnostics: diag,
	}, nil
}
