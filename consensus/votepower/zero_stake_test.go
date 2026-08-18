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

// TestComputeWithZeroTotalEffectiveStake covers a committee whose staked members
// all carry zero effective stake. There is no total to take a share of, so no
// share is computed from it, and the roster is still returned with its voting
// power adding up to one.
func TestComputeWithZeroTotalEffectiveStake(t *testing.T) {
	previous := shard.Schedule
	shard.Schedule = shardingconfig.MainnetSchedule
	t.Cleanup(func() { shard.Schedule = previous })

	zero := numeric.ZeroDec()
	comm := &shard.Committee{
		ShardID: 0,
		Slots: shard.SlotList{
			{
				EcdsaAddress:   common.BytesToAddress([]byte{0x01}),
				BLSPublicKey:   bls.SerializedPublicKey{0x01},
				EffectiveStake: &zero,
			},
		},
	}

	var roster *Roster
	require.NotPanics(t, func() {
		var err error
		roster, err = Compute(comm, big.NewInt(1))
		require.NoError(t, err)
	})
	require.True(t,
		roster.OurVotingPowerTotalPercentage.
			Add(roster.TheirVotingPowerTotalPercentage).
			Equal(numeric.OneDec()),
		"voting power should still add up to one",
	)
}
