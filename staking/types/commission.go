package types

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"
)

type (
	// Commission defines a commission parameters for a given validator.
	Commission struct {
		CommissionRates
		UpdateHeight *big.Int `json:"update-height"`
	}

	// CommissionRates defines the initial commission rates to be used for creating a
	// validator.
	CommissionRates struct {
		// the commission rate charged to delegators, as a fraction
		Rate numeric.Dec `json:"rate"`
		// maximum commission rate which validator can ever charge, as a fraction
		MaxRate numeric.Dec `json:"max-rate"`
		// maximum increase of the validator commission every epoch, as a fraction
		MaxChangeRate numeric.Dec `json:"max-change-rate"`
	}
)

// Copy deep copies the staking.CommissionRates
func (cr CommissionRates) Copy() CommissionRates {
	return CommissionRates{
		Rate:          cr.Rate.Copy(),
		MaxRate:       cr.MaxRate.Copy(),
		MaxChangeRate: cr.MaxChangeRate.Copy(),
	}
}
