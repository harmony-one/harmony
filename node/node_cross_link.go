package node

import (
	"fmt"
	"math/big"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/verify"
	"github.com/pkg/errors"
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

	crossLinks := &types.CrossLinks{}
	err := rlp.DecodeBytes(cxLinksData, crossLinks)
	if err != nil {
		return errors.Wrapf(
			err, "[CrossLinkVerification] failed to decode cross links",
		)
	}

	if !crossLinks.IsSorted() {
		return errors.New("[CrossLinkVerification] cross links are not sorted")
	}

	for _, crossLink := range *crossLinks {
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
			if i > crossLinkBatchSize*2 { // A sanity check to prevent spamming
				break
			}

			if _, ok := existingCLs[cl.Hash()]; ok {
				utils.Logger().Debug().Err(err).
					Msgf("[ProcessingCrossLink] Cross Link already exists in pending queue, pass. Beacon Epoch: %d, Block num: %d, Epoch: %d, shardID %d",
						node.Blockchain().CurrentHeader().Epoch(), cl.Number(), cl.Epoch(), cl.ShardID())
				continue
			}

			exist, err := node.Blockchain().ReadCrossLink(cl.ShardID(), cl.Number().Uint64())
			if err == nil && exist != nil {
				utils.Logger().Debug().Err(err).
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
		return errors.New("[VerifyCrossLink] Shard chains should not verify cross links")
	}

	if cl.BlockNum() <= 1 {
		return errors.New("[VerifyCrossLink] CrossLink BlockNumber should greater than 1")
	}

	if !node.Blockchain().Config().IsCrossLink(cl.Epoch()) {
		return errors.Errorf(
			"[VerifyCrossLink] CrossLink Epoch should >= cross link starting epoch %v %v",
			cl.Epoch(), node.Blockchain().Config().CrossLinkEpoch,
		)
	}

	aggSig := &bls.Sign{}
	sig := cl.Signature()
	if err := aggSig.Deserialize(sig[:]); err != nil {
		return errors.Wrapf(
			err,
			"[VerifyCrossLink] unable to deserialize multi-signature from payload",
		)
	}

	committee, err := node.lookupCommittee(cl.Epoch(), cl.ShardID())
	if err != nil {
		return err
	}
	decider, err := node.lookupDecider(cl.Epoch(), cl.ShardID())
	if err != nil {
		return err
	}

	return verify.AggregateSigForCommittee(
		node.Blockchain(), committee, decider, aggSig, cl.Hash(), cl.BlockNum(), cl.ViewID().Uint64(), cl.Epoch(), cl.Bitmap(),
	)
}

func (node *Node) lookupDecider(
	epoch *big.Int, shardID uint32,
) (quorum.Decider, error) {

	// Look up
	key := fmt.Sprintf("decider-%d-%d", epoch.Uint64(), shardID)
	if b, ok := node.deciderCache.Get(key); ok {
		return b.(quorum.Decider), nil
	}

	// If not found, construct
	committee, err := node.lookupCommittee(epoch, shardID)
	if err != nil {
		return nil, err
	}

	decider := quorum.NewDecider(
		quorum.SuperMajorityStake, committee.ShardID,
	)

	decider.SetMyPublicKeyProvider(func() (multibls.PublicKeys, error) {
		return nil, nil
	})

	if _, err := decider.SetVoters(committee, epoch); err != nil {
		return nil, err
	}

	// Put in cache
	node.deciderCache.Add(key, decider)

	return decider, nil
}

func (node *Node) lookupCommittee(
	epoch *big.Int, shardID uint32,
) (*shard.Committee, error) {

	// Look up
	key := fmt.Sprintf("committee-%d-%d", epoch.Uint64(), shardID)
	if b, ok := node.committeeCache.Get(key); ok {
		return b.(*shard.Committee), nil
	}

	// If not found, construct
	shardState, err := node.Blockchain().ReadShardState(epoch)
	if err != nil {
		return nil, err
	}

	committee, err := shardState.FindCommitteeByID(shardID)
	if err != nil {
		return nil, err
	}

	// Put in cache
	node.committeeCache.Add(key, committee)

	return committee, nil
}
