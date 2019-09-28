package types

import (
	"math/big"
)

type (
	// Commission defines a commission parameters for a given validator.
	Commission struct {
		CommissionRates `json:"commission_rates" yaml:"commission_rates"`
		UpdateHeight    *big.Int `json:"update_time" yaml:"update_time"` // the block height the commission rate was last changed
	}

	// CommissionRates defines the initial commission rates to be used for creating a
	// validator.
	CommissionRates struct {
		Rate          Dec `json:"rate" yaml:"rate"`                       // the commission rate charged to delegators, as a fraction
		MaxRate       Dec `json:"max_rate" yaml:"max_rate"`               // maximum commission rate which validator can ever charge, as a fraction
		MaxChangeRate Dec `json:"max_change_rate" yaml:"max_change_rate"` // maximum increase of the validator commission every epoch, as a fraction
	}
)
