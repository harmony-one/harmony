package votepower

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/require"
)

// TestComputeWithNoHarmonySlots covers a committee made up only of staked
// members. The harmony share of the vote has no slots to sit in, so it belongs
// to the staked members as a group rather than to whichever of them is last.
func TestComputeWithNoHarmonySlots(t *testing.T) {
	previous := shard.Schedule
	shard.Schedule = shardingconfig.MainnetSchedule
	t.Cleanup(func() { shard.Schedule = previous })

	stake := numeric.NewDec(100)
	comm := &shard.Committee{ShardID: 0}
	for i := 1; i <= 3; i++ {
		s := stake
		comm.Slots = append(comm.Slots, shard.Slot{
			EcdsaAddress:   common.BytesToAddress([]byte{byte(i)}),
			BLSPublicKey:   bls.SerializedPublicKey{byte(i)},
			EffectiveStake: &s,
		})
	}

	spread := func(r *Roster) numeric.Dec {
		first := r.Voters[comm.Slots[0].BLSPublicKey].OverallPercent
		last := r.Voters[comm.Slots[len(comm.Slots)-1].BLSPublicKey].OverallPercent
		return last.Sub(first)
	}

	roster, err := Compute(comm, big.NewInt(1), true)
	require.NoError(t, err)
	// Equal stakes, so the last member is left holding no more than the rounding
	// remainder the balancing step is meant for.
	require.True(t,
		spread(roster).LT(numeric.MustNewDecFromStr("0.000001")),
		"last member holds %s more voting power than the first", spread(roster),
	)

	// Before the fork the whole harmony share lands on the last staked member.
	legacy, err := Compute(comm, big.NewInt(1), false)
	require.NoError(t, err)
	require.True(t, spread(legacy).GT(numeric.MustNewDecFromStr("0.5")),
		"expected the legacy behaviour to concentrate voting power")
}
