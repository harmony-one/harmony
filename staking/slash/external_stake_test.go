package slash

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/stretchr/testify/require"
)

func externalStakeWrapper(self, external *big.Int) *staking.ValidatorWrapper {
	w := &staking.ValidatorWrapper{}
	w.Address = common.BytesToAddress([]byte{0x01})
	w.BlockReward = big.NewInt(0)
	w.Validator.CommissionRates = staking.CommissionRates{
		Rate: numeric.ZeroDec(), MaxRate: numeric.OneDec(), MaxChangeRate: numeric.ZeroDec(),
	}
	w.SlotPubKeys = []bls.SerializedPublicKey{{0x01}}
	w.MinSelfDelegation = new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	w.MaxTotalDelegation = new(big.Int).Mul(big.NewInt(100000), big.NewInt(1e18))
	w.LastEpochInCommittee = big.NewInt(0)
	w.CreationHeight = big.NewInt(0)
	w.Delegations = staking.Delegations{
		staking.NewDelegation(common.BytesToAddress([]byte{0x01}), new(big.Int).Set(self)),
		staking.NewDelegation(common.BytesToAddress([]byte{0x02}), new(big.Int).Set(external)),
	}
	return w
}

// TestSlashWithNoExternalStake covers a validator that still carries an external
// delegator whose stake has all been undelegated. Delegation entries remain after
// a full undelegation, so the external stake the debt is apportioned over can add
// up to nothing, leaving no share to apportion by.
func TestSlashWithNoExternalStake(t *testing.T) {
	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	require.NoError(t, err)

	self := new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	snapshot := externalStakeWrapper(self, big.NewInt(0))
	current := externalStakeWrapper(self, big.NewInt(0))

	track := &Application{TotalSlashed: big.NewInt(0), TotalBeneficiaryReward: big.NewInt(0)}

	require.NotPanics(t, func() {
		err = delegatorSlashApply(
			snapshot, current, statedb, common.BytesToAddress([]byte{0x09}),
			big.NewInt(1), track, true, true,
		)
	})
	require.NoError(t, err)

	// The validator's own stake is still slashed by half.
	require.Equal(t, new(big.Int).Div(self, big.NewInt(2)), track.TotalSlashed)
	require.Equal(t, effective.Banned, current.Status)
}
