package state

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/numeric"
	stk "github.com/harmony-one/harmony/staking/types"
)

// TestAddRewardShorterCurrentDelegations checks that AddReward reports an error
// when the validator holds fewer delegations than its snapshot, rather than
// indexing past the end of the current list.
func TestAddRewardShorterCurrentDelegations(t *testing.T) {
	addr := common.BytesToAddress([]byte{0xAB})
	delegator := common.BytesToAddress([]byte{0xCD})

	db, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatal(err)
	}

	// The stored validator carries a single (self) delegation. Seed the cache
	// directly: AddReward reads the wrapper from there.
	db.stateValidators[addr] = makeRewardTestWrapper(addr, []common.Address{addr})

	// The snapshot records two, as if a delegation had disappeared.
	snapshot := makeRewardTestWrapper(addr, []common.Address{addr, delegator})
	shares := map[common.Address]numeric.Dec{
		addr:      numeric.MustNewDecFromStr("0.5"),
		delegator: numeric.MustNewDecFromStr("0.5"),
	}

	err = db.AddReward(snapshot, big.NewInt(1000), shares)
	if err == nil {
		t.Fatal("expected an error when the snapshot has more delegations than the validator")
	}
	if !strings.Contains(err.Error(), "fewer than") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func makeRewardTestWrapper(addr common.Address, delegators []common.Address) *stk.ValidatorWrapper {
	w := &stk.ValidatorWrapper{}
	w.Address = addr
	w.SlotPubKeys = nil
	w.BlockReward = big.NewInt(0)
	for _, d := range delegators {
		w.Delegations = append(w.Delegations, stk.NewDelegation(d, big.NewInt(100)))
	}
	w.Validator.CommissionRates = stk.CommissionRates{
		Rate:          numeric.ZeroDec(),
		MaxRate:       numeric.OneDec(),
		MaxChangeRate: numeric.ZeroDec(),
	}
	return w
}
