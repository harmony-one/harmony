package committee

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// MembershipList  ..
type MembershipList interface {
	Read(
		epoch *big.Int, config params.ChainConfig, stakerReader StakingCandidatesReader,
	) (shard.SuperCommittee, error)
}

// PublicKeys per epoch
type PublicKeys interface {
	ReadPublicKeys(epoch *big.Int, config params.ChainConfig) []*bls.PublicKey
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

type partialStakingEnabled struct{}

var (
	// WithStakingEnabled ..
	WithStakingEnabled Reader = partialStakingEnabled{}
	// Genesis is the committee used at initial creation of Harmony blockchain
	Genesis = preStakingEnabledCommittee(shard.Schedule.InstanceForEpoch(big.NewInt(0)))
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

func with400Stakers(s shardingconfig.Instance, stakerReader StakingCandidatesReader) (shard.SuperCommittee, error) {
	// TODO Nervous about this because overtime the list will become quite large
	candidates := stakerReader.ValidatorCandidates()
	validators := make([]*staking.Validator, len(candidates))
	for i := range candidates {
		// TODO Should be using .ValidatorStakingWithDelegation, not implemented yet
		validator, err := stakerReader.ValidatorInformation(candidates[i])
		if err != nil {
			return nil, err
		}
		validators[i] = validator
	}

	return nil, nil
}

// ReadPublicKeys produces publicKeys of entire supercommittee per epoch
func (def partialStakingEnabled) ReadPublicKeys(epoch *big.Int, config params.ChainConfig) []*bls.PublicKey {
	// instance := shard.Schedule.InstanceForEpoch(epoch)
	// if !config.IsStaking(epoch) {
	// 	superComm := preStakingEnabledCommittee(instance)
	// 	identities := make([]*bls.PublicKey, int(instance.NumShards())*instance.NumNodesPerShard())
	// 	spot := 0
	// 	for i := range superComm {
	// 		for j := range superComm[i].NodeList {
	// 			identity := &bls.PublicKey{}
	// 			superComm[i].NodeList[j].BLSPublicKey.ToLibBLSPublicKey(identity)
	// 			identities[spot] = identity
	// 			spot++
	// 		}
	// 	}
	// 	return identities
	// }
	return nil
}

// Read returns the supercommittee used until staking
// was enabled, previously called CalculateInitShardState
// Pass as well the chainReader?
func (def partialStakingEnabled) Read(
	epoch *big.Int, config params.ChainConfig, stakerReader StakingCandidatesReader,
) (shard.SuperCommittee, error) {
	instance := shard.Schedule.InstanceForEpoch(epoch)
	if !config.IsStaking(epoch) {
		return preStakingEnabledCommittee(instance), nil
	}
	return with400Stakers(instance, stakerReader)
}
