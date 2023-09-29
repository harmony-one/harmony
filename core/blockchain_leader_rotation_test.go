package core

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/stretchr/testify/require"
)

var k1 = bls.SerializedPublicKey{1, 2, 3}

func TestRotationMetaProcess(t *testing.T) {
	t.Run("same_leader_increase_count", func(t *testing.T) {
		rs := processRotationMeta(1, bls.SerializedPublicKey{}, LeaderRotationMeta{
			Pub:    bls.SerializedPublicKey{}.Bytes(),
			Epoch:  1,
			Count:  1,
			Shifts: 1,
		})
		require.Equal(t, LeaderRotationMeta{
			Pub:    bls.SerializedPublicKey{}.Bytes(),
			Epoch:  1,
			Count:  2,
			Shifts: 1,
		}, rs)
	})

	t.Run("new_leader_increase_shifts", func(t *testing.T) {
		rs := processRotationMeta(1, k1, LeaderRotationMeta{
			Pub:    bls.SerializedPublicKey{}.Bytes(),
			Epoch:  1,
			Count:  1,
			Shifts: 1,
		})
		require.Equal(t, LeaderRotationMeta{
			Pub:    k1.Bytes(),
			Epoch:  1,
			Count:  1,
			Shifts: 2,
		}, rs)
	})

	t.Run("new_epoch_reset_count", func(t *testing.T) {
		rs := processRotationMeta(2, k1, LeaderRotationMeta{
			Pub:    bls.SerializedPublicKey{}.Bytes(),
			Epoch:  1,
			Count:  1,
			Shifts: 1,
		})
		require.Equal(t, LeaderRotationMeta{
			Pub:    k1.Bytes(),
			Epoch:  2,
			Count:  1,
			Shifts: 0,
		}, rs)
	})
}
