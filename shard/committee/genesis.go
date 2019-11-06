package committee

import (
	"fmt"
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

// Members ..
type Members interface {
	PublicKeys
	MembershipList
}

type partialStakingEnabled struct{}

var (
	// IncorporatingStaking ..
	IncorporatingStaking Members = partialStakingEnabled{}
)

func genesisCommittee() shard.SuperCommittee {
	s := shard.Schedule.InstanceForEpoch(big.NewInt(0))
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

func (def partialStakingEnabled) ReadPublicKeys(epoch *big.Int) []*bls.PublicKey {
	switch shard.Schedule.InstanceForEpoch(epoch).SuperCommittee().(type) {
	case shardingconfig.Genesis:
		return nil
	case shardingconfig.PartiallyOpenStake:
		return nil
	}
	return nil
}

// ReadGenesis returns the supercommittee used until staking
// was enabled, previously called CalculateInitShardState
func (def partialStakingEnabled) Read(epoch *big.Int) shard.SuperCommittee {
	switch shard.Schedule.InstanceForEpoch(epoch).SuperCommittee().(type) {
	case shardingconfig.Genesis:
		fmt.Println("calling genesis")
		return genesisCommittee()
	case shardingconfig.PartiallyOpenStake:
		fmt.Println("calling partiallopenstake")
		return nil
	}
	return nil
}
