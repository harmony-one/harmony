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
	validatorS0, validatorS2         = &staking.ValidatorWrapper{}, &staking.ValidatorWrapper{}
)

func init() {
	validatorS0Addr, _ = common2.Bech32ToAddress(to0)
	validatorS2Addr, _ = common2.Bech32ToAddress(to2)
}

func (fakerAuctioneer) ReadElectedValidatorList() ([]common.Address, error) {
	return []common.Address{validatorS0Addr, validatorS2Addr}, nil
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
	addrs, _ := (fakerAuctioneer{}).ReadElectedValidatorList()
	for _, addr := range addrs {
		stateHandle.CreateAccount(addr)
	}
	stateHandle.SetBalance(validatorS0Addr, big.NewInt(0).SetUint64(1994680320000000000))
	stateHandle.SetBalance(validatorS2Addr, big.NewInt(0).SetUint64(1999975592000000000))
	return stateHandle
}

func TestSetInactiveUnavailableValidators(t *testing.T) {
	state := defaultStateWithAccountsApplied()
	nowEpoch := big.NewInt(5)
	if err := SetInactiveUnavailableValidators(
		fakerAuctioneer{}, state, nowEpoch,
	); err != nil {
		//
	}

	t.Log("Unimplemented")
}
