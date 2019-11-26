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
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

// ValidatorListProvider ..
type ValidatorListProvider interface {
	Compute(
		epoch *big.Int, reader DataProvider,
	) (shard.State, error)
	ReadFromDB(epoch *big.Int, reader DataProvider) (shard.State, error)
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
	ReadShardState(epoch *big.Int) (shard.State, error)
	// GetHeader retrieves a block header from the database by hash and number.
	GetHeaderByHash(common.Hash) *block.Header
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig
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
)

func preStakingEnabledCommittee(s shardingconfig.Instance) shard.State {
	shardNum := int(s.NumShards())
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()
	shardSize := s.NumNodesPerShard()
	hmyAccounts := s.HmyAccounts()
	fnAccounts := s.FnAccounts()
	shardState := shard.State{}
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
		shardState = append(shardState, com)
	}
	return shardState
}

func eposStakedCommittee(
	s shardingconfig.Instance, stakerReader DataProvider, stakedSlotsCount int,
) (shard.State, error) {
	// TODO Nervous about this because overtime the list will become quite large
	candidates := stakerReader.ValidatorCandidates()
	essentials := map[common.Address]effective.SlotOrder{}

	utils.Logger().Info().Int("staked-candidates", len(candidates)).Msg("preparing epos staked committee")

	// TODO benchmark difference if went with data structure that sorts on insert
	for i := range candidates {
		validator, err := stakerReader.ReadValidatorInformation(candidates[i])
		validatorStake := big.NewInt(0)
		for _, delegation := range validator.Delegations {
			validatorStake.Add(validatorStake, delegation.Amount)
		}
		if err != nil {
			return nil, err
		}
		essentials[validator.Address] = effective.SlotOrder{
			validatorStake,
			validator.SlotPubKeys,
		}
	}

	shardCount := int(s.NumShards())
	superComm := make(shard.State, shardCount)
	hAccounts := s.HmyAccounts()

	for i := 0; i < shardCount; i++ {
		superComm[i] = shard.Committee{uint32(i), shard.SlotList{}}
	}

	for i := range hAccounts {
		spot := i % shardCount
		pub := &bls.PublicKey{}
		pub.DeserializeHexStr(hAccounts[i].BlsPublicKey)
		pubKey := shard.BlsPublicKey{}
		pubKey.FromLibBLSPublicKey(pub)
		superComm[spot].Slots = append(superComm[spot].Slots, shard.Slot{
			common2.ParseAddr(hAccounts[i].Address),
			pubKey,
			nil,
		})
	}
	staked := effective.Apply(essentials, stakedSlotsCount)
	shardBig := big.NewInt(int64(shardCount))

	if l := len(staked); l < stakedSlotsCount {
		// WARN unlikely to happen in production but will happen as we are developing
		stakedSlotsCount = l
	}

	for i := 0; i < stakedSlotsCount; i++ {
		bucket := int(new(big.Int).Mod(staked[i].BlsPublicKey.Big(), shardBig).Int64())
		slot := staked[i]
		superComm[bucket].Slots = append(superComm[bucket].Slots, shard.Slot{
			slot.Address,
			staked[i].BlsPublicKey,
			&slot.Dec,
		})
	}
	if c := len(candidates); c != 0 {
		utils.Logger().Info().Int("staked-candidates", c).
			RawJSON("staked-super-committee", []byte(superComm.JSON())).
			Msg("EPoS based super-committe")
	}
	return superComm, nil
}

// GetCommitteePublicKeys returns the public keys of a shard
func (def partialStakingEnabled) GetCommitteePublicKeys(committee *shard.Committee) []*bls.PublicKey {
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
) (newSuperComm shard.State, err error) {
	return reader.ReadShardState(epoch)
}

// ReadFromComputation is single entry point for reading the State of the network
func (def partialStakingEnabled) Compute(
	epoch *big.Int, stakerReader DataProvider,
) (newSuperComm shard.State, err error) {
	preStaking := true
	if stakerReader != nil {
		config := stakerReader.Config()
		if config.IsStaking(epoch) {
			preStaking = false
		}
	}

	instance := shard.Schedule.InstanceForEpoch(epoch)
	if preStaking {
		return preStakingEnabledCommittee(instance), nil
	}
	stakedSlots :=
		(instance.NumNodesPerShard() - instance.NumHarmonyOperatedNodesPerShard()) *
			int(instance.NumShards())
	return eposStakedCommittee(instance, stakerReader, stakedSlots)
}
