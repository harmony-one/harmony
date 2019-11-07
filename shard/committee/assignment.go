package committee

import (
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
)

// MembershipList  ..
type MembershipList interface {
	Read(*big.Int) shard.SuperCommittee
}

// PublicKeys per epoch
type PublicKeys interface {
	ReadPublicKeys(*big.Int) []*bls.PublicKey
}

// MemberReader ..
type MemberReader interface {
	PublicKeys
	MembershipList
}

type partialStakingEnabled struct{}

var (
	// IncorporatingStaking ..
	IncorporatingStaking MemberReader = partialStakingEnabled{}
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

// ReadPublicKeys produces publicKeys of entire supercommittee per epoch
func (def partialStakingEnabled) ReadPublicKeys(epoch *big.Int) []*bls.PublicKey {
	switch instance := shard.Schedule.InstanceForEpoch(epoch); instance.SuperCommittee() {
	case shardingconfig.Genesis:
		superComm := preStakingEnabledCommittee(instance)
		identities := make([]*bls.PublicKey, int(instance.NumShards())*instance.NumNodesPerShard())
		spot := 0
		for i := range superComm {
			for j := range superComm[i].NodeList {
				identity := &bls.PublicKey{}
				superComm[i].NodeList[j].BLSPublicKey.ToLibBLSPublicKey(identity)
				identities[spot] = identity
				spot++
			}
		}
		return identities
	case shardingconfig.PartiallyOpenStake:
		return nil
	}
	return nil
}

// Read returns the supercommittee used until staking
// was enabled, previously called CalculateInitShardState
// Pass as well the chainReader?
func (def partialStakingEnabled) Read(epoch *big.Int) shard.SuperCommittee {
	switch instance := shard.Schedule.InstanceForEpoch(epoch); instance.SuperCommittee() {
	case shardingconfig.Genesis:
		return preStakingEnabledCommittee(instance)
	case shardingconfig.PartiallyOpenStake:
		return nil
	}
	return nil
}
