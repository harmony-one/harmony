package committee

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// ValidatorListProvider ..
type ValidatorListProvider interface {
	Compute(
		epoch *big.Int, reader DataProvider,
	) (*shard.State, error)
	ReadFromDB(epoch *big.Int, reader DataProvider) (*shard.State, error)
	GetCommitteePublicKeys(committee *shard.Committee) []*bls.PublicKey
}

// Reader is committee.Reader and it is the API that committee membership assignment needs
type Reader interface {
	ValidatorListProvider
}

// StakingCandidatesReader ..
type StakingCandidatesReader interface {
	ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error)
	ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorWrapper, error)
	ValidatorCandidates() []common.Address
}

// ChainReader is a subset of Engine.ChainReader, just enough to do assignment
type ChainReader interface {
	// ReadShardState retrieves sharding state given the epoch number.
	// This api reads the shard state cached or saved on the chaindb.
	// Thus, only should be used to read the shard state of the current chain.
	ReadShardState(epoch *big.Int) (*shard.State, error)
	// GetHeader retrieves a block header from the database by hash and number.
	GetHeaderByHash(common.Hash) *block.Header
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig
	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *block.Header
}

// DataProvider ..
type DataProvider interface {
	StakingCandidatesReader
	ChainReader
}

type partialStakingEnabled struct{}

var (
	// WithStakingEnabled ..
	WithStakingEnabled Reader = partialStakingEnabled{}
	// ErrComputeForEpochInPast ..
	ErrComputeForEpochInPast = errors.New("cannot compute for epoch in past")
)

func preStakingEnabledCommittee(s shardingconfig.Instance) *shard.State {
	shardNum := int(s.NumShards())
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()
	shardSize := s.NumNodesPerShard()
	hmyAccounts := s.HmyAccounts()
	fnAccounts := s.FnAccounts()
	shardState := &shard.State{}
	// Shard state needs to be sorted by shard ID
	for i := 0; i < shardNum; i++ {
		com := shard.Committee{ShardID: uint32(i)}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardNum // The initial account to use for genesis nodes
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hmyAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.Slot{
				common2.ParseAddr(hmyAccounts[index].Address),
				pubKey,
				nil,
			}
			com.Slots = append(com.Slots, curNodeID)
		}
		// add FN runner's key
		for j := shardHarmonyNodes; j < shardSize; j++ {
			index := i + (j-shardHarmonyNodes)*shardNum
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(fnAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.Slot{
				common2.ParseAddr(fnAccounts[index].Address),
				pubKey,
				nil,
			}
			com.Slots = append(com.Slots, curNodeID)
		}
		shardState.Shards = append(shardState.Shards, com)
	}
	return shardState
}

func eposStakedCommittee(
	s shardingconfig.Instance, stakerReader DataProvider, stakedSlotsCount int,
) (*shard.State, error) {
	// TODO Nervous about this because overtime the list will become quite large
	candidates := stakerReader.ValidatorCandidates()
	essentials, blsKeys :=
		map[common.Address]effective.SlotOrder{}, map[shard.BlsPublicKey]struct{}{}

	utils.Logger().Info().
		Int("staked-candidates", len(candidates)).
		Msg("preparing epos staked committee")

	// TODO benchmark difference if went with data structure that sorts on insert
	for i := range candidates {
		validator, err := stakerReader.ReadValidatorInformation(candidates[i])
		if err != nil {
			return nil, err
		}
		if !effective.IsEligibleForEPOSAuction(validator) {
			utils.Logger().Info().
				Int("staked-candidates", len(candidates)).
				RawJSON("candidate", []byte(validator.String())).
				Msg("validator not eligible for epos")
			continue
		}
		if err := validator.SanityCheck(); err != nil {
			utils.Logger().Info().
				Int("staked-candidates", len(candidates)).
				Err(err).
				RawJSON("candidate", []byte(validator.String())).
				Msg("validator sanity check failed")
			continue
		}
		validatorStake := big.NewInt(0)
		for _, delegation := range validator.Delegations {
			validatorStake.Add(validatorStake, delegation.Amount)
		}

		found := false
		dupKey := shard.BlsPublicKey{}
		for _, key := range validator.SlotPubKeys {
			if _, ok := blsKeys[key]; ok {
				found = true
				dupKey = key
			} else {
				blsKeys[key] = struct{}{}
			}
		}
		if found {
			const m = "Duplicate bls key found %x, in validator %+v. Ignoring"
			utils.Logger().Info().
				Int("staked-candidates", len(candidates)).
				Msgf(m, dupKey, validator)
			continue
		}

		essentials[validator.Address] = effective.SlotOrder{
			validatorStake,
			validator.SlotPubKeys,
		}
	}

	shardCount := int(s.NumShards())
	shardState := &shard.State{}
	shardState.Shards = make([]shard.Committee, shardCount)
	hAccounts := s.HmyAccounts()
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()

	for i := 0; i < shardCount; i++ {
		shardState.Shards[i] = shard.Committee{uint32(i), shard.SlotList{}}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardCount
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			shardState.Shards[i].Slots = append(shardState.Shards[i].Slots, shard.Slot{
				common2.ParseAddr(hAccounts[index].Address),
				pubKey,
				nil,
			})
		}
	}

	if stakedSlotsCount == 0 {
		utils.Logger().Info().
			Int("staked-candidates", len(candidates)).
			Int("slots-for-epos", stakedSlotsCount).
			Msg("committe composed only of harmony node")
		return shardState, nil
	}

	staked := effective.Apply(essentials, stakedSlotsCount)
	shardBig := big.NewInt(int64(shardCount))

	if l := len(staked); l < stakedSlotsCount {
		// WARN unlikely to happen in production but will happen as we are developing
		stakedSlotsCount = l
	}

	totalStake := numeric.ZeroDec()

	for i := 0; i < stakedSlotsCount; i++ {
		shardID := int(new(big.Int).Mod(staked[i].BlsPublicKey.Big(), shardBig).Int64())
		slot := staked[i]
		totalStake = totalStake.Add(slot.Dec)
		shardState.Shards[shardID].Slots = append(shardState.Shards[shardID].Slots, shard.Slot{
			slot.Address,
			staked[i].BlsPublicKey,
			&slot.Dec,
		})
	}

	if c := len(candidates); c != 0 {
		utils.Logger().Info().
			Int("staked-candidates", c).
			Str("total-staked-by-validators", totalStake.String()).
			RawJSON("staked-super-committee", []byte(shardState.String())).
			Msg("epos based super-committe")
	}

	return shardState, nil
}

// GetCommitteePublicKeys returns the public keys of a shard
func (def partialStakingEnabled) GetCommitteePublicKeys(committee *shard.Committee) []*bls.PublicKey {
	if committee == nil {
		utils.Logger().Error().Msg("[GetCommitteePublicKeys] Committee is nil")
		return []*bls.PublicKey{}
	}
	allIdentities := make([]*bls.PublicKey, len(committee.Slots))
	for i := range committee.Slots {
		identity := &bls.PublicKey{}
		committee.Slots[i].BlsPublicKey.ToLibBLSPublicKey(identity)
		allIdentities[i] = identity
	}

	return allIdentities
}

func (def partialStakingEnabled) ReadFromDB(
	epoch *big.Int, reader DataProvider,
) (newSuperComm *shard.State, err error) {
	return reader.ReadShardState(epoch)
}

// ReadFromComputation is single entry point for reading the State of the network
func (def partialStakingEnabled) Compute(
	epoch *big.Int, stakerReader DataProvider,
) (newSuperComm *shard.State, err error) {
	preStaking := true
	if stakerReader != nil {
		config := stakerReader.Config()
		if config.IsStaking(epoch) {
			preStaking = false
		}
	}

	instance := shard.Schedule.InstanceForEpoch(epoch)
	if preStaking {
		// Pre-staking shard state doesn't need to set epoch (backward compatible)
		return preStakingEnabledCommittee(instance), nil
	}
	// Sanity check, can't compute against epochs in past
	if e := stakerReader.CurrentHeader().Epoch(); epoch.Cmp(e) == -1 {
		utils.Logger().Error().Uint64("header-epoch", e.Uint64()).
			Uint64("compute-epoch", epoch.Uint64()).
			Msg("Tried to compute committee for epoch in past")
		return nil, ErrComputeForEpochInPast
	}
	stakedSlots :=
		(instance.NumNodesPerShard() - instance.NumHarmonyOperatedNodesPerShard()) *
			int(instance.NumShards())
	shardState, err := eposStakedCommittee(instance, stakerReader, stakedSlots)

	if err != nil {
		return nil, err
	}
	// Set the epoch of shard state
	shardState.Epoch = big.NewInt(0).Set(epoch)
	return shardState, nil
}
