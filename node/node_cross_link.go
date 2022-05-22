package node

import (
	"fmt"
	"math/big"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	maxPendingCrossLinkSize = 1000
	crossLinkBatchSize      = 3
)

var (
	errAlreadyExist = errors.New("crosslink already exist")
)

// VerifyBlockCrossLinks verifies the cross links of the block
func (node *Node) VerifyBlockCrossLinks(block *types.Block) error {
	cxLinksData := block.Header().CrossLinks()
	if len(cxLinksData) == 0 {
		utils.Logger().Debug().Msgf("[CrossLinkVerification] Zero CrossLinks in the header")
		return nil
	}

	crossLinks := types.CrossLinks{}
	err := rlp.DecodeBytes(cxLinksData, &crossLinks)
	if err != nil {
		return errors.Wrapf(
			err, "[CrossLinkVerification] failed to decode cross links",
		)
	}

	if !crossLinks.IsSorted() {
		return errors.New("[CrossLinkVerification] cross links are not sorted")
	}

	for _, crossLink := range crossLinks {
		cl, err := node.Blockchain().ReadCrossLink(crossLink.ShardID(), crossLink.BlockNum())
		if err == nil && cl != nil {
			// Add slash for exist same blocknum but different crosslink
			return errAlreadyExist
		}
		if err := node.VerifyCrossLink(crossLink); err != nil {
			return errors.Wrapf(err, "cannot VerifyBlockCrossLinks")

		}
	}
	return nil
}

// ProcessCrossLinkHeartbeatMessage process crosslink heart beat signal.
func (node *Node) ProcessCrossLinkHeartbeatMessage(msgPayload []byte) {
	if node.IsRunningBeaconChain() {
		return
	}

	if err := node.processCrossLinkHeartbeatMessage(msgPayload); err != nil {
		fmt.Println("Err: ", err)
	}
}

func (node *Node) processCrossLinkHeartbeatMessage(msgPayload []byte) error {
	fmt.Println("ProcessCrossLinkHeartbeatMessage==")
	hb := types.CrosslinkHeartbeat{}
	err := rlp.DecodeBytes(msgPayload, &hb)
	if err != nil {
		return err
	}
	shardID := node.Blockchain().CurrentBlock().ShardID()
	if hb.ShardID != shardID {
		return errors.Errorf("invalid shard id: expected %d, got %d", shardID, hb.ShardID)
	}

	// Outdated signal.
	fmt.Printf("ProcessCrossLinkHeartbeatMessage?? %d %d\n", hb.BlockID, latestSentCrosslink.Get(hb.ShardID))
	if hb.BlockID <= latestSentCrosslink.Get(hb.ShardID) {
		return nil
	}

	fmt.Println("ProcessCrossLinkHeartbeatMessage==22")
	sig := &ffi_bls.Sign{}
	err = sig.Deserialize(hb.Signature)
	if err != nil {
		return err
	}

	hb.Signature = nil
	serialized, err := rlp.EncodeToBytes(hb)
	if err != nil {
		return err
	}

	pub := ffi_bls.PublicKey{}
	err = pub.Deserialize(hb.PublicKey)
	if err != nil {
		return err
	}

	fmt.Println("ProcessCrossLinkHeartbeatMessage==333")
	ok := sig.VerifyHash(&pub, serialized)
	if !ok {
		return errors.New("invalid signature")
	}

	state, err := node.EpochChain().ReadShardState(big.NewInt(int64(hb.Epoch)))
	if err != nil {
		return err
	}

	committee, err := state.FindCommitteeByID(shard.BeaconChainShardID)
	if err != nil {
		return err
	}
	pubs, err := committee.BLSPublicKeys()
	if err != nil {
		return err
	}

	fmt.Println("ProcessCrossLinkHeartbeatMessage==444")
	keyExists := false
	for _, row := range pubs {
		if pub.IsEqual(row.Object) {
			keyExists = true
			break
		}
	}

	if !keyExists {
		return errors.New("pub key doesn't exist")
	}

	fmt.Println("ProcessCrossLinkHeartbeatMessage==5555")
	latestSentCrosslink.Set(hb.ShardID, hb.BlockID)

	fmt.Printf("Set: %d %d\n", hb.ShardID, hb.BlockID)
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

			exist, err := node.Blockchain().ReadCrossLink(cl.ShardID(), cl.Number().Uint64())
			if err == nil && exist != nil {
				nodeCrossLinkMessageCounterVec.With(prometheus.Labels{"type": "duplicate_crosslink"}).Inc()
				utils.Logger().Debug().Err(err).
					Msgf("[ProcessingCrossLink] Cross Link already exists, pass. Beacon Epoch: %d, Block num: %d, Epoch: %d, shardID %d", node.Blockchain().CurrentHeader().Epoch(), cl.Number(), cl.Epoch(), cl.ShardID())
				continue
			}

			if err = node.VerifyCrossLink(cl); err != nil {
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
	/*
		else {
			var crosslinks []types.CrossLink
			if err := rlp.DecodeBytes(msgPayload, &crosslinks); err != nil {
				utils.Logger().Error().
					Err(err).
					Msg("[ProcessingCrossLink] Crosslink Message Broadcast Unable to Decode")
				return
			}
			shardID := node.Blockchain().ShardID()
			lastCl, _ := node.Blockchain().ReadShardLastCrossLink(shardID)

			clToWrite := lastCl
			for _, crosslink := range crosslinks {
				if crosslink.ShardID() != shardID {
					continue
				}

				err := node.EpochChain().Engine().VerifyCrossLink(node.EpochChain(), crosslink)
				if err != nil {
					nodeCrossLinkMessageCounterVec.With(prometheus.Labels{"type": "invalid_crosslink"}).Inc()
					utils.Logger().Info().
						Str("cross-link-issue", err.Error()).
						Msgf("[ProcessingCrossLink] Failed to verify "+
							"new cross link for blockNum %d epochNum %d shard %d skipped: %v",
							crosslink.BlockNum(), crosslink.Epoch().Uint64(), crosslink.ShardID(), crosslink)
					continue
				}

				if clToWrite == nil {
					clToWrite = &crosslink
				} else {
					if crosslink.BlockNum() > clToWrite.BlockNum() {
						clToWrite = &crosslink
					}
				}
			}
			if clToWrite != nil && clToWrite != lastCl {
				err := rawdb.WriteShardLastCrossLink(node.Blockchain().ChainDb(), shardID, clToWrite.Serialize())
				if err != nil {
					utils.Logger().Info().
						Str("cross-link-issue", err.Error()).
						Msgf("[ProcessingCrossLink] Failed to save crosslink: "+
							"blockNum %d epochNum %d shard %d skipped: %v",
							clToWrite.BlockNum(), clToWrite.Epoch().Uint64(), clToWrite.ShardID(), clToWrite)
				}
			}
		}
	*/
}

// VerifyCrossLink verifies the header is valid
func (node *Node) VerifyCrossLink(cl types.CrossLink) error {
	if node.Blockchain().ShardID() != shard.BeaconChainShardID {
		return errors.New("[VerifyCrossLink] Shard chains should not verify cross links")
	}
	engine := node.Blockchain().Engine()

	if err := engine.VerifyCrossLink(node.Blockchain(), cl); err != nil {
		return errors.Wrap(err, "[VerifyCrossLink]")
	}
	return nil
}
