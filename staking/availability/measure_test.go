package availability

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

type fakerAuctioneer struct{}

const (
	to0 = "one1zyxauxquys60dk824p532jjdq753pnsenrgmef"
	to2 = "one14438psd5vrjes7qm97jrj3t0s5l4qff5j5cn4h"
)

var (
	validatorS0Addr, validatorS2Addr = common.Address{}, common.Address{}
	addrs                            = []common.Address{}
	validatorS0, validatorS2         = &staking.ValidatorWrapper{}, &staking.ValidatorWrapper{}
)

func init() {
	validatorS0Addr, _ = common2.Bech32ToAddress(to0)
	validatorS2Addr, _ = common2.Bech32ToAddress(to2)
	addrs = []common.Address{validatorS0Addr, validatorS2Addr}
}

func (fakerAuctioneer) ReadValidatorSnapshot(
	addr common.Address,
) (*staking.ValidatorWrapper, error) {
	switch addr {
	case validatorS0Addr:
		return validatorS0, nil
	case validatorS2Addr:
		return validatorS0, nil
	default:
		panic("bad input in test case")
	}
}

func defaultStateWithAccountsApplied() *state.DB {
	st := ethdb.NewMemDatabase()
	stateHandle, _ := state.New(common.Hash{}, state.NewDatabase(st))
	for _, addr := range addrs {
		stateHandle.CreateAccount(addr)
	}
	stateHandle.SetBalance(validatorS0Addr, big.NewInt(0).SetUint64(1994680320000000000))
	stateHandle.SetBalance(validatorS2Addr, big.NewInt(0).SetUint64(1999975592000000000))
	return stateHandle
}

func TestCompute(t *testing.T) {
	// state := defaultStateWithAccountsApplied()
	// const junkValue = 0

	// if err := compute(
	// 	fakerAuctioneer{}, state, validatorS0,
	// ); err != nil {
	// 	//
	// }

	// if err := compute(
	// 	fakerAuctioneer{}, state, validatorS2,
	// ); err != nil {
	// 	//
	// }

	t.Log("Unimplemented")
}
