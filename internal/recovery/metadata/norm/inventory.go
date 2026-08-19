package norm

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"sort"

	"github.com/harmony-one/harmony/internal/recovery/report"
	"github.com/harmony-one/harmony/internal/recovery/strictdb"

	"github.com/harmony-one/harmony/core/types"
)

// surveyInventory collects the informational, non-normative survey:
// validator-stats (kept untouched, §8 Q4 — never in the deletion plan, the
// .hmr, or the absence assertions), sync-era/legacy keys with value digests
// for B4's cleanup, and the per-shard crosslink/spent freeze inputs whose
// post-target subsets come from the audit (§4.4).
func (n *normalizer) surveyInventory() error {
	// validator-stats: count + informational digest.
	statsHash := sha256.New()
	err := strictdb.ForEach(n.s.Raw, prefixStats, func(key, value []byte) error {
		n.inventory.Stats.Count++
		statsHash.Write(FrameRecord(key, value))
		return nil
	})
	if err != nil {
		return err
	}
	if n.inventory.Stats.Count > 0 {
		n.inventory.Stats.FrameSHA = fmt.Sprintf("%x", statsHash.Sum(nil))
	}

	// Sync-era and legacy keys (reported only; not metadata sections).
	syncKeys := []string{
		"LastPivot", "TrieSync", "SnapshotDisabled", "SnapshotRoot", "SnapshotJournal",
		"SnapshotGenerator", "SnapshotRecovery", "SnapshotSyncStatus", "SkeletonSyncStatus",
		"SnapdbInfo", "unclean-shutdown", "InvalidBlock",
	}
	for _, k := range syncKeys {
		kb := []byte(k)
		has, err := n.s.Raw.Has(kb)
		if err != nil {
			return fmt.Errorf("norm: probe %s: %w", k, err)
		}
		if !has {
			continue
		}
		v, err := n.s.Raw.Get(kb)
		if err != nil {
			return fmt.Errorf("norm: read %s: %w", k, err)
		}
		n.inventory.SyncEra = append(n.inventory.SyncEra, SyncEraKey{
			Key: k, ValueSHA: report.SHA256Hex(v),
		})
	}
	if has, err := n.s.Raw.Has([]byte("continuous")); err != nil {
		return err
	} else if has {
		v, err := n.s.Raw.Get([]byte("continuous"))
		if err != nil {
			return err
		}
		n.inventory.LeaderContinuous = &SyncEraKey{Key: "continuous", ValueSHA: report.SHA256Hex(v)}
	}

	// Per-shard crosslink records + pointer + spent markers.
	type shardAcc struct {
		links   KeyDigestInventory
		linkH   hash.Hash
		pointer []byte
		spent   KeyDigestInventory
		spentH  hash.Hash
	}
	shardIDs := []uint32{}
	accs := map[uint32]*shardAcc{}
	acc := func(sid uint32) *shardAcc {
		a, ok := accs[sid]
		if !ok {
			a = &shardAcc{linkH: sha256.New(), spentH: sha256.New()}
			accs[sid] = a
			shardIDs = append(shardIDs, sid)
		}
		return a
	}

	err = strictdb.ForEach(n.s.Raw, prefixCrossLink, func(key, value []byte) error {
		ns, meta := strictdb.Classify(key)
		switch ns {
		case strictdb.NsCrossLinkPointer:
			a := acc(meta.ShardID)
			a.pointer = append([]byte(nil), value...)
		case strictdb.NsCrossLink:
			a := acc(meta.ShardID)
			if a.links.Count == 0 || meta.Number < a.links.MinNumber {
				a.links.MinNumber = meta.Number
			}
			if meta.Number > a.links.MaxNumber {
				a.links.MaxNumber = meta.Number
			}
			a.links.Count++
			a.linkH.Write(FrameRecord(key, value))
		default:
			// "cl"-prefixed junk of unexpected shape: report, never plan.
			n.addFinding(report.SeverityInfo, report.ClassDiagnostic, "crosslink-unexpected-key-shape", key,
				"key under the cl prefix has neither pointer nor record shape", false)
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = strictdb.ForEach(n.s.Raw, prefixCXSpent, func(key, value []byte) error {
		ns, meta := strictdb.Classify(key)
		if ns != strictdb.NsCXReceiptSpent {
			n.addFinding(report.SeverityInfo, report.ClassDiagnostic, "cxspent-unexpected-key-shape", key,
				"key under the cxReceiptSpent prefix has an unexpected shape", false)
			return nil
		}
		a := acc(meta.ShardID)
		if a.spent.Count == 0 || meta.Number < a.spent.MinNumber {
			a.spent.MinNumber = meta.Number
		}
		if meta.Number > a.spent.MaxNumber {
			a.spent.MaxNumber = meta.Number
		}
		a.spent.Count++
		a.spentH.Write(FrameRecord(key, value))
		return nil
	})
	if err != nil {
		return err
	}

	sort.Slice(shardIDs, func(i, j int) bool { return shardIDs[i] < shardIDs[j] })
	for _, sid := range shardIDs {
		a := accs[sid]
		inv := ShardInventory{ShardID: sid, CrossLinks: a.links, CXReceiptsSpent: a.spent}
		if a.links.Count > 0 {
			inv.CrossLinks.FrameSHA = fmt.Sprintf("%x", a.linkH.Sum(nil))
		}
		if a.spent.Count > 0 {
			inv.CXReceiptsSpent.FrameSHA = fmt.Sprintf("%x", a.spentH.Sum(nil))
		}
		if a.pointer != nil {
			inv.PointerPresent = true
			inv.PointerValueSHA = report.SHA256Hex(a.pointer)
			if cl, err := types.DeserializeCrossLink(a.pointer); err == nil {
				inv.PointerBlockNum = cl.BlockNum()
			}
		}
		n.inventory.Shards = append(n.inventory.Shards, inv)
	}
	return nil
}
