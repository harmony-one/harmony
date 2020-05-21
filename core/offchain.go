package core

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/block"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/apr"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// CommitOffChainData write off chain data of a block onto db writer.
func (bc *BlockChain) CommitOffChainData(
	batch rawdb.DatabaseWriter,
	block *types.Block,
	receipts []*types.Receipt,
	cxReceipts []*types.CXReceipt,
	payout reward.Reader,
	state *state.DB,
) (status WriteStatus, err error) {
	// Write receipts of the block
	rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)
	isBeaconChain := bc.CurrentHeader().ShardID() == shard.BeaconChainShardID
	isStaking := bc.chainConfig.IsStaking(block.Epoch())
	isPreStaking := bc.chainConfig.IsPreStaking(block.Epoch())
	header := block.Header()
	isNewEpoch := len(header.ShardState()) > 0
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

	nextBlockEpoch, err := bc.getNextBlockEpoch(header)
	if err != nil {
		return NonStatTy, err
	}

	// Shard State and Validator Update
	if isNewEpoch {
		// Write shard state for the new epoch
		_, err := bc.WriteShardStateBytes(batch, nextBlockEpoch, header.ShardState())
		if err != nil {
			header.Logger(utils.Logger()).Warn().Err(err).Msg("cannot store shard state")
			return NonStatTy, err
		}
	}

	// Do bookkeeping for new staking txns
	newVals, err := bc.UpdateStakingMetaData(
		batch, block, state, epoch, nextBlockEpoch,
	)
	if err != nil {
		utils.Logger().Err(err).Msg("UpdateStakingMetaData failed")
		return NonStatTy, err
	}

	// Snapshot for all validators for new epoch at the second to last block
	// This snapshot of the state is consistent with the state used for election
	if isBeaconChain && shard.Schedule.IsLastBlock(header.Number().Uint64()+1) {
		// Update snapshots for all validators
		// Beacon chain always snapshot for the next epoch as cur_epoch + 1
		// as beacon chain won't have gap in epochs.
		newEpoch := big.NewInt(0).Add(header.Epoch(), big.NewInt(1))
		if err := bc.UpdateValidatorSnapshots(batch, newEpoch, state, newVals); err != nil {
			return NonStatTy, err
		}
	}

	// Writing beacon chain cross links
	if isBeaconChain &&
		bc.chainConfig.IsCrossLink(block.Epoch()) &&
		len(header.CrossLinks()) > 0 {
		crossLinks := &types.CrossLinks{}
		if err := rlp.DecodeBytes(
			header.CrossLinks(), crossLinks,
		); err != nil {
			header.Logger(utils.Logger()).Err(err).
				Msg("[insertChain/crosslinks] cannot parse cross links")
			return NonStatTy, err
		}
		if !crossLinks.IsSorted() {
			header.Logger(utils.Logger()).Err(err).
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
		if err != nil && nodeconfig.GetDefaultConfig().ShardID == shard.BeaconChainShardID {
			// Only beacon chain worries about this
			const msg = "DeleteFromPendingCrossLinks, crosslinks in header %d,  pending crosslinks: %d, problem: %+v"
			utils.Logger().Debug().Msgf(msg, len(*crossLinks), num, err)
		}
		const msg = "DeleteFromPendingCrossLinks, crosslinks in header %d,  pending crosslinks: %d"
		utils.Logger().
			Debug().
			Msgf(msg, len(*crossLinks), num)
		utils.Logger().Debug().Msgf(msg, len(*crossLinks), num)
	}

	if isBeaconChain && bc.Config().IsCrossLink(bc.CurrentBlock().Epoch()) {
		// Roll up latest crosslinks
		for i, c := uint32(0), shard.Schedule.InstanceForEpoch(
			epoch,
		).NumShards(); i < c; i++ {
			bc.LastContinuousCrossLink(batch, i)
		}
	}

	// BELOW ARE NON-MISSION-CRITICAL COMMITS

	// Update voting power of validators for all shards
	tempValidatorStats := map[common.Address]*staking.ValidatorStats{}
	if isNewEpoch && isBeaconChain {
		currentSuperCommittee, _ := bc.ReadShardState(bc.CurrentHeader().Epoch())
		if shardState, err := shard.DecodeWrapper(
			header.ShardState(),
		); err == nil {
			if stats, err := bc.UpdateValidatorVotingPower(
				batch, block, shardState, currentSuperCommittee, state,
			); err != nil {
				utils.Logger().
					Err(err).
					Msg("[UpdateValidatorVotingPower] Failed to update voting power")
			} else {
				tempValidatorStats = stats
			}
		} else {
			utils.Logger().
				Err(err).
				Msg("[UpdateValidatorVotingPower] Failed to decode shard state")
		}
		// fix the possible wrong apr in the staking epoch
		stakingEpoch := bc.Config().StakingEpoch
		secondStakingEpoch := big.NewInt(0).Add(stakingEpoch, common.Big1)
		thirdStakingEpoch := big.NewInt(0).Add(secondStakingEpoch, common.Big1)
		isThirdStakingEpoch := block.Epoch().Cmp(thirdStakingEpoch) == 0
		if isThirdStakingEpoch {
			// we have to do it for all validators, not only currently elected
			if validators, err := bc.ReadValidatorList(); err == nil {
				for _, addr := range validators {
					// get wrapper from the second staking epoch
					if snapshot, err := bc.ReadValidatorSnapshotAtEpoch(
						secondStakingEpoch, addr,
					); err == nil {
						if block := bc.GetBlockByNumber(
							shard.Schedule.EpochLastBlock(stakingEpoch.Uint64()),
						); block != nil {
							if aprComputed, err := apr.ComputeForValidator(
								bc, block, snapshot.Validator,
							); err == nil {
								stats, ok := tempValidatorStats[addr]
								if !ok {
									stats, err = bc.ReadValidatorStats(addr)
									if err != nil {
										continue
									}
								}
								if len(stats.APRs) <= 0 {
									continue
								}
								// take care of first apr not being staking apr
								if stats.APRs[0].Epoch.Cmp(stakingEpoch) > 0 {
									continue
								}
								stats.APRs[0] = staking.APREntry{stakingEpoch, *aprComputed}
								tempValidatorStats[addr] = stats
							}
						}
					}
				}
			}
		}
	}

	// Update block reward accumulator and slashes
	if isBeaconChain {
		if isStaking {
			roundResult := payout.ReadRoundResult()
			if err := bc.UpdateBlockRewardAccumulator(
				batch, roundResult.Total, block.Number().Uint64(),
			); err != nil {
				return NonStatTy, err
			}
			for _, paid := range [...][]reward.Payout{
				roundResult.BeaconchainAward, roundResult.ShardChainAward,
			} {
				for i := range paid {
					stats, ok := tempValidatorStats[paid[i].Addr]
					if !ok {
						stats, err = bc.ReadValidatorStats(paid[i].Addr)
						if err != nil {
							utils.Logger().Info().Err(err).
								Str("addr", paid[i].Addr.Hex()).
								Str("bls-earning-key", paid[i].EarningKey.Hex()).
								Msg("could not read validator stats to update for earning per key")
							continue
						}
						tempValidatorStats[paid[i].Addr] = stats
					}
					for j := range stats.MetricsPerShard {
						if stats.MetricsPerShard[j].Vote.Identity == paid[i].EarningKey {
							stats.MetricsPerShard[j].Earned.Add(
								stats.MetricsPerShard[j].Earned,
								paid[i].NewlyEarned,
							)
						}
					}

				}
			}

			bc.writeValidatorStats(tempValidatorStats, batch)

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
			if isNewEpoch && isPreStaking {
				// if prestaking and last block, write out the validator stats
				// so that it is available for the staking epoch
				bc.writeValidatorStats(tempValidatorStats, batch)
			}
			// block reward never accumulate before staking
			bc.WriteBlockRewardAccumulator(batch, common.Big0, block.Number().Uint64())
		}
	}

	return CanonStatTy, nil
}

func (bc *BlockChain) writeValidatorStats(
	tempValidatorStats map[common.Address]*staking.ValidatorStats,
	batch rawdb.DatabaseWriter,
) {
	type t struct {
		addr  common.Address
		stats *staking.ValidatorStats
	}

	sortedStats, i := make([]t, len(tempValidatorStats)), 0
	for key := range tempValidatorStats {
		sortedStats[i] = t{key, tempValidatorStats[key]}
		i++
	}

	sort.SliceStable(
		sortedStats,
		func(i, j int) bool {
			return bytes.Compare(
				sortedStats[i].addr[:], sortedStats[j].addr[:],
			) == -1
		},
	)
	for _, stat := range sortedStats {
		if err := rawdb.WriteValidatorStats(
			batch, stat.addr, stat.stats,
		); err != nil {
			utils.Logger().Info().Err(err).
				Str("validator address", stat.addr.Hex()).
				Msg("could not update stats for validator")
		}
	}
}

func (bc *BlockChain) getNextBlockEpoch(header *block.Header) (*big.Int, error) {
	nextBlockEpoch := header.Epoch()
	if len(header.ShardState()) > 0 {
		nextBlockEpoch = new(big.Int).Add(header.Epoch(), common.Big1)
		decodeShardState, err := shard.DecodeWrapper(header.ShardState())
		if err != nil {
			return nil, err
		}
		if decodeShardState.Epoch != nil && bc.chainConfig.IsStaking(decodeShardState.Epoch) {
			// After staking, the epoch will be decided by the epoch in the shard state.
			nextBlockEpoch = new(big.Int).Set(decodeShardState.Epoch)
		}
	}
	return nextBlockEpoch, nil
}
