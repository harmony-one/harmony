package node

import (
	"sync"
	"time"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var CrosslinkOutdatedErr = errors.New("crosslink signal is outdated")

const (
	maxPendingCrossLinkSize = 1000
	crossLinkBatchSize      = 3
	maxCrossLinkRetries     = 3
	retryDelay              = 5 * time.Second
)

// crossLinkRetryTracker tracks retry attempts for cross-links
type crossLinkRetryTracker struct {
	mu sync.RWMutex
	// failedCrossLinks tracks cross-links that failed verification
	failedCrossLinks map[string]*retryRecord
}

type retryRecord struct {
	crossLink  *types.CrossLink
	retryCount int
	lastRetry  time.Time
}

var globalRetryTracker = &crossLinkRetryTracker{
	failedCrossLinks: make(map[string]*retryRecord),
}

func (t *crossLinkRetryTracker) recordFailure(cl *types.CrossLink) bool {
	key := cl.Hash().Hex()

	t.mu.Lock()
	defer t.mu.Unlock()

	record, exists := t.failedCrossLinks[key]
	if !exists {
		record = &retryRecord{
			crossLink:  cl,
			retryCount: 1,
			lastRetry:  time.Now(),
		}
		t.failedCrossLinks[key] = record
		return true // Allow retry
	}

	record.retryCount++
	record.lastRetry = time.Now()

	// If max retries exceeded, mark for deletion
	if record.retryCount >= maxCrossLinkRetries {
		utils.Logger().Warn().
			Str("crossLinkHash", key).
			Int("retryCount", record.retryCount).
			Uint64("epoch", cl.Epoch().Uint64()).
			Uint32("shardID", cl.ShardID()).
			Msg("[CrossLink] Max retries exceeded - marking for deletion")
		return false // Don't allow more retries
	}

	return true // Allow retry
}

func (t *crossLinkRetryTracker) cleanupFailed(cl *types.CrossLink) {
	key := cl.Hash().Hex()

	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.failedCrossLinks, key)
}

func (t *crossLinkRetryTracker) getRetryCount(cl *types.CrossLink) int {
	key := cl.Hash().Hex()

	t.mu.RLock()
	defer t.mu.RUnlock()

	if record, exists := t.failedCrossLinks[key]; exists {
		return record.retryCount
	}
	return 0
}

// ProcessCrossLinkHeartbeatMessage process crosslink heart beat signal.
// This function is only called on shards 1,2,3 when network message `CrosslinkHeartbeat` receiving.
func (node *Node) ProcessCrossLinkHeartbeatMessage(msgPayload []byte) {
	if err := node.processCrossLinkHeartbeatMessage(msgPayload); err != nil {
		utils.Logger().Err(err).
			Msg("[ProcessCrossLinkHeartbeatMessage] failed process crosslink heartbeat signal")
	}
}

func (node *Node) processEpochBlockMessage(msgPayload []byte) error {
	if node.IsRunningBeaconChain() {
		return errors.New("received beacon block for beacon chain")
	}
	block, err := core.RlpDecodeBlockOrBlockWithSig(msgPayload)
	if err != nil {
		return errors.WithMessage(err, "failed to decode block")
	}
	if _, err := node.EpochChain().InsertChain(types.Blocks{block}, true); err != nil {
		return errors.WithMessage(err, "failed insert epoch block")
	}
	return nil
}

func (node *Node) ProcessEpochBlockMessage(msgPayload []byte) {
	if err := node.processEpochBlockMessage(msgPayload); err != nil {
		utils.Logger().Err(err).
			Msg("[ProcessEpochBlock] failed process epoch block")
	}
}

func (node *Node) processCrossLinkHeartbeatMessage(msgPayload []byte) error {
	hb := types.CrosslinkHeartbeat{}
	err := rlp.DecodeBytes(msgPayload, &hb)
	if err != nil {
		return errors.WithMessagef(err, "cannot decode crosslink heartbeat message, len: %d", len(msgPayload))
	}
	cur := node.Blockchain().CurrentBlock()
	shardID := cur.ShardID()
	if hb.ShardID != shardID {
		return errors.Errorf("invalid shard id: expected %d, got %d", shardID, hb.ShardID)
	}

	// Outdated signal.
	if s := node.crosslinks.LastKnownCrosslinkHeartbeatSignal(); s != nil && s.LatestContinuousBlockNum > hb.LatestContinuousBlockNum {
		return errors.WithMessagef(CrosslinkOutdatedErr, "latest continuous block num: %d, got %d", s.LatestContinuousBlockNum, hb.LatestContinuousBlockNum)
	}

	sig := &ffi_bls.Sign{}
	err = sig.Deserialize(hb.Signature)
	if err != nil {
		return errors.WithMessagef(err, "cannot deserialize signature, len: %d", len(hb.Signature))
	}

	hb.Signature = nil
	serialized, err := rlp.EncodeToBytes(hb)
	if err != nil {
		return errors.WithMessage(err, "cannot serialize crosslink heartbeat message")
	}

	pub := ffi_bls.PublicKey{}
	err = pub.Deserialize(hb.PublicKey)
	if err != nil {
		return errors.WithMessagef(err, "cannot deserialize public key, len: %d", len(hb.PublicKey))
	}

	ok := sig.VerifyHash(&pub, serialized)
	if !ok {
		return errors.New("invalid signature")
	}

	state, err := node.EpochChain().ReadShardState(cur.Epoch())
	if err != nil {
		return errors.WithMessagef(err, "cannot read shard state for epoch %d", cur.Epoch())
	}
	committee, err := state.FindCommitteeByID(shard.BeaconChainShardID)
	if err != nil {
		return errors.WithMessagef(err, "cannot find committee for shard %d", shard.BeaconChainShardID)
	}
	pubs, err := committee.BLSPublicKeys()
	if err != nil {
		return errors.WithMessage(err, "cannot get BLS public keys")
	}

	keyExists := false
	for _, row := range pubs {
		if pub.IsEqual(row.Object) {
			keyExists = true
			break
		}
	}

	if !keyExists {
		return errors.Errorf("pub key %s not found in committiee for epoch %d and shard %d, my current shard is %d, pub keys len %d", pub.SerializeToHexStr(), hb.Epoch, shard.BeaconChainShardID, shardID, len(pubs))
	}

	utils.Logger().Info().
		Msgf("[ProcessCrossLinkHeartbeatMessage] storing hb signal with block num %d", hb.LatestContinuousBlockNum)
	node.crosslinks.SetLastKnownCrosslinkHeartbeatSignal(&hb)
	return nil
}

// ProcessCrossLinkMessage verify and process Node/CrossLink message into crosslink when it's valid
func (node *Node) ProcessCrossLinkMessage(msgPayload []byte) {
	if node.IsRunningBeaconChain() {
		pendingCLs, err := node.Blockchain().ReadPendingCrossLinks()
		if err == nil && len(pendingCLs) >= maxPendingCrossLinkSize {
			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] Pending Crosslink reach maximum size: %d", len(pendingCLs))
			return
		}
		if err != nil {
			utils.Logger().Debug().
				Err(err).
				Int("num crosslinks", len(pendingCLs)).
				Msg("[ProcessingCrossLink] Read Pending Crosslink failed")
		}

		existingCLs := map[common2.Hash]struct{}{}
		for _, pending := range pendingCLs {
			existingCLs[pending.Hash()] = struct{}{}
		}

		var crosslinks []types.CrossLink
		if err := rlp.DecodeBytes(msgPayload, &crosslinks); err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("[ProcessingCrossLink] Crosslink Message Broadcast Unable to Decode")
			return
		}

		var candidates []types.CrossLink
		var failedCrossLinks []types.CrossLink

		utils.Logger().Debug().
			Msgf("[ProcessingCrossLink] Received crosslinks: %d", len(crosslinks))

		for i, cl := range crosslinks {
			// limit processing to prevent spam
			if i > crossLinkBatchSize*2 { // A sanity check to prevent spamming
				utils.Logger().Warn().
					Int("processed", i).
					Int("total", len(crosslinks)).
					Int("limit", crossLinkBatchSize*2).
					Msg("[ProcessingCrossLink] Batch size limit reached, stopping processing")
				break
			}

			// Check if node is synced enough to handle this cross-link
			localEpoch := node.Blockchain().CurrentBlock().Header().Epoch().Uint64()
			crossLinkEpoch := cl.Epoch().Uint64()

			// Allow processing cross-links from current epoch and up to 1 epoch ahead
			// This provides some tolerance for minor sync delays while preventing processing of far-future cross-links
			maxAllowedEpoch := localEpoch + 1

			if crossLinkEpoch > maxAllowedEpoch {
				utils.Logger().Debug().
					Str("crossLinkHash", cl.Hash().Hex()).
					Uint64("crossLinkEpoch", crossLinkEpoch).
					Uint64("localEpoch", localEpoch).
					Uint64("maxAllowedEpoch", maxAllowedEpoch).
					Uint32("shardID", cl.ShardID()).
					Msg("[ProcessingCrossLink] Skipping cross-link - node not synced to this epoch yet (epoch gap too large)")
				// Add to failed list to be deleted since we can't process it
				failedCrossLinks = append(failedCrossLinks, cl)
				continue
			}

			// Check if we should retry this cross-link
			if !globalRetryTracker.recordFailure(&cl) {
				utils.Logger().Warn().
					Str("crossLinkHash", cl.Hash().Hex()).
					Uint64("epoch", cl.Epoch().Uint64()).
					Uint32("shardID", cl.ShardID()).
					Msg("[ProcessingCrossLink] Skipping cross-link after max retries - will be deleted")
				// Add to failed list to be deleted
				failedCrossLinks = append(failedCrossLinks, cl)
				continue
			}

			// Try to verify the cross-link
			if err := node.Blockchain().Engine().VerifyCrossLink(node.Blockchain(), cl); err != nil {
				utils.Logger().Error().
					Err(err).
					Str("crossLinkHash", cl.Hash().Hex()).
					Uint64("epoch", cl.Epoch().Uint64()).
					Uint32("shardID", cl.ShardID()).
					Int("retryCount", globalRetryTracker.getRetryCount(&cl)).
					Msg("[ProcessingCrossLink] Failed to verify cross-link - will retry")

				// Sleep before retry to avoid hammering the system
				time.Sleep(retryDelay)
				continue
			}

			// Success! Clean up the retry tracker (if it has been retried)
			globalRetryTracker.cleanupFailed(&cl)

			// Check if cross-link already exists
			if _, exists := existingCLs[cl.Hash()]; exists {
				utils.Logger().Debug().
					Str("crossLinkHash", cl.Hash().Hex()).
					Msg("[ProcessingCrossLink] Cross-link already exists, skipping")
				continue
			}

			// Add to candidates for processing
			candidates = append(candidates, cl)
		}

		// Log summary of processing results
		utils.Logger().Info().
			Int("totalCrossLinks", len(crosslinks)).
			Int("processedCrossLinks", len(candidates)).
			Int("failedCrossLinks", len(failedCrossLinks)).
			Int("filteredBySyncStatus", len(crosslinks)-len(candidates)-len(failedCrossLinks)).
			Msg("[ProcessingCrossLink] Cross-link processing summary")

		// Delete failed cross-links from pending queue
		if len(failedCrossLinks) > 0 {
			if _, err := node.Blockchain().DeleteFromPendingCrossLinks(failedCrossLinks); err != nil {
				utils.Logger().Error().
					Err(err).
					Int("failedCount", len(failedCrossLinks)).
					Msg("[ProcessingCrossLink] Failed to delete failed cross-links from pending queue")
			} else {
				utils.Logger().Info().
					Int("deletedCount", len(failedCrossLinks)).
					Msg("[ProcessingCrossLink] Successfully deleted failed cross-links from pending queue")
			}
		}

		// Process valid cross-links by adding them to pending queue
		if len(candidates) > 0 {
			utils.Logger().Info().
				Int("count", len(candidates)).
				Msg("[ProcessingCrossLink] Processing valid cross-links")

			// Add to pending cross-links queue
			if _, err := node.Blockchain().AddPendingCrossLinks(candidates); err != nil {
				utils.Logger().Error().
					Err(err).
					Int("count", len(candidates)).
					Msg("[ProcessingCrossLink] Failed to add cross-links to pending queue")
			}
		}
	}
}

// VerifyCrossLink verifies the header is valid
func (node *Node) VerifyCrossLink(cl types.CrossLink) error {
	if node.Blockchain().ShardID() != shard.BeaconChainShardID {
		return errors.New("[VerifyCrossLink] Shard chains should not verify cross links")
	}
	instance := shard.Schedule.InstanceForEpoch(node.Blockchain().CurrentHeader().Epoch())
	if cl.ShardID() >= instance.NumShards() {
		return errors.New("[VerifyCrossLink] ShardID should less than NumShards")
	}
	engine := node.Blockchain().Engine()

	if err := engine.VerifyCrossLink(node.Blockchain(), cl); err != nil {
		return errors.Wrap(err, "[VerifyCrossLink]")
	}
	return nil
}
