package core

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
)

const (
	// GenesisEpoch is the number of the genesis epoch.
	GenesisEpoch = 0
)

// GetEpochFromBlockNumber calculates the epoch number the block belongs to
func GetEpochFromBlockNumber(blockNumber uint64) uint64 {
	return ShardingSchedule.CalcEpochNumber(blockNumber).Uint64()
}

// CalculateNewShardState get sharding state from previous epoch and calculate sharding state for new epoch
func CalculateNewShardState(
	bc *BlockChain, epoch *big.Int, assign committee.Assigner,
) (shard.SuperCommittee, error) {
	if assign.IsInitEpoch(epoch) {
		return assign.InitCommittee(ShardingSchedule.InstanceForEpoch(epoch)), nil
	}
	prevEpoch := new(big.Int).Sub(epoch, common.Big1)

	prevSuperCommittee, _ := bc.ReadShardState(prevEpoch)

	nextSuperCommittee := assign.NextCommittee(ShardingSchedule.InstanceForEpoch(epoch), prevEpoch, prevSuperCommittee)
	if nextSuperCommittee == nil {
		// TODO make nextcommittee return error
		return nil, ctxerror.New("cannot retrieve previous sharding state")
	}

	return nextSuperCommittee, nil
}

// TODO ek – shardingSchedule should really be part of a general-purpose network
//  configuration.  We are OK for the time being,
//  until the day we should let one node process join multiple networks.

// ShardingSchedule is the sharding configuration schedule.
// Depends on the type of the network.  Defaults to the mainnet schedule.
var (
	ShardingSchedule shardingconfig.Schedule = shardingconfig.MainnetSchedule
)

// CalculateShardState returns the shard state based on epoch number
// This api for getting shard state is what should be used to get shard state regardless of
// current chain dependency (ex. getting shard state from block header received during cross-shard transaction)
func CalculateShardState(epoch *big.Int, assign committee.Assigner, previous shard.SuperCommittee) shard.SuperCommittee {
	utils.Logger().Info().Int64("epoch", epoch.Int64()).Msg("Get Shard State of Epoch.")
	return assign.NextCommittee(ShardingSchedule.InstanceForEpoch(epoch), epoch, previous)
}

// CalculatePublicKeys returns the publickeys given epoch and shardID
func CalculatePublicKeys(epoch *big.Int, shardID uint32, assign committee.Assigner, previous shard.SuperCommittee) []*bls.PublicKey {
	shardState := CalculateShardState(epoch, assign, previous)
	// Update validator public keys
	subCommittee := shardState.FindCommitteeByID(shardID)

	if subCommittee == nil {
		utils.Logger().Warn().Uint32("shardID", shardID).Uint64("epoch", epoch.Uint64()).Msg("Cannot find committee")
		return nil
	}

	pubKeys := make([]*bls.PublicKey, len(subCommittee.NodeList))
	for i := 0; i <= len(subCommittee.NodeList); i++ {
		//
	}

	for _, node := range subCommittee.NodeList {
		pubKey := &bls.PublicKey{}
		pubKeyBytes := node.BLSPublicKey[:]
		err := pubKey.Deserialize(pubKeyBytes)
		if err != nil {
			utils.Logger().Warn().Str("pubKeyBytes", hex.EncodeToString(pubKeyBytes)).Msg("Cannot Deserialize pubKey")
			return nil
		}
		pubKeys = append(pubKeys, pubKey)
	}
	return pubKeys
}
