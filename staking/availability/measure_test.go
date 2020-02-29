package availability

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

type electionReader interface {
	ReadElectedValidatorList() ([]common.Address, error)
	ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorWrapper, error)
}

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

func TestSetInactiveUnavailableValidators(t *testing.T) {
	t.Log("Unimplemented")
}
