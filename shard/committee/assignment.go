package committee

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// SuperCommitteeID means reading off whole network when using calls that accept
// a shardID parameter
const SuperCommitteeID = -1

// MembershipList  ..
type MembershipList interface {
	ReadFromComputation(
		epoch *big.Int, config params.ChainConfig, reader StakingCandidatesReader,
	) (shard.SuperCommittee, error)
	ReadFromChain(epoch *big.Int, reader SuperCommitteeCachedReader) (shard.SuperCommittee, error)
}

// PublicKeys per epoch
type PublicKeys interface {
	// If call shardID with SuperCommitteeID then only superCommittee is non-nil,
	// otherwise get back the shardSpecific slice as well.
	ReadPublicKeys(
		epoch *big.Int, config params.ChainConfig, shardID int,
	) (superCommittee, shardSpecific []*bls.PublicKey)
}

// Reader ..
type Reader interface {
	PublicKeys
	MembershipList
}

// StakingCandidatesReader ..
type StakingCandidatesReader interface {
	ValidatorInformation(addr common.Address) (*staking.Validator, error)
	ValidatorStakingWithDelegation(addr common.Address) numeric.Dec
	ValidatorCandidates() []common.Address
}

// SuperCommitteeCachedReader way to use ChainReader without cyclic import from engine
type SuperCommitteeCachedReader interface {
	ReadShardState(epoch *big.Int) (shard.SuperCommittee, error)
}

type partialStakingEnabled struct{}

var (
	// WithStakingEnabled ..
	WithStakingEnabled Reader = partialStakingEnabled{}
)

func preStakingEnabledCommittee(s shardingconfig.Instance) shard.SuperCommittee {
	shardNum := int(s.NumShards())
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()
	shardSize := s.NumNodesPerShard()
	hmyAccounts := s.HmyAccounts()
	fnAccounts := s.FnAccounts()
	shardState := shard.SuperCommittee{}
	for i := 0; i < shardNum; i++ {
		com := shard.Committee{ShardID: uint32(i)}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardNum // The initial account to use for genesis nodes
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hmyAccounts[index].BLSPublicKey)
			pubKey := shard.BLSPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				ECDSAAddress: common2.ParseAddr(hmyAccounts[index].Address),
				BLSPublicKey: pubKey,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		// add FN runner's key
		for j := shardHarmonyNodes; j < shardSize; j++ {
			index := i + (j-shardHarmonyNodes)*shardNum
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(fnAccounts[index].BLSPublicKey)
			pubKey := shard.BLSPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				ECDSAAddress: common2.ParseAddr(fnAccounts[index].Address),
				BLSPublicKey: pubKey,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		shardState = append(shardState, com)
	}
	return shardState
}

func with400Stakers(
	s shardingconfig.Instance, stakerReader StakingCandidatesReader,
) (shard.SuperCommittee, error) {
	// TODO Nervous about this because overtime the list will become quite large
	candidates := stakerReader.ValidatorCandidates()
	stakers := make([]*staking.Validator, len(candidates))
	for i := range candidates {
		// TODO Should be using .ValidatorStakingWithDelegation, not implemented yet
		validator, err := stakerReader.ValidatorInformation(candidates[i])
		if err != nil {
			return nil, err
		}
		stakers[i] = validator
	}

	sort.SliceStable(
		stakers,
		func(i, j int) bool { return stakers[i].Stake.Cmp(stakers[j].Stake) >= 0 },
	)
	const sCount = 401
	top := stakers[:sCount]
	shardCount := int(s.NumShards())
	superComm := make(shard.SuperCommittee, shardCount)
	fillCount := make([]int, shardCount)
	// TODO Finish this logic, not correct, need to operate EPoS on slot level,
	// not validator level

	for i := 0; i < shardCount; i++ {
		superComm[i] = shard.Committee{}
		superComm[i].NodeList = make(shard.NodeIDList, s.NumNodesPerShard())
	}

	scratchPad := &bls.PublicKey{}

	for i := range top {
		spot := int(top[i].Address.Big().Int64()) % shardCount
		fillCount[spot]++
		// scratchPad.DeserializeHexStr()
		pubKey := shard.BLSPublicKey{}
		pubKey.FromLibBLSPublicKey(scratchPad)
		superComm[spot].NodeList = append(
			superComm[spot].NodeList,
			shard.NodeID{
				ECDSAAddress: top[i].Address,
				BLSPublicKey: pubKey,
				Validator:    &shard.StakedValidator{big.NewInt(0)},
			},
		)
	}

	utils.Logger().Info().Ints("distribution of Stakers in Shards", fillCount)
	return superComm, nil
}

// ReadPublicKeys produces publicKeys of entire supercommittee per epoch, optionally providing a
// shard specific subcommittee
func (def partialStakingEnabled) ReadPublicKeys(
	epoch *big.Int, config params.ChainConfig, shardID int,
) ([]*bls.PublicKey, []*bls.PublicKey) {
	instance := shard.Schedule.InstanceForEpoch(epoch)
	if !config.IsStaking(epoch) {
		superComm := preStakingEnabledCommittee(instance)
		spot := 0
		allIdentities := make([]*bls.PublicKey, int(instance.NumShards())*instance.NumNodesPerShard())
		for i := range superComm {
			for j := range superComm[i].NodeList {
				identity := &bls.PublicKey{}
				superComm[i].NodeList[j].BLSPublicKey.ToLibBLSPublicKey(identity)
				allIdentities[spot] = identity
				spot++
			}
		}

		if shardID == SuperCommitteeID {
			return allIdentities, nil
		}

		subCommittee := superComm.FindCommitteeByID(uint32(shardID))
		subCommitteeIdentities := make([]*bls.PublicKey, len(subCommittee.NodeList))
		spot = 0
		for i := range subCommittee.NodeList {
			identity := &bls.PublicKey{}
			subCommittee.NodeList[i].BLSPublicKey.ToLibBLSPublicKey(identity)
			subCommitteeIdentities[spot] = identity
			spot++
		}

		return allIdentities, subCommitteeIdentities
	}

	// TODO Implement for the staked case
	return nil, nil
}

func (def partialStakingEnabled) ReadFromChain(
	epoch *big.Int, reader SuperCommitteeCachedReader,
) (newSuperComm shard.SuperCommittee, err error) {
	return reader.ReadShardState(epoch)
}

// ReadFromComputation is single entry point for reading the SuperCommittee of the network
func (def partialStakingEnabled) ReadFromComputation(
	epoch *big.Int, config params.ChainConfig, stakerReader StakingCandidatesReader,
) (newSuperComm shard.SuperCommittee, err error) {
	instance := shard.Schedule.InstanceForEpoch(epoch)
	if !config.IsStaking(epoch) {
		return preStakingEnabledCommittee(instance), nil
	}
	return with400Stakers(instance, stakerReader)
}
