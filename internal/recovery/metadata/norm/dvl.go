package norm

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/recovery/report"
	"github.com/harmony-one/harmony/internal/recovery/strictdb"
	staking "github.com/harmony-one/harmony/staking/types"
)

// dvl implements the §4.4 delegator-reverse-index rules: strict decode,
// stable filter BlockNum <= target (order preserved — exact per the
// append-only dvl semantics, §2.1), retained-pointer validation, and the
// in-memory reverse map for the completeness pass.
func (n *normalizer) dvl() error {
	return strictdb.ForEach(n.s.Raw, prefixDVL, func(key, value []byte) error {
		if len(key) != len(prefixDVL)+20 {
			n.counts.DVL.Invalid++
			n.fatal(report.ClassInvalidRetained, "dvl-malformed-key", key,
				fmt.Sprintf("dvl key length %d, want %d", len(key), len(prefixDVL)+20))
			return nil
		}
		delegator := common.BytesToAddress(key[len(prefixDVL):])

		var indexes staking.DelegationIndexes
		if err := rlp.DecodeBytes(value, &indexes); err != nil {
			n.counts.DVL.Invalid++
			n.fatal(report.ClassInvalidRetained, "dvl-undecodable", key,
				fmt.Sprintf("dvl record for %s does not strict-decode: %v", delegator.Hex(), err))
			return nil
		}

		var kept staking.DelegationIndexes
		seenValidator := map[common.Address]bool{}
		for _, idx := range indexes {
			if idx.BlockNum == nil || !idx.BlockNum.IsUint64() {
				n.counts.DVL.Invalid++
				n.fatal(report.ClassInvalidRetained, "dvl-malformed-blocknum", key,
					fmt.Sprintf("delegation index for %s -> %s has malformed BlockNum %v",
						delegator.Hex(), idx.ValidatorAddress.Hex(), idx.BlockNum))
				continue
			}
			if idx.BlockNum.Uint64() > n.a.TargetHeight {
				// Stable filter: post-target appends drop, order preserved.
				n.dvlEntriesRemoved++
				n.removedDVL = append(n.removedDVL, RemovedDVLEntry{
					Delegator: delegator,
					Validator: idx.ValidatorAddress,
					Index:     idx.Index,
					BlockNum:  idx.BlockNum.Uint64(),
				})
				continue
			}
			if seenValidator[idx.ValidatorAddress] {
				// Impossible per addDelegationIndex (§2.1); implies corruption.
				n.counts.DVL.Duplicate++
				n.fatal(report.ClassInvalidRetained, "dvl-duplicate-validator-entry", key,
					fmt.Sprintf("delegator %s holds two retained entries for validator %s",
						delegator.Hex(), idx.ValidatorAddress.Hex()))
				continue
			}
			seenValidator[idx.ValidatorAddress] = true
			n.validateRetainedIndex(key, delegator, idx)
			kept = append(kept, idx)
			vm, ok := n.retained[delegator]
			if !ok {
				vm = map[common.Address]uint64{}
				n.retained[delegator] = vm
			}
			vm[idx.ValidatorAddress] = idx.Index
			// Rough footprint: two addresses + index + map overhead.
			n.inventory.DVLReverseMapBytes += 20 + 20 + 8 + 48
		}

		if len(kept) == 0 {
			// Empty post-filter list: logical delete of the key.
			n.counts.DVL.Removed++
			n.dvlDeletions = append(n.dvlDeletions, PlannedDeletion{
				Key: hexKey(key), Reason: "post-target-only",
			})
			return nil
		}
		newValue := mustRLP(kept)
		n.counts.DVL.Retained++
		n.set.DVL = append(n.set.DVL, Record{
			Key: append([]byte(nil), key...), Value: newValue,
		})
		if !bytes.Equal(newValue, value) {
			n.dvlRewrites = append(n.dvlRewrites, PlannedRewrite{
				Key:            hexKey(key),
				NewValueSHA256: report.SHA256Hex(newValue),
				Reason:         "post-target-entries-filtered",
			})
		}
		return nil
	})
}

// validateRetainedIndex checks one retained pointer against the target
// wrappers (§4.4): validator listed, wrapper exists, Index in range, and
// the pointed-at Delegation belongs to the key's delegator.
func (n *normalizer) validateRetainedIndex(key []byte, delegator common.Address, idx staking.DelegationIndex) {
	wIdx, listed := n.wrapperByAddr[idx.ValidatorAddress]
	if !listed {
		n.counts.DVL.Invalid++
		n.fatal(report.ClassInvalidRetained, "dvl-validator-not-listed", key,
			fmt.Sprintf("retained index %s -> %s references a validator outside the normalized list",
				delegator.Hex(), idx.ValidatorAddress.Hex()))
		return
	}
	w := n.wrappers[wIdx].wrapper
	if w == nil {
		n.counts.DVL.Invalid++
		n.fatal(report.ClassInvalidRetained, "dvl-validator-wrapper-unavailable", key,
			fmt.Sprintf("retained index %s -> %s has no decodable target wrapper",
				delegator.Hex(), idx.ValidatorAddress.Hex()))
		return
	}
	if idx.Index >= uint64(len(w.Delegations)) {
		n.counts.DVL.Invalid++
		n.fatal(report.ClassInvalidRetained, "dvl-index-out-of-range", key,
			fmt.Sprintf("retained index %s -> %s slot %d, wrapper holds %d delegations",
				delegator.Hex(), idx.ValidatorAddress.Hex(), idx.Index, len(w.Delegations)))
		return
	}
	if w.Delegations[idx.Index].DelegatorAddress != delegator {
		n.counts.DVL.Invalid++
		n.fatal(report.ClassInvalidRetained, "dvl-pointer-mismatch", key,
			fmt.Sprintf("retained index %s -> %s slot %d points at delegator %s",
				delegator.Hex(), idx.ValidatorAddress.Hex(), idx.Index,
				w.Delegations[idx.Index].DelegatorAddress.Hex()))
	}
}

// dvlReverseCompleteness runs the §4.4 reverse pass (sound per §2.1: no
// pruning exists at this base; Undelegate/CollectRewards never remove
// Delegation entries): for every listed wrapper and every Delegations[i], a
// retained dvl entry {delegator -> (validator, i, BlockNum <= target)} must
// exist.
func (n *normalizer) dvlReverseCompleteness() {
	for i := range n.wrappers {
		entry := &n.wrappers[i]
		if entry.wrapper == nil {
			continue // already Fatal via the list rules
		}
		for di := range entry.wrapper.Delegations {
			delegator := entry.wrapper.Delegations[di].DelegatorAddress
			vm := n.retained[delegator]
			idx, ok := uint64(0), false
			if vm != nil {
				idx, ok = vm[entry.addr]
			}
			if !ok {
				n.counts.DVL.Missing++
				n.fatal(report.ClassMissingRequired, "dvl-missing-required-index", dvlKey(delegator),
					fmt.Sprintf("delegation %s -> %s (slot %d) has no retained dvl entry (clean-DB fallback signal)",
						delegator.Hex(), entry.addr.Hex(), di))
				continue
			}
			if idx != uint64(di) {
				n.counts.DVL.Invalid++
				n.fatal(report.ClassInvalidRetained, "dvl-index-slot-mismatch", dvlKey(delegator),
					fmt.Sprintf("delegation %s -> %s: wrapper slot %d, retained dvl entry says %d",
						delegator.Hex(), entry.addr.Hex(), di, idx))
			}
		}
	}
	sortRecords(n.set.DVL)
}
