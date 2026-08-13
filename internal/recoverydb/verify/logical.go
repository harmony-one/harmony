package verify

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
)

// LogicalDigestDomain is the total-digest domain label.
const LogicalDigestDomain = "logical.total"

// LogicalDigest is the §11.4 logical KV digest: SHA-256 over
// len(key)‖key‖len(value)‖value in lexical key order, per namespace and
// total, domain-separated, with DigestExcludedKey as the single shared
// exclusion predicate.
type LogicalDigest struct {
	Total      report.Digest            `json:"total"`
	Buckets    map[string]report.Digest `json:"buckets"`
	TotalKeys  uint64                   `json:"total_keys"`
	TotalBytes uint64                   `json:"total_bytes"` // logical key+value bytes
}

// DigestExcludedKey is THE canonical logical-digest exclusion predicate,
// shared by every digest computation (compact's marker write, package
// binding, verify's raw scan — round 15 finding 1). Excluded, per the
// OPERATOR'S explicit specification decision (round 16 finding 1), which
// deliberately supersedes the plan's single-exclusion wording in §11.4:
//   - the recovery marker (the plan's original single exclusion: the digest
//     verifies against the value the marker embeds without self-reference —
//     round 7 finding 1);
//   - the preimage-generation bookkeeping pair, which the stock node
//     REWRITES on every clean Stop (core/blockchain_impl.go Stop →
//     WritePreImageStartEndBlock(db, 0, head)) and preimage-enabled open,
//     so including it would make sealed digests unstable across the very
//     boot + clean-stop cycle the release exists for (round 14 finding 4).
//     The pair is NOT unchecked: verify's raw scan requires it complete
//     (both keys or neither) with exact pinned values — see
//     validatePreimageMarkers. The deviation is also documented in the
//     shipped runbook (release/install_template.md).
func DigestExcludedKey(key []byte) bool {
	return bytes.Equal(key, keys.RecoveryMarkerKey) ||
		bytes.Equal(key, keys.PreimageGenStartKey) ||
		bytes.Equal(key, keys.PreimageGenEndKey)
}

// ComputeLogicalDigest scans the whole keyspace in lexical order. LevelDB
// iterators with a nil prefix yield keys in exactly that order.
func ComputeLogicalDigest(db ethdb.Iteratee) (*LogicalDigest, error) {
	total := report.NewHasher(LogicalDigestDomain)
	bucketHashers := map[string]*report.Hasher{}
	out := &LogicalDigest{Buckets: map[string]report.Digest{}}
	err := strictdb.ForEach(db, nil, func(key, value []byte) error {
		if DigestExcludedKey(key) {
			return nil
		}
		bucket := keys.Classify(key)
		h, ok := bucketHashers[bucket]
		if !ok {
			h = report.NewHasher("logical." + bucket)
			bucketHashers[bucket] = h
		}
		h.Add(key, value)
		total.Add(key, value)
		out.TotalKeys++
		out.TotalBytes += uint64(len(key) + len(value))
		return nil
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(bucketHashers))
	for name := range bucketHashers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.Buckets[name] = bucketHashers[name].Digest()
	}
	out.Total = total.Digest()
	return out, nil
}
