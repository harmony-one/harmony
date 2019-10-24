package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
)

func CreateNewValidator() Validator {
	cr := CommissionRates{Rate: numeric.OneDec(), MaxRate: numeric.OneDec(), MaxChangeRate: numeric.ZeroDec()}
	c := Commission{cr, big.NewInt(300)}
	d := Description{Name: "SuperHero", Identity: "YouWillNotKnow", Website: "under_construction", Details: "N/A"}
	v := Validator{Address: common.Address{}, SlotPubKeys: nil,
		Stake: big.NewInt(500), UnbondingHeight: big.NewInt(20), MinSelfDelegation: big.NewInt(7),
		Active: false, Commission: c, Description: d}
	return v
}
