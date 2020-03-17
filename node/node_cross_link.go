package node

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/verify"
)

const (
	maxPendingCrossLinkSize = 1000
	crossLinkBatchSize      = 10
)

// VerifyBlockCrossLinks verifies the cross links of the block
func (node *Node) VerifyBlockCrossLinks(block *types.Block) error {
	if len(block.Header().CrossLinks()) == 0 {
		utils.Logger().Debug().Msgf("[CrossLinkVerification] Zero CrossLinks in the header")
		return nil
	}

	crossLinks := &types.CrossLinks{}
	err := rlp.DecodeBytes(block.Header().CrossLinks(), crossLinks)
	if err != nil {
		return ctxerror.New("[CrossLinkVerification] failed to decode cross links",
			"blockHash", block.Hash(),
			"crossLinks", len(block.Header().CrossLinks()),
		).WithCause(err)
	}

	if !crossLinks.IsSorted() {
		return ctxerror.New("[CrossLinkVerification] cross links are not sorted",
			"blockHash", block.Hash(),
			"crossLinks", len(block.Header().CrossLinks()),
		)
	}

	for _, crossLink := range *crossLinks {
		cl, err := node.Blockchain().ReadCrossLink(crossLink.ShardID(), crossLink.BlockNum())
		if err == nil && cl != nil {
			// Add slash for exist same blocknum but different crosslink
			return ctxerror.New("crosslink already exist!")
		}
		if err = node.VerifyCrossLink(crossLink); err != nil {
			return ctxerror.New("cannot VerifyBlockCrossLinks",
				"blockHash", block.Hash(),
				"blockNum", block.Number(),
				"crossLinkShard", crossLink.ShardID(),
				"crossLinkBlock", crossLink.BlockNum(),
				"numTx", len(block.Transactions()),
			).WithCause(err)
		}
	}
	return nil
}

// ProcessCrossLinkMessage verify and process Node/CrossLink message into crosslink when it's valid
func (node *Node) ProcessCrossLinkMessage(msgPayload []byte) {
	if node.NodeConfig.ShardID == shard.BeaconChainShardID {
		pendingCLs, err := node.Blockchain().ReadPendingCrossLinks()
		if err == nil && len(pendingCLs) >= maxPendingCrossLinkSize {
			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] Pending Crosslink reach maximum size: %d", len(pendingCLs))
			return
		}

		crosslinks := []types.CrossLink{}
		if err := rlp.DecodeBytes(msgPayload, &crosslinks); err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("[ProcessingCrossLink] Crosslink Message Broadcast Unable to Decode")
			return
		}

		candidates := []types.CrossLink{}
		utils.Logger().Debug().
			Msgf("[ProcessingCrossLink] Received crosslinks: %d", len(crosslinks))

		for i, cl := range crosslinks {
			if i > crossLinkBatchSize {
				break
			}
			exist, err := node.Blockchain().ReadCrossLink(cl.ShardID(), cl.Number().Uint64())
			if err == nil && exist != nil {
				utils.Logger().Err(err).
					Msgf("[ProcessingCrossLink] Cross Link already exists, pass. Beacon Epoch: %d, Block num: %d, Epoch: %d, shardID %d", node.Blockchain().CurrentHeader().Epoch(), cl.Number(), cl.Epoch(), cl.ShardID())
				continue
			}

			if err = node.VerifyCrossLink(cl); err != nil {
				utils.Logger().Info().
					Str("cross-link-issue", err.Error()).
					Msgf("[ProcessingCrossLink] Failed to verify new cross link for blockNum %d epochNum %d shard %d skipped: %v", cl.BlockNum(), cl.Epoch().Uint64(), cl.ShardID(), cl)
				continue
			}

			candidates = append(candidates, cl)
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
		return ctxerror.New("[VerifyCrossLink] Shard chains should not verify cross links")
	}

	if cl.BlockNum() <= 1 {
		return ctxerror.New("[VerifyCrossLink] CrossLink BlockNumber should greater than 1")
	}

	if !node.Blockchain().Config().IsCrossLink(cl.Epoch()) {
		return ctxerror.New(
			"[VerifyCrossLink] CrossLink Epoch should >= cross link starting epoch",
			"crossLinkEpoch", cl.Epoch(), "cross_link_starting_eoch",
			node.Blockchain().Config().CrossLinkEpoch,
		)
	}

	// Verify signature of the new cross link header
	// TODO: check whether to recalculate shard state
	shardState, err := node.Blockchain().ReadShardState(cl.Epoch())
	if err != nil {
		return err
	}

	committee, err := shardState.FindCommitteeByID(cl.ShardID())

	if err != nil {
		return err
	}

	aggSig := &bls.Sign{}
	sig := cl.Signature()
	if err = aggSig.Deserialize(sig[:]); err != nil {
		return ctxerror.New(
			"[VerifyCrossLink] unable to deserialize multi-signature from payload",
		).WithCause(err)
	}

	return verify.AggregateSigForCommittee(
		committee, aggSig, cl.Hash(), cl.BlockNum(), cl.Epoch(), cl.Bitmap(),
	)
}
