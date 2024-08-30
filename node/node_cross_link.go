package node

import (
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var CrosslinkOutdatedErr = errors.New("crosslink signal is outdated")

const (
	maxPendingCrossLinkSize = 1000
	crossLinkBatchSize      = 3
)

// ProcessCrossLinkHeartbeatMessage process crosslink heart beat signal.
// This function is only called on shards 1,2,3 when network message `CrosslinkHeartbeat` receiving.
func (node *Node) ProcessCrossLinkHeartbeatMessage(msgPayload []byte) {
	if err := node.processCrossLinkHeartbeatMessage(msgPayload); err != nil {
		utils.Logger().Err(err).
			Msg("[ProcessCrossLinkHeartbeatMessage] failed process crosslink heartbeat signal")
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
		if err != nil {
			utils.Logger().Debug().
				Err(err).
				Msgf("[ProcessingCrossLink] Pending Crosslink reach maximum size: %d", len(pendingCLs))
			return
		}
		if len(pendingCLs) >= maxPendingCrossLinkSize {
			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] Pending Crosslink reach maximum size: %d", len(pendingCLs))
			return
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
		utils.Logger().Debug().
			Msgf("[ProcessingCrossLink] Received crosslinks: %d", len(crosslinks))

		for i, cl := range crosslinks {
			if i > crossLinkBatchSize*2 { // A sanity check to prevent spamming
				break
			}

			if _, ok := existingCLs[cl.Hash()]; ok {
				nodeCrossLinkMessageCounterVec.With(prometheus.Labels{"type": "duplicate_crosslink"}).Inc()
				utils.Logger().Debug().Err(err).
					Msgf("[ProcessingCrossLink] Cross Link already exists in pending queue, pass. Beacon Epoch: %d, Block num: %d, Epoch: %d, shardID %d",
						node.Blockchain().CurrentHeader().Epoch(), cl.Number(), cl.Epoch(), cl.ShardID())
				continue
			}

			// ReadCrossLink beacon chain usage.
			exist, err := node.Blockchain().ReadCrossLink(cl.ShardID(), cl.Number().Uint64())
			if err == nil && exist != nil {
				nodeCrossLinkMessageCounterVec.With(prometheus.Labels{"type": "duplicate_crosslink"}).Inc()
				utils.Logger().Debug().Err(err).
					Msgf("[ProcessingCrossLink] Cross Link already exists, pass. Beacon Epoch: %d, Block num: %d, Epoch: %d, shardID %d", node.Blockchain().CurrentHeader().Epoch(), cl.Number(), cl.Epoch(), cl.ShardID())
				continue
			}

			if err = core.VerifyCrossLink(node.Blockchain(), cl); err != nil {
				nodeCrossLinkMessageCounterVec.With(prometheus.Labels{"type": "invalid_crosslink"}).Inc()
				utils.Logger().Info().
					Str("cross-link-issue", err.Error()).
					Msgf("[ProcessingCrossLink] Failed to verify new cross link for blockNum %d epochNum %d shard %d skipped: %v", cl.BlockNum(), cl.Epoch().Uint64(), cl.ShardID(), cl)
				continue
			}

			candidates = append(candidates, cl)
			nodeCrossLinkMessageCounterVec.With(prometheus.Labels{"type": "new_crosslink"}).Inc()

			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] Committing for shardID %d, blockNum %d",
					cl.ShardID(), cl.Number().Uint64(),
				)
		}
		Len, _ := node.Blockchain().AddPendingCrossLinks(candidates)
		utils.Logger().Debug().
			Msgf("[ProcessingCrossLink] Add pending crosslinks,  total pending: %d", Len)
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
