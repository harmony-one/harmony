package norm

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/internal/recovery/report"
	"github.com/harmony-one/harmony/internal/recovery/strictdb"
)

// shardStates implements the ss rules (§4.4): ss<epoch> must exist and
// byte-equal the boundary header's ShardState field (§2.1: the epoch-last
// block of the prior epoch carries it; production writes the raw header
// field bytes unchanged). Future epochs are deleted (the audit
// byte-verifies its reproduced next-epoch record against the deleted one);
// prior epochs are preserved.
func (n *normalizer) shardStates() error {
	targetKey := shardStateKey(epochBig(n.a.Epoch))
	var targetSeen bool

	err := strictdb.ForEach(n.s.Raw, prefixShardState, func(key, value []byte) error {
		suffix := key[len(prefixShardState):]
		epoch, canonical, cerr := n.checkCanonicalEpochSuffix(key, suffix, func(e *big.Int) []byte {
			return shardStateKey(e)
		}, &n.counts.ShardState.Duplicate)
		if cerr != nil {
			return cerr
		}
		if !canonical {
			n.counts.ShardState.Invalid++
			return nil
		}
		switch {
		case !epoch.IsUint64() || epoch.Uint64() > n.a.Epoch:
			n.counts.ShardState.Removed++
			n.epochDeletions = append(n.epochDeletions, PlannedDeletion{
				Key: hexKey(key), Reason: "future-epoch",
			})
		case epoch.Uint64() == n.a.Epoch:
			targetSeen = true
			expected, err := n.boundaryShardState()
			if err != nil {
				n.fatal(report.ClassMissingRequired, "boundary-header-missing", key,
					fmt.Sprintf("cannot read boundary header %d to verify ss<%d>: %v (conservatively a fallback signal)",
						n.a.BoundaryHeight, n.a.Epoch, err))
				return nil
			}
			if !bytes.Equal(value, expected) {
				n.counts.ShardState.Invalid++
				n.fatal(report.ClassInvalidRetained, "shard-state-mismatch", key,
					fmt.Sprintf("ss<%d> differs from header(%d).ShardState()", n.a.Epoch, n.a.BoundaryHeight))
				return nil
			}
			n.counts.ShardState.Retained++
			n.set.ShardState = Record{Key: append([]byte(nil), key...), Value: nonNil(value)}
		default: // prior epochs preserved untouched
			n.counts.ShardState.Retained++
			n.inventory.ShardStatesPrior.Count++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !targetSeen {
		n.counts.ShardState.Missing++
		n.fatal(report.ClassMissingRequired, "shard-state-missing", targetKey,
			fmt.Sprintf("ss<%d> absent (clean-DB fallback signal)", n.a.Epoch))
		n.set.ShardState = Record{Key: targetKey, Value: nil}
	}
	return nil
}

// boundaryShardState reads header(BoundaryHeight).ShardState() once.
func (n *normalizer) boundaryShardState() ([]byte, error) {
	hdr, err := n.s.Headers.HeaderByNumber(n.a.BoundaryHeight)
	if err != nil {
		return nil, err
	}
	if hdr == nil {
		return nil, fmt.Errorf("boundary header %d not found", n.a.BoundaryHeight)
	}
	return hdr.ShardState(), nil
}

// epochAux applies the dead-writer epoch-suffixed rules (§2.1): epochs
// above the target are deleted defensively; any observed hit is itemized
// (expected zero for epochs > target on mainnet).
func (n *normalizer) epochAux() error {
	type ns struct {
		prefix []byte
		label  string
	}
	for _, name := range []ns{
		{prefixEpochNumber, "harmony-epoch-block-number"},
		{prefixEpochVRF, "epoch-vrf-block-numbers"},
		{prefixEpochVDF, "epoch-vdf-block-number"},
	} {
		name := name
		err := strictdb.ForEach(n.s.Raw, name.prefix, func(key, value []byte) error {
			suffix := key[len(name.prefix):]
			epoch, canonical, cerr := n.checkCanonicalEpochSuffix(key, suffix, func(e *big.Int) []byte {
				return append(append([]byte(nil), name.prefix...), e.Bytes()...)
			}, &n.counts.EpochAux.Duplicate)
			if cerr != nil {
				return cerr
			}
			if !canonical {
				n.counts.EpochAux.Invalid++
				return nil
			}
			if !epoch.IsUint64() || epoch.Uint64() > n.a.Epoch {
				n.counts.EpochAux.Removed++
				n.epochDeletions = append(n.epochDeletions, PlannedDeletion{
					Key: hexKey(key), Reason: "future-epoch-dead-writer",
				})
				n.addFinding(report.SeverityInfo, report.ClassDiagnostic, "dead-writer-key-observed", key,
					fmt.Sprintf("%s record for epoch %s above target (dead writer namespace; expected zero)", name.label, epoch), false)
				return nil
			}
			n.counts.EpochAux.Retained++
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// rewardAccumulator applies the blk-rwd rules (§4.4): the target record is
// mandatory and included in the .hmr (§8 Q5); records above the target are
// deleted (exact-key arithmetic, ~20,982 keys on mainnet); records below
// are preserved.
func (n *normalizer) rewardAccumulator() error {
	targetKey := blkRwdKey(n.a.TargetHeight)
	var targetSeen bool
	err := strictdb.ForEach(n.s.Raw, prefixBlkRwd, func(key, value []byte) error {
		rest := key[len(prefixBlkRwd):]
		if len(rest) != 8 {
			n.counts.RewardAccum.Invalid++
			n.fatal(report.ClassInvalidRetained, "blk-rwd-malformed-key", key,
				fmt.Sprintf("blk-rwd key length %d, want %d", len(key), len(prefixBlkRwd)+8))
			return nil
		}
		var number uint64
		for _, b := range rest {
			number = number<<8 | uint64(b)
		}
		switch {
		case number == n.a.TargetHeight:
			targetSeen = true
			n.counts.RewardAccum.Retained++
			// A zero accumulator stores an empty value; keep it present
			// (non-nil) so the .hmr section frames it as a real record.
			n.set.RewardAccumulator = Record{Key: append([]byte(nil), key...), Value: nonNil(value)}
		case number > n.a.TargetHeight:
			n.counts.RewardAccum.Removed++
			n.epochDeletions = append(n.epochDeletions, PlannedDeletion{
				Key: hexKey(key), Reason: "post-target",
			})
		default:
			n.counts.RewardAccum.Retained++
			n.inventory.RewardAccumPrior.Count++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !targetSeen {
		n.counts.RewardAccum.Missing++
		n.fatal(report.ClassMissingRequired, "blk-rwd-target-missing", targetKey,
			fmt.Sprintf("blk-rwd-%d absent (clean-DB fallback signal)", n.a.TargetHeight))
		n.set.RewardAccumulator = Record{Key: targetKey, Value: nil}
	}
	return nil
}

// pendingAndLegacy plans the unconditional deletions: pendingCL/pendingSC
// (handoff §2.2, node-local queues) and the dead legacy LastCommits key
// (§8 Q6; §2.2 safety — B4 writes the exact block-sig-<target> so nothing
// ever falls back).
func (n *normalizer) pendingAndLegacy() error {
	for _, p := range [][]byte{prefixPendingCL, prefixPendingSC} {
		p := p
		err := strictdb.ForEach(n.s.Raw, p, func(key, value []byte) error {
			n.counts.Pending.Removed++
			n.epochDeletions = append(n.epochDeletions, PlannedDeletion{
				Key: hexKey(key), Reason: "node-local-queue",
			})
			return nil
		})
		if err != nil {
			return err
		}
	}
	has, err := n.s.Raw.Has(keyLastCommits)
	if err != nil {
		return fmt.Errorf("norm: probe LastCommits: %w", err)
	}
	if has {
		n.counts.Pending.Removed++
		n.epochDeletions = append(n.epochDeletions, PlannedDeletion{
			Key: hexKey(keyLastCommits), Reason: "legacy-dead-key",
		})
	}
	return nil
}
