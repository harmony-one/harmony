package committee

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// StateID means reading off whole network when using calls that accept
// a shardID parameter
const StateID = -1

// ValidatorList ..
type ValidatorList interface {
	Compute(
		epoch *big.Int, config params.ChainConfig, reader StakingCandidatesReader,
	) (shard.State, error)
	ReadFromDB(epoch *big.Int, reader ChainReader) (shard.State, error)
}

// PublicKeys per epoch
type PublicKeys interface {
	// If call shardID with StateID then only superCommittee is non-nil,
	// otherwise get back the shardSpecific slice as well.
	ComputePublicKeys(
		epoch *big.Int, reader ChainReader, shardID int,
	) (superCommittee, shardSpecific []*bls.PublicKey)

	ReadPublicKeysFromDB(
		hash common.Hash, reader ChainReader,
	) ([]*bls.PublicKey, error)
}

// Reader ..
type Reader interface {
	PublicKeys
	ValidatorList
}

// StakingCandidatesReader ..
type StakingCandidatesReader interface {
	ValidatorInformation(addr common.Address) (*staking.Validator, error)
	ValidatorStakingWithDelegation(addr common.Address) *big.Int
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
			curNodeID := shard.NodeID{
				common2.ParseAddr(hmyAccounts[index].Address),
				pubKey,
				nil,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		// add FN runner's key
		for j := shardHarmonyNodes; j < shardSize; j++ {
			index := i + (j-shardHarmonyNodes)*shardNum
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(fnAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				common2.ParseAddr(fnAccounts[index].Address),
				pubKey,
				nil,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		shardState = append(shardState, com)
	}
	return shardState
}

func with400Stakers(
	s shardingconfig.Instance, stakerReader StakingCandidatesReader,
) (shard.State, error) {
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
	superComm := make(shard.State, shardCount)
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
		pubKey := shard.BlsPublicKey{}
		pubKey.FromLibBLSPublicKey(scratchPad)
		superComm[spot].NodeList = append(
			superComm[spot].NodeList,
			shard.NodeID{
				top[i].Address,
				pubKey,
				&shard.StakedMember{big.NewInt(0)},
			},
		)
	}

	utils.Logger().Info().Ints("distribution of Stakers in Shards", fillCount)
	return superComm, nil
}

func (def partialStakingEnabled) ReadPublicKeysFromDB(
	h common.Hash, reader ChainReader,
) ([]*bls.PublicKey, error) {
	header := reader.GetHeaderByHash(h)
	shardID := header.ShardID()
	superCommittee, err := reader.ReadShardState(header.Epoch())
	if err != nil {
		return nil, err
	}
	subCommittee := superCommittee.FindCommitteeByID(shardID)
	if subCommittee == nil {
		return nil, ctxerror.New("cannot find shard in the shard state",
			"blockNumber", header.Number(),
			"shardID", header.ShardID(),
		)
	}
	committerKeys := []*bls.PublicKey{}

	for i := range subCommittee.NodeList {
		committerKey := new(bls.PublicKey)
		err := subCommittee.NodeList[i].BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, ctxerror.New("cannot convert BLS public key",
				"blsPublicKey", subCommittee.NodeList[i].BlsPublicKey).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	return committerKeys, nil

	return nil, nil
}

// ComputePublicKeys produces publicKeys of entire supercommittee per epoch, optionally providing a
// shard specific subcommittee
func (def partialStakingEnabled) ComputePublicKeys(
	epoch *big.Int, reader ChainReader, shardID int,
) ([]*bls.PublicKey, []*bls.PublicKey) {
	config := reader.Config()
	instance := shard.Schedule.InstanceForEpoch(epoch)
	if !config.IsStaking(epoch) {
		superComm := preStakingEnabledCommittee(instance)
		spot := 0
		allIdentities := make([]*bls.PublicKey, int(instance.NumShards())*instance.NumNodesPerShard())
		for i := range superComm {
			for j := range superComm[i].NodeList {
				identity := &bls.PublicKey{}
				superComm[i].NodeList[j].BlsPublicKey.ToLibBLSPublicKey(identity)
				allIdentities[spot] = identity
				spot++
			}
		}

		if shardID == StateID {
			return allIdentities, nil
		}

		subCommittee := superComm.FindCommitteeByID(uint32(shardID))
		subCommitteeIdentities := make([]*bls.PublicKey, len(subCommittee.NodeList))
		spot = 0
		for i := range subCommittee.NodeList {
			identity := &bls.PublicKey{}
			subCommittee.NodeList[i].BlsPublicKey.ToLibBLSPublicKey(identity)
			subCommitteeIdentities[spot] = identity
			spot++
		}

		return allIdentities, subCommitteeIdentities
	}
	// TODO Implement for the staked case
	return nil, nil
}

func (def partialStakingEnabled) ReadFromDB(
	epoch *big.Int, reader ChainReader,
) (newSuperComm shard.State, err error) {
	return reader.ReadShardState(epoch)
}

// ReadFromComputation is single entry point for reading the State of the network
func (def partialStakingEnabled) Compute(
	epoch *big.Int, config params.ChainConfig, stakerReader StakingCandidatesReader,
) (newSuperComm shard.State, err error) {
	instance := shard.Schedule.InstanceForEpoch(epoch)
	if !config.IsStaking(epoch) {
		return preStakingEnabledCommittee(instance), nil
	}
	return with400Stakers(instance, stakerReader)
}
