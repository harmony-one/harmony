package types

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/numeric"
)

var (
	zeroDec     = numeric.ZeroDec()
	oneThirdDec = numeric.NewDecWithPrec(33, 2)
	halfDec     = numeric.NewDecWithPrec(5, 1)
	twoThirdDec = numeric.NewDecWithPrec(66, 2)
	oneDec      = numeric.OneDec()
)

func TestCommissionRates_Copy(t *testing.T) {
	tests := []struct {
		cr CommissionRates
	}{
		{},
		{CommissionRates{zeroDec, halfDec, oneDec}},
		{CommissionRates{zeroDec, oneThirdDec, twoThirdDec}},
		{CommissionRates{oneThirdDec, twoThirdDec, oneDec}},
	}
	for i, test := range tests {
		cp := test.cr.Copy()

		if err := assertCommissionRatesDeepCopy(test.cr, cp); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func assertCommissionRatesDeepCopy(cr1, cr2 CommissionRates) error {
	if !reflect.DeepEqual(cr1, cr2) {
		return errors.New("not deep equal")
	}
	return assertCommissionRatesCopy(cr1, cr2)
}

func assertCommissionRatesCopy(cr1, cr2 CommissionRates) error {
	if err := assertDecCopy(cr1.Rate, cr2.Rate); err != nil {
		return fmt.Errorf("rate: %v", err)
	}
	if err := assertDecCopy(cr1.MaxRate, cr2.MaxRate); err != nil {
		return fmt.Errorf("maxRate: %v", err)
	}
	if err := assertDecCopy(cr1.MaxChangeRate, cr2.MaxChangeRate); err != nil {
		return fmt.Errorf("maxChangeRate: %v", err)
	}
	return nil
}

func assertDecCopy(d1, d2 numeric.Dec) error {
	if d1.IsNil() != d2.IsNil() {
		return errors.New("IsNil not equal")
	}
	if d1.IsNil() {
		return nil
	}
	if d1 == d2 {
		return errors.New("same address")
	}
	return nil
}
