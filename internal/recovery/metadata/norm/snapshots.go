package norm

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/recovery/report"
	"github.com/harmony-one/harmony/internal/recovery/strictdb"
	staking "github.com/harmony-one/harmony/staking/types"
)

// checkCanonicalEpochSuffix enforces the canonical-suffix rule shared by
// every epoch-suffixed namespace (§4.4): the raw suffix must byte-equal
// epoch.Bytes(). It returns the parsed epoch; a noncanonical alias emits a
// Fatal NoncanonicalKey and, if the canonical twin also exists, a Fatal
// duplicate. A failed canonical-twin probe is a genuine I/O error and
// propagates (exit 14 at the CLI) — it is never converted into "no twin".
func (n *normalizer) checkCanonicalEpochSuffix(key, suffix []byte, canonicalKey func(*big.Int) []byte, dupCount *uint64) (*big.Int, bool, error) {
	epoch := new(big.Int).SetBytes(suffix)
	if bytes.Equal(suffix, epoch.Bytes()) {
		return epoch, true, nil
	}
	n.fatal(report.ClassNoncanonicalKey, "noncanonical-epoch-suffix", key,
		fmt.Sprintf("epoch suffix %x is not the canonical big.Int encoding of %s (production only emits canonical keys)", suffix, epoch))
	canon := canonicalKey(epoch)
	has, err := n.s.Raw.Has(canon)
	if err != nil {
		return epoch, false, fmt.Errorf("norm: probe canonical twin %x: %w", canon, err)
	}
	if has {
		if dupCount != nil {
			*dupCount++
		}
		n.fatal(report.ClassInvalidRetained, "duplicate-logical-record", key,
			fmt.Sprintf("alias key and canonical key %x both present for epoch %s", canon, epoch))
	}
	return epoch, false, nil
}

// snapshots implements the §4.4 validator-snapshot rules.
func (n *normalizer) snapshots() error {
	targetEpoch := epochBig(n.a.Epoch)
	seenTarget := map[common.Address]bool{}

	err := strictdb.ForEach(n.s.Raw, prefixSnapshot, func(key, value []byte) error {
		rest := key[len(prefixSnapshot):]
		if len(rest) < 20 {
			n.counts.Snapshots.Invalid++
			n.fatal(report.ClassInvalidRetained, "snapshot-malformed-key", key,
				fmt.Sprintf("key too short for address component (len %d)", len(key)))
			return nil
		}
		addr := common.BytesToAddress(rest[:20])
		suffix := rest[20:]
		epoch, canonical, err := n.checkCanonicalEpochSuffix(key, suffix, func(e *big.Int) []byte {
			return snapshotKey(addr, e)
		}, &n.counts.Snapshots.Duplicate)
		if err != nil {
			return err
		}
		if !canonical {
			n.counts.Snapshots.Invalid++
			return nil
		}

		switch {
		case epoch.Uint64() > n.a.Epoch || !epoch.IsUint64():
			// Includes the abandoned next-epoch batch.
			n.counts.Snapshots.Removed++
			n.snapDeletions = append(n.snapDeletions, PlannedDeletion{
				Key: hexKey(key), Reason: "future-epoch",
			})
		case epoch.Uint64() == n.a.Epoch:
			idx, listed := n.wrapperByAddr[addr]
			if !listed {
				// Not in the normalized list: post-target-created
				// (audit-reconciled) — covers both removed list entries and
				// unknown addresses.
				n.counts.Snapshots.Removed++
				n.snapDeletions = append(n.snapDeletions, PlannedDeletion{
					Key: hexKey(key), Reason: "post-target-created",
				})
				return nil
			}
			if seenTarget[addr] {
				// Raw-key uniqueness makes this unreachable for canonical
				// keys; defensive.
				n.counts.Snapshots.Duplicate++
				n.fatal(report.ClassInvalidRetained, "snapshot-duplicate", key,
					fmt.Sprintf("second target-epoch snapshot for %s", addr.Hex()))
				return nil
			}
			seenTarget[addr] = true
			if !n.validateRetainedSnapshot(key, value, addr, &n.wrappers[idx]) {
				n.counts.Snapshots.Invalid++
				return nil
			}
			n.counts.Snapshots.Retained++
			n.set.Snapshots = append(n.set.Snapshots, Record{
				Key: append([]byte(nil), key...), Value: append([]byte(nil), value...),
			})
		default: // epoch < target: preserved untouched; diagnostics only.
			n.inventory.SnapshotsPriorEpoch.Count++
			var w staking.ValidatorWrapper
			if err := rlp.DecodeBytes(value, &w); err != nil {
				n.addFinding(report.SeverityReviewItem, report.ClassDiagnostic,
					"prior-epoch-snapshot-undecodable", key,
					fmt.Sprintf("epoch %s snapshot does not decode: %v (preserved untouched)", epoch, err), true)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Every listed validator must have exactly one target-epoch snapshot;
	// canonical-suffix enforcement + raw-key uniqueness give "at most one",
	// §2.1 write points give "at least one".
	for _, addr := range n.normalizedList {
		if !seenTarget[addr] {
			n.counts.Snapshots.Missing++
			n.fatal(report.ClassMissingRequired, "snapshot-missing-for-listed", snapshotKey(addr, targetEpoch),
				fmt.Sprintf("listed validator %s has no epoch-%d snapshot (clean-DB fallback signal)", addr.Hex(), n.a.Epoch))
		}
	}
	sortRecords(n.set.Snapshots)
	return nil
}

// validateRetainedSnapshot runs the strict decode + embedded-address check
// and the best-effort content verification (§4.4, one behavior — §8 Q9).
func (n *normalizer) validateRetainedSnapshot(key, value []byte, addr common.Address, entry *wrapperEntry) bool {
	var w staking.ValidatorWrapper
	if err := rlp.DecodeBytes(value, &w); err != nil {
		n.fatal(report.ClassInvalidRetained, "snapshot-undecodable", key,
			fmt.Sprintf("retained snapshot for %s does not strict-decode: %v", addr.Hex(), err))
		return false
	}
	if w.Address != addr {
		n.fatal(report.ClassInvalidRetained, "snapshot-address-mismatch", key,
			fmt.Sprintf("retained snapshot embeds address %s, key says %s", w.Address.Hex(), addr.Hex()))
		return false
	}

	// Best-effort reconstruction: H = max(snapshotBase, CreationHeight)
	// (§2.1 write points). Where the root does not resolve, structural
	// checks stand alone (structural-only cannot detect corruption that
	// stays well-formed RLP with the right embedded address).
	if n.s.Hist == nil || entry.wrapper == nil || entry.wrapper.CreationHeight == nil || !entry.wrapper.CreationHeight.IsUint64() {
		n.coverage.StructuralOnly++
		return true
	}
	h := n.a.SnapshotBase
	if ch := entry.wrapper.CreationHeight.Uint64(); ch > h {
		h = ch
	}
	hist, err := n.s.Hist.StateAt(h)
	if err != nil || hist == nil {
		n.coverage.StructuralOnly++
		return true
	}
	expected := hist.GetCode(addr)
	if herr := hist.Error(); herr != nil {
		// Partial historical trie: unresolvable, not a mismatch.
		n.coverage.StructuralOnly++
		return true
	}
	if len(expected) == 0 {
		n.fatal(report.ClassInvalidRetained, "snapshot-account-absent-at-write-height", key,
			fmt.Sprintf("historical state at %d resolves but holds no wrapper for %s", h, addr.Hex()))
		return false
	}
	if !bytes.Equal(expected, value) {
		n.fatal(report.ClassInvalidRetained, "snapshot-content-mismatch", key,
			fmt.Sprintf("retained snapshot bytes differ from the wrapper at height %d (creation %s)", h, entry.wrapper.CreationHeight))
		return false
	}
	n.coverage.ReconstructedVerified++
	return true
}
