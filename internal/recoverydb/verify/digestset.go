package verify

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
)

// OffchainDigests is the off-chain half of the DigestSet.
type OffchainDigests struct {
	CXSpent            report.Digest
	CXOutgoingWindow   report.Digest
	CrosslinkIndex     report.Digest
	CrosslinkShardLast report.Digest
	ValidatorList      report.Digest
	Delegations        report.Digest
	ValidatorSnapshots report.Digest
	ShardStates        report.Digest
	EpochBlockNumbers  report.Digest
	EpochVrf           report.Digest
	EpochVdf           report.Digest
	RewardAccumulators report.Digest
}

// ComputeOffchainDigests runs the strict-iterator digest passes over the
// off-chain namespaces (plan WS2 --full-offchain-check, WS4 step 9). Every
// key under a scanned prefix is classified; keys that physically belong to
// the bare-hash32 keyspace (e.g. a trie node whose hash begins with "cl")
// are skipped, keys of unexpected shape are fatal.
func ComputeOffchainDigests(db ethdb.Iteratee, window report.DigestWindow) (*OffchainDigests, error) {
	out := &OffchainDigests{}

	scan := func(prefix []byte, want map[string]*report.Hasher, filter func(bucket string, key []byte) (bool, error)) error {
		return strictdb.ForEach(db, prefix, func(key, value []byte) error {
			bucket := keys.Classify(key)
			if bucket == keys.BucketBareHash32 {
				return nil // physically a trie node / legacy code blob
			}
			h, ok := want[bucket]
			if !ok {
				return fmt.Errorf("verify: unexpected key %x (bucket %s) under prefix %q", key, bucket, prefix)
			}
			if filter != nil {
				keep, err := filter(bucket, key)
				if err != nil {
					return err
				}
				if !keep {
					return nil
				}
			}
			h.Add(key, value)
			return nil
		})
	}

	// cxReceiptSpent (full set, never windowed — plan §10.5).
	cxSpentH := report.NewHasher("offchain.cxSpent")
	if err := scan(keys.CxSpentPrefix, map[string]*report.Hasher{keys.BucketCxSpent: cxSpentH}, nil); err != nil {
		return nil, err
	}
	out.CXSpent = cxSpentH.Digest()

	// Outgoing cxReceipt records, window-scoped by source block number.
	cxOutH := report.NewHasher("offchain.cxOutgoingWindow")
	if err := scan(keys.CxReceiptPrefix, map[string]*report.Hasher{keys.BucketCxReceipt: cxOutH},
		func(bucket string, key []byte) (bool, error) {
			payload := key[len(keys.CxReceiptPrefix):]
			num := binary.BigEndian.Uint64(payload[4:12])
			return num >= window.RetainFrom && num <= window.Target, nil
		}); err != nil {
		return nil, err
	}
	out.CXOutgoingWindow = cxOutH.Digest()

	// Crosslink index + per-shard last values share the "cl" prefix.
	clIdxH := report.NewHasher("offchain.crosslinkIndex")
	clLastH := report.NewHasher("offchain.crosslinkShardLast")
	if err := scan(keys.CrosslinkPrefix, map[string]*report.Hasher{
		keys.BucketCrosslinkIndex:     clIdxH,
		keys.BucketCrosslinkShardLast: clLastH,
	}, nil); err != nil {
		return nil, err
	}
	out.CrosslinkIndex = clIdxH.Digest()
	out.CrosslinkShardLast = clLastH.Digest()

	// Validator list (single key).
	vlH := report.NewHasher("offchain.validatorList")
	if err := scan(keys.ValidatorListKey, map[string]*report.Hasher{keys.BucketValidatorList: vlH}, nil); err != nil {
		return nil, err
	}
	out.ValidatorList = vlH.Digest()

	// Delegator -> validator list indexes.
	dvlH := report.NewHasher("offchain.delegations")
	if err := scan(keys.DVLPrefix, map[string]*report.Hasher{keys.BucketDVL: dvlH}, nil); err != nil {
		return nil, err
	}
	out.Delegations = dvlH.Digest()

	// Validator snapshots. NOTE: the "validator-s" iteration prefix would
	// also catch validator-stats; scan the exact snapshot prefix and let the
	// classifier reject anything else.
	vsH := report.NewHasher("offchain.validatorSnapshots")
	if err := scan(keys.ValidatorSnapshotPrefix, map[string]*report.Hasher{keys.BucketValidatorSnapshot: vsH}, nil); err != nil {
		return nil, err
	}
	out.ValidatorSnapshots = vsH.Digest()

	// Shard states.
	ssH := report.NewHasher("offchain.shardStates")
	if err := scan(keys.ShardStatePrefix, map[string]*report.Hasher{keys.BucketShardState: ssH}, nil); err != nil {
		return nil, err
	}
	out.ShardStates = ssH.Digest()

	// Epoch block number / VRF / VDF records.
	ebnH := report.NewHasher("offchain.epochBlockNumbers")
	if err := scan(keys.EpochBlockNumberPrefix, map[string]*report.Hasher{keys.BucketEpochBlockNumber: ebnH}, nil); err != nil {
		return nil, err
	}
	out.EpochBlockNumbers = ebnH.Digest()

	vrfH := report.NewHasher("offchain.epochVrf")
	if err := scan(keys.EpochVrfPrefix, map[string]*report.Hasher{keys.BucketEpochVrf: vrfH}, nil); err != nil {
		return nil, err
	}
	out.EpochVrf = vrfH.Digest()

	vdfH := report.NewHasher("offchain.epochVdf")
	if err := scan(keys.EpochVdfPrefix, map[string]*report.Hasher{keys.BucketEpochVdf: vdfH}, nil); err != nil {
		return nil, err
	}
	out.EpochVdf = vdfH.Digest()

	// Block reward accumulators over the retained window only.
	rwdH := report.NewHasher("offchain.rewardAccumulators")
	if err := scan(keys.RewardAccumPrefix, map[string]*report.Hasher{keys.BucketRewardAccum: rwdH},
		func(bucket string, key []byte) (bool, error) {
			num := binary.BigEndian.Uint64(key[len(keys.RewardAccumPrefix):])
			return num >= window.RetainFrom && num <= window.Target, nil
		}); err != nil {
		return nil, err
	}
	out.RewardAccumulators = rwdH.Digest()

	return out, nil
}

// BuildDigestSet assembles the full DigestSet from a state walk result and
// off-chain digests.
func BuildDigestSet(
	network string, shardID uint32,
	targetHeight uint64, targetHash, stateRoot common.Hash,
	window report.DigestWindow,
	state *StateWalkResult, off *OffchainDigests,
) *report.DigestSet {
	return &report.DigestSet{
		SchemaVersion: report.DigestSetSchemaV1,
		Network:       network,
		ShardID:       shardID,
		TargetHeight:  targetHeight,
		TargetHash:    targetHash.Hex(),
		StateRoot:     stateRoot.Hex(),
		Window:        window,

		Accounts:     state.Accounts,
		StorageSlots: state.StorageSlots,
		Codes:        state.Codes,

		CXSpent:            off.CXSpent,
		CXOutgoingWindow:   off.CXOutgoingWindow,
		CrosslinkIndex:     off.CrosslinkIndex,
		CrosslinkShardLast: off.CrosslinkShardLast,
		ValidatorList:      off.ValidatorList,
		Delegations:        off.Delegations,
		ValidatorSnapshots: off.ValidatorSnapshots,
		ShardStates:        off.ShardStates,
		EpochBlockNumbers:  off.EpochBlockNumbers,
		EpochVrf:           off.EpochVrf,
		EpochVdf:           off.EpochVdf,
		RewardAccumulators: off.RewardAccumulators,
	}
}

var _ = ethdb.Database(nil)
