package state

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/numeric"
	stk "github.com/harmony-one/harmony/staking/types"
)

func shareTestWrapper(addr common.Address, delegators int, amount *big.Int) *stk.ValidatorWrapper {
	w := &stk.ValidatorWrapper{}
	w.Address = addr
	w.BlockReward = big.NewInt(0)
	w.Validator.CommissionRates = stk.CommissionRates{
		Rate: numeric.ZeroDec(), MaxRate: numeric.OneDec(), MaxChangeRate: numeric.ZeroDec(),
	}
	for i := 0; i < delegators; i++ {
		w.Delegations = append(w.Delegations,
			stk.NewDelegation(common.BytesToAddress([]byte{byte(i + 1)}), new(big.Int).Set(amount)))
	}
	return w
}

// TestAddRewardNeverPaysMoreThanTheReward checks that the rewards credited to a
// validator's delegations add up to no more than the reward being distributed.
// Each delegator's share is rounded on its own, so the shares can add up to
// slightly more than the whole.
func TestAddRewardNeverPaysMoreThanTheReward(t *testing.T) {
	addr := common.BytesToAddress([]byte{0xAB})
	amount := big.NewInt(1000)

	for _, delegators := range []int{3, 6, 7, 9, 11} {
		snapshot := shareTestWrapper(addr, delegators, amount)
		total := numeric.NewDecFromBigInt(snapshot.TotalDelegation())
		shares := map[common.Address]numeric.Dec{}
		for _, d := range snapshot.Delegations {
			shares[d.DelegatorAddress] = numeric.NewDecFromBigInt(d.Amount).Quo(total)
		}

		for _, pool := range []int64{7, 10, 20, 50, 100, 1000} {
			db, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()), nil)
			if err != nil {
				t.Fatal(err)
			}
			db.SetStrictStateValidation(true)
			cur := shareTestWrapper(addr, delegators, amount)
			db.stateValidators[addr] = cur

			if err := db.AddReward(snapshot, big.NewInt(pool), shares); err != nil {
				t.Fatal(err)
			}
			sum := big.NewInt(0)
			for i := range cur.Delegations {
				sum.Add(sum, cur.Delegations[i].Reward)
			}
			if sum.Cmp(big.NewInt(pool)) > 0 {
				t.Errorf("delegators=%d pool=%d: distributed %v, more than the reward",
					delegators, pool, sum)
			}
		}
	}
}
