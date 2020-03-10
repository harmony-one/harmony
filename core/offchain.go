package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

// CommitOffChainData write off chain data of a block onto db writer.
func (bc *BlockChain) CommitOffChainData(
	batch rawdb.DatabaseWriter,
	block *types.Block,
	receipts []*types.Receipt,
	cxReceipts []*types.CXReceipt,
	payout *big.Int,
	state *state.DB,
	root common.Hash,
) (status WriteStatus, err error) {
	// Write receipts of the block
	rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)

	// Cross-shard txns
	epoch := block.Header().Epoch()
	if bc.chainConfig.HasCrossTxFields(block.Epoch()) {
		shardingConfig := shard.Schedule.InstanceForEpoch(epoch)
		shardNum := int(shardingConfig.NumShards())
		for i := 0; i < shardNum; i++ {
			if i == int(block.ShardID()) {
				continue
			}

			shardReceipts := types.CXReceipts(cxReceipts).GetToShardReceipts(uint32(i))
			if err := rawdb.WriteCXReceipts(
				batch, uint32(i), block.NumberU64(), block.Hash(), shardReceipts,
			); err != nil {
				utils.Logger().Error().Err(err).
					Interface("shardReceipts", shardReceipts).
					Int("toShardID", i).
					Msg("WriteCXReceipts cannot write into database")
				return NonStatTy, err
			}
		}
		// Mark incomingReceipts in the block as spent
		bc.WriteCXReceiptsProofSpent(batch, block.IncomingReceipts())
	}

	// VRF + VDF
	// check non zero VRF field in header and add to local db
	// if len(block.Vrf()) > 0 {
	//	vrfBlockNumbers, _ := bc.ReadEpochVrfBlockNums(block.Header().Epoch())
	//	if (len(vrfBlockNumbers) > 0) && (vrfBlockNumbers[len(vrfBlockNumbers)-1] == block.NumberU64()) {
	//		utils.Logger().Error().
	//			Str("number", block.Number().String()).
	//			Str("epoch", block.Header().Epoch().String()).
	//			Msg("VRF block number is already in local db")
	//	} else {
	//		vrfBlockNumbers = append(vrfBlockNumbers, block.NumberU64())
	//		err = bc.WriteEpochVrfBlockNums(block.Header().Epoch(), vrfBlockNumbers)
	//		if err != nil {
	//			utils.Logger().Error().
	//				Str("number", block.Number().String()).
	//				Str("epoch", block.Header().Epoch().String()).
	//				Msg("failed to write VRF block number to local db")
	//			return NonStatTy, err
	//		}
	//	}
	//}
	//
	////check non zero Vdf in header and add to local db
	//if len(block.Vdf()) > 0 {
	//	err = bc.WriteEpochVdfBlockNum(block.Header().Epoch(), block.Number())
	//	if err != nil {
	//		utils.Logger().Error().
	//			Str("number", block.Number().String()).
	//			Str("epoch", block.Header().Epoch().String()).
	//			Msg("failed to write VDF block number to local db")
	//		return NonStatTy, err
	//	}
	//}

	// Shard State and Validator Update
	header := block.Header()
	if len(header.ShardState()) > 0 {
		// Write shard state for the new epoch
		epoch := new(big.Int).Add(header.Epoch(), common.Big1)
		shardState, err := block.Header().GetShardState()
		if err == nil && shardState.Epoch != nil && bc.chainConfig.IsStaking(shardState.Epoch) {
			// After staking, the epoch will be decided by the epoch in the shard state.
			epoch = new(big.Int).Set(shardState.Epoch)
		}

		newShardState, err := bc.WriteShardStateBytes(batch, epoch, header.ShardState())
		if err != nil {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("cannot store shard state")
			return NonStatTy, err
		}

		// Update elected validators
		if err := bc.WriteElectedValidatorList(
			batch, newShardState.StakedValidators().Addrs,
		); err != nil {
			return NonStatTy, err
		}

		// Update snapshots for all validators
		if err := bc.UpdateValidatorSnapshots(batch, epoch); err != nil {
			return NonStatTy, err
		}
	}

	// Do bookkeeping for new staking txns
	for _, tx := range block.StakingTransactions() {
		// keep offchain database consistency with onchain we need revert
		// but it should not happend unless local database corrupted
		if err := bc.UpdateStakingMetaData(batch, tx, root); err != nil {
			utils.Logger().Debug().Msgf("oops, UpdateStakingMetaData failed, err: %+v", err)
			return NonStatTy, err
		}
	}

	// Update voting power of validators for all shards
	if ss := block.Header().ShardState(); len(ss) > 0 &&
		block.ShardID() == shard.BeaconChainShardID {
		shardState := &shard.State{}
		if shardState, err = shard.DecodeWrapper(ss); err == nil {
			if err = bc.UpdateValidatorVotingPower(batch, shardState); err != nil {
				utils.Logger().
					Err(err).
					Msg("[UpdateValidatorVotingPower] Failed to update voting power")
			}
		} else {
			utils.Logger().
				Err(err).
				Msg("[UpdateValidatorVotingPower] Failed to decode shard state")
		}
	}

	// Writing beacon chain cross links
	if header.ShardID() == shard.BeaconChainShardID &&
		bc.chainConfig.IsCrossLink(block.Epoch()) &&
		len(header.CrossLinks()) > 0 {
		crossLinks := &types.CrossLinks{}
		if err := rlp.DecodeBytes(
			header.CrossLinks(), crossLinks,
		); err != nil {
			header.Logger(utils.Logger()).
				Warn().Err(err).
				Msg("[insertChain/crosslinks] cannot parse cross links")
			return NonStatTy, err
		}
		if !crossLinks.IsSorted() {
			header.Logger(utils.Logger()).
				Warn().Err(err).
				Msg("[insertChain/crosslinks] cross links are not sorted")
			return NonStatTy, errors.New("proposed cross links are not sorted")
		}
		for _, crossLink := range *crossLinks {
			// Process crosslink
			if err := bc.WriteCrossLinks(
				batch, types.CrossLinks{crossLink},
			); err == nil {
				utils.Logger().Info().
					Uint64("blockNum", crossLink.BlockNum()).
					Uint32("shardID", crossLink.ShardID()).
					Msg("[insertChain/crosslinks] Cross Link Added to Beaconchain")
			}

			cl0, _ := bc.ReadShardLastCrossLink(crossLink.ShardID())
			if cl0 == nil {
				rawdb.WriteShardLastCrossLink(batch, crossLink.ShardID(), crossLink.Serialize())
			}
		}

		// clean/update local database cache after crosslink inserted into blockchain
		num, err := bc.DeleteFromPendingCrossLinks(*crossLinks)
		if err != nil {
			const msg = "DeleteFromPendingCrossLinks, crosslinks in header %d,  pending crosslinks: %d, problem: %+v"
			utils.Logger().Debug().Msgf(msg, len(*crossLinks), num, err)
		}
		const msg = "DeleteFromPendingCrossLinks, crosslinks in header %d,  pending crosslinks: %d"
		utils.Logger().
			Debug().
			Msgf(msg, len(*crossLinks), num)
		utils.Logger().Debug().Msgf(msg, len(*crossLinks), num)
	}
	// Roll up latest crosslinks
	for i := uint32(0); i < shard.Schedule.InstanceForEpoch(epoch).NumShards(); i++ {
		bc.LastContinuousCrossLink(batch, i)
	}

	if bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {
		if bc.chainConfig.IsStaking(block.Epoch()) {
			if err := bc.UpdateBlockRewardAccumulator(
				batch, payout, block.Number().Uint64(),
			); err != nil {
				return NonStatTy, err
			}
			records := slash.Records{}
			if s := header.Slashes(); len(s) > 0 {
				if err := rlp.DecodeBytes(s, &records); err != nil {
					utils.Logger().Debug().Err(err).Msg("could not decode slashes in header")
				}
				if err := bc.DeleteFromPendingSlashingCandidates(records); err != nil {
					utils.Logger().Debug().Err(err).Msg("could not deleting pending slashes")
				}
			}
		} else {
			// block reward never accumulate before staking
			bc.WriteBlockRewardAccumulator(batch, common.Big0, block.Number().Uint64())
		}
	}

	return CanonStatTy, nil
}
