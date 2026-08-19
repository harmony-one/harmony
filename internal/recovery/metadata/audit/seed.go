package audit

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
)

// chain tombstone key builders (mirroring core/rawdb/schema.go shapes).
func u64be8(n uint64) []byte {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = byte(n)
		n >>= 8
	}
	return b
}

func canonicalKey(n uint64) []byte { return append(append([]byte("h"), u64be8(n)...), 'n') }
func headerKey(n uint64, h common.Hash) []byte {
	return append(append([]byte("h"), u64be8(n)...), h.Bytes()...)
}
func headerTDKey(n uint64, h common.Hash) []byte  { return append(headerKey(n, h), 't') }
func headerNumberKey(h common.Hash) []byte        { return append([]byte("H"), h.Bytes()...) }
func bodyKey(n uint64, h common.Hash) []byte      { return append(append([]byte("b"), u64be8(n)...), h.Bytes()...) }
func receiptsKey(n uint64, h common.Hash) []byte  { return append(append([]byte("r"), u64be8(n)...), h.Bytes()...) }
func blockSigKey(n uint64) []byte                 { return append([]byte("block-sig-"), u64be8(n)...) }
func crosslinkKey(shardID uint32, n uint64) []byte {
	return append(append([]byte("cl"), u32be4(shardID)...), u64be8(n)...)
}

var headKeys = [][]byte{[]byte("LastHeader"), []byte("LastBlock"), []byte("LastFast"), []byte("LastFinalized")}

// SeedSpec captures what was seeded, for the report and the seed-equality
// acceptance test.
type SeedSpec struct {
	PlanDeletions   int      `json:"plan_deletions"`
	PlanRewrites    int      `json:"plan_rewrites"`
	ChainTombstones int      `json:"chain_tombstones"`
	BranchHashes    int      `json:"branch_hashes"`
	ExtraMaskedKeys int      `json:"extra_masked_keys"` // pass-2 crosslink/spent masks
	PointerSeeds    []string `json:"pointer_seeds,omitempty"`
}

// Seed applies the full mechanical application of the normalization output
// to the overlay (plan §4.6): every DeletionPlan deletion becomes a
// tombstone, every rewrite is materialized (masking alone would make
// rewritten keys read absent — not B4's end state), post-target canonical
// chain records are tombstoned, and the heads are rewritten to the target
// tuple. extraMask carries pass-2's branch-written crosslink/spent keys;
// pointerSeeds carries the derived pass-2 pointer records.
func Seed(o *Overlay, res *norm.Result, src sourceReader, targetHash common.Hash, targetHeight uint64,
	extraMask [][]byte, pointerSeeds map[string][]byte) (*SeedSpec, error) {
	spec := &SeedSpec{}

	// 1. Deletion tombstones.
	for _, d := range res.Deletions.Deletions() {
		o.Mask(common.FromHex(d.Key))
		spec.PlanDeletions++
	}
	// 2. Materialized rewrites: the normalized values for every rewrite
	// key come from the NormalizedSet (the plan itself carries hashes
	// only).
	rewriteKeys := map[string]struct{}{}
	for _, rw := range res.Deletions.Rewrites() {
		rewriteKeys[string(common.FromHex(rw.Key))] = struct{}{}
	}
	seedRecord := func(r norm.Record) error {
		if _, ok := rewriteKeys[string(r.Key)]; !ok {
			return nil
		}
		if err := o.SeedPut(r.Key, r.Value); err != nil {
			return err
		}
		spec.PlanRewrites++
		return nil
	}
	if err := seedRecord(res.Normalized.ValidatorList); err != nil {
		return nil, err
	}
	for _, r := range res.Normalized.DVL {
		if err := seedRecord(r); err != nil {
			return nil, err
		}
	}
	if spec.PlanRewrites != len(rewriteKeys) {
		return nil, fmt.Errorf("audit: %d rewrites in the plan but only %d normalized values found to materialize",
			len(rewriteKeys), spec.PlanRewrites)
	}

	// 3. Post-target canonical chain tombstones: walk the source's
	// canonical mappings upward until the first gap.
	for n := targetHeight + 1; ; n++ {
		hash, err := src.CanonicalHash(n)
		if err != nil || hash == (common.Hash{}) {
			break
		}
		o.Mask(canonicalKey(n))
		o.Mask(headerKey(n, hash))
		o.Mask(headerTDKey(n, hash))
		o.Mask(headerNumberKey(hash))
		o.Mask(bodyKey(n, hash))
		o.Mask(receiptsKey(n, hash))
		o.Mask(blockSigKey(n))
		spec.ChainTombstones += 7
		spec.BranchHashes++
	}
	// Legacy LastCommits is masked (plan §4.6; its deletion is already in
	// the plan when present, but the mask is unconditional).
	o.Mask([]byte("LastCommits"))

	// 4. Heads rewound to the target tuple.
	for _, hk := range headKeys {
		if err := o.SeedPut(hk, targetHash.Bytes()); err != nil {
			return nil, err
		}
	}

	// 5. Pass-2 additions: branch-written crosslink/spent masks and the
	// derived pointer seeds.
	for _, k := range extraMask {
		o.Mask(k)
		spec.ExtraMaskedKeys++
	}
	for k, v := range pointerSeeds {
		if err := o.SeedPut([]byte(k), v); err != nil {
			return nil, err
		}
		spec.PointerSeeds = append(spec.PointerSeeds, fmt.Sprintf("%x", k))
	}

	o.SealSeed()
	return spec, nil
}

// sourceReader is the minimal side-handle surface Seed needs.
type sourceReader interface {
	CanonicalHash(n uint64) (common.Hash, error)
}
