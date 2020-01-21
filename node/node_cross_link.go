package node

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
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
	if node.NodeConfig.ShardID == 0 {
		pendingCLs, err := node.Blockchain().ReadPendingCrossLinks()
		if err == nil && len(pendingCLs) >= maxPendingCrossLinkSize {
			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] Pending Crosslink reach maximum size: %d", len(pendingCLs))
			return
		}

		var crosslinks []types.CrossLink
		err = rlp.DecodeBytes(msgPayload, &crosslinks)
		if err != nil {
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
				// TODO: leader add double sign checking
				utils.Logger().Err(err).
					Msgf("[ProcessingCrossLink] Cross Link already exists, pass. Beacon Epoch: %d, Block num: %d, Epoch: %d, shardID %d", node.Blockchain().CurrentHeader().Epoch(), cl.Number(), cl.Epoch(), cl.ShardID())
				continue
			}

			if err = node.VerifyCrossLink(cl); err != nil {
				utils.Logger().Err(err).
					Msgf("[ProcessingCrossLink] Failed to verify new cross link for blockNum %d epochNum %d shard %d skipped: %v", cl.BlockNum(), cl.Epoch().Uint64(), cl.ShardID(), cl)
				continue
			}

			candidates = append(candidates, cl)
			utils.Logger().Debug().
				Msgf("[ProcessingCrossLink] Committing for shardID %d, blockNum %d", cl.ShardID(), cl.Number().Uint64())
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
		return ctxerror.New("[VerifyCrossLink] CrossLink Epoch should >= cross link starting epoch", "crossLinkEpoch", cl.Epoch(), "cross_link_starting_eoch", node.Blockchain().Config().CrossLinkEpoch)
	}

	// Verify signature of the new cross link header
	// TODO: check whether to recalculate shard state
	shardState, err := node.Blockchain().ReadShardState(cl.Epoch())
	committee := shardState.FindCommitteeByID(cl.ShardID())

	if err != nil || committee == nil {
		return ctxerror.New("[VerifyCrossLink] Failed to read shard state for cross link", "beaconEpoch", node.Blockchain().CurrentHeader().Epoch(), "epoch", cl.Epoch(), "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
	}
	var committerKeys []*bls.PublicKey

	parseKeysSuccess := true
	for _, member := range committee.Slots {
		committerKey := new(bls.PublicKey)
		err = member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			parseKeysSuccess = false
			break
		}
		committerKeys = append(committerKeys, committerKey)
	}
	if !parseKeysSuccess {
		return ctxerror.New("[VerifyCrossLink] cannot convert BLS public key", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
	}

	mask, err := bls_cosi.NewMask(committerKeys, nil)
	if err != nil {
		return ctxerror.New("[VerifyCrossLink] cannot create group sig mask", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
	}
	if err := mask.SetMask(cl.Bitmap()); err != nil {
		return ctxerror.New("[VerifyCrossLink] cannot set group sig mask bits", "shardID", cl.ShardID(), "blockNum", cl.BlockNum()).WithCause(err)
	}

	decider := quorum.NewDecider(quorum.SuperMajorityStake)
	decider.SetShardIDProvider(func() (uint32, error) {
		return cl.ShardID(), nil
	})
	decider.SetMyPublicKeyProvider(func() (*bls.PublicKey, error) {
		return nil, nil
	})
	if _, err := decider.SetVoters(committee.Slots, false); err != nil {
		return ctxerror.New("[VerifyCrossLink] Cannot SetVoters for committee", "shardID", cl.ShardID())
	}
	if !decider.IsQuorumAchievedByMask(mask, false) {
		return ctxerror.New("[VerifyCrossLink] Not enough voting power for crosslink", "shardID", cl.ShardID())
	}

	aggSig := bls.Sign{}
	sig := cl.Signature()
	err = aggSig.Deserialize(sig[:])
	if err != nil {
		return ctxerror.New("[VerifyCrossLink] unable to deserialize multi-signature from payload").WithCause(err)
	}

	hash := cl.Hash()
	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, cl.BlockNum())
	commitPayload := append(blockNumBytes, hash[:]...)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		return ctxerror.New("[VerifyCrossLink] Failed to verify the signature for cross link", "shardID", cl.ShardID(), "blockNum", cl.BlockNum())
	}
	return nil
}
