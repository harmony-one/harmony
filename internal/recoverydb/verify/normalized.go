package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
)

// Normalized-metadata section names (canonical order). The serialization/
// digest schema here is this plan's documented schema until the in-place
// effort agrees on a shared one (plan §8): each section is a domain-separated
// digest over its raw KV pairs in lexical key order; the normalized-output
// digest is SHA-256 over "name=digest\n" lines in canonical section order.
var NormalizedSections = []string{
	"validator-list",
	"validator-snapshots",
	"delegation-indexes",
	"shard-states",
	"epoch-records",
}

// ComputeNormalizedSections recomputes the canonical normalized-metadata
// sections from an artifact (plan WS5 step 1 / WS6 §11.1).
func ComputeNormalizedSections(db ethdb.Iteratee) (map[string]string, error) {
	out := map[string]string{}

	digestPrefixes := func(section string, wantBucket map[string]bool, prefixes ...[]byte) error {
		h := report.NewHasher("normalized." + section)
		for _, prefix := range prefixes {
			err := strictdb.ForEach(db, prefix, func(key, value []byte) error {
				bucket := keys.Classify(key)
				if bucket == keys.BucketBareHash32 {
					return nil
				}
				if !wantBucket[bucket] {
					return fmt.Errorf("verify: unexpected key %x (bucket %s) under normalized section %s", key, bucket, section)
				}
				h.Add(key, value)
				return nil
			})
			if err != nil {
				return err
			}
		}
		out[section] = h.Digest().SHA256
		return nil
	}

	if err := digestPrefixes("validator-list",
		map[string]bool{keys.BucketValidatorList: true}, keys.ValidatorListKey); err != nil {
		return nil, err
	}
	if err := digestPrefixes("validator-snapshots",
		map[string]bool{keys.BucketValidatorSnapshot: true}, keys.ValidatorSnapshotPrefix); err != nil {
		return nil, err
	}
	if err := digestPrefixes("delegation-indexes",
		map[string]bool{keys.BucketDVL: true}, keys.DVLPrefix); err != nil {
		return nil, err
	}
	if err := digestPrefixes("shard-states",
		map[string]bool{keys.BucketShardState: true}, keys.ShardStatePrefix); err != nil {
		return nil, err
	}
	if err := digestPrefixes("epoch-records",
		map[string]bool{
			keys.BucketEpochBlockNumber: true,
			keys.BucketEpochVrf:         true,
			keys.BucketEpochVdf:         true,
		},
		keys.EpochBlockNumberPrefix, keys.EpochVrfPrefix, keys.EpochVdfPrefix); err != nil {
		return nil, err
	}
	return out, nil
}

// NormalizedOutputDigest combines section digests into the single
// normalized-output digest recorded in the marker (computed from the
// artifact in BOTH reference and internal modes — it needs no external
// input, plan WS5 step 1).
func NormalizedOutputDigest(sections map[string]string) string {
	h := sha256.New()
	h.Write([]byte("hmy-recoverydb-normalized-output-v1\x00"))
	for _, name := range NormalizedSections {
		fmt.Fprintf(h, "%s=%s\n", name, sections[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// MetadataReferenceManifest is the optional in-place reference manifest
// consumed by compact-db/verify-db (--metadata-reference-manifest). Its
// recorded digest is the sibling <file>.sha256 (integrity layer).
type MetadataReferenceManifest struct {
	SchemaVersion string            `json:"schema_version"`
	Sections      map[string]string `json:"sections"`
}

// MetadataReferenceSchemaV1 is the accepted manifest schema.
const MetadataReferenceSchemaV1 = "hmy-recovery-metadata-reference-v1"

// LoadMetadataReferenceManifest verifies the manifest's recorded digest
// (sibling .sha256) and strictly decodes it, returning the manifest and its
// verified SHA-256 (the value that goes into the marker's reference field).
func LoadMetadataReferenceManifest(path string) (*MetadataReferenceManifest, string, error) {
	sum, err := integrity.VerifyChecksumFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("verify: metadata-reference manifest digest check: %w", err)
	}
	var m MetadataReferenceManifest
	if err := report.ReadJSONStrict(path, &m); err != nil {
		return nil, "", err
	}
	if m.SchemaVersion != MetadataReferenceSchemaV1 {
		return nil, "", fmt.Errorf("verify: unsupported metadata-reference schema %q", m.SchemaVersion)
	}
	var missing []string
	for _, name := range NormalizedSections {
		if _, ok := m.Sections[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, "", fmt.Errorf("verify: metadata-reference manifest missing sections %v", missing)
	}
	return &m, sum, nil
}

// CompareNormalizedSections returns per-section differences between the
// manifest's sections and the artifact's recomputed sections (empty=match).
func CompareNormalizedSections(manifest *MetadataReferenceManifest, artifact map[string]string) []string {
	var diffs []string
	for _, name := range NormalizedSections {
		if manifest.Sections[name] != artifact[name] {
			diffs = append(diffs, fmt.Sprintf("section %s: manifest %s, artifact %s", name, manifest.Sections[name], artifact[name]))
		}
	}
	return diffs
}
