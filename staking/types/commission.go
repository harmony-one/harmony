package types

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/numeric"
)

type (
	// Commission defines a commission parameters for a given validator.
	Commission struct {
		CommissionRates
		UpdateHeight *big.Int
	}

	// CommissionRates defines the initial commission rates to be used for creating a
	// validator.
	CommissionRates struct {
		Rate          numeric.Dec // the commission rate charged to delegators, as a fraction
		MaxRate       numeric.Dec // maximum commission rate which validator can ever charge, as a fraction
		MaxChangeRate numeric.Dec // maximum increase of the validator commission every epoch, as a fraction
	}
)

// MarshalJSON ..
func (c Commission) MarshalJSON() ([]byte, error) {
	type t struct {
		CommissionRates `json:"commision-rates"`
		UpdateHeight    string `json:"update-height"`
	}
	return json.Marshal(t{
		CommissionRates: c.CommissionRates,
		UpdateHeight:    c.UpdateHeight.String(),
	})
}

// MarshalJSON ..
func (cr CommissionRates) MarshalJSON() ([]byte, error) {
	type t struct {
		Rate          string `json:"rate"`
		MaxRate       string `json:"max-rate"`
		MaxChangeRate string `json:"max-change-rate"`
	}
	return json.Marshal(t{
		Rate:          cr.Rate.String(),
		MaxRate:       cr.MaxRate.String(),
		MaxChangeRate: cr.MaxChangeRate.String(),
	})
}

// String returns a human readable string representation of a validator.
func (c Commission) String() string {
	return fmt.Sprintf(`
  Commission:
  Rate:                %s
  MaxRate:             %s
  MaxChangeRate:       %s
  UpdateHeight:        %v`,
		c.Rate, c.MaxRate, c.MaxChangeRate,
		c.UpdateHeight)
}
