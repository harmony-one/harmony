package network

import (
	"testing"

	"github.com/harmony-one/harmony/numeric"
)

func TestFiveSecondsBaseStakedReward(t *testing.T) {
	expectedNewReward := BaseStakedReward.Mul(numeric.MustNewDecFromStr("5")).Quo(numeric.MustNewDecFromStr("8"))

	if !expectedNewReward.Equal(FiveSecondsBaseStakedReward) {
		t.Errorf(
			"Expected: %s, Got: %s", FiveSecondsBaseStakedReward.String(), expectedNewReward.String(),
		)
	}

	expectedNewReward = BaseStakedReward.Mul(numeric.MustNewDecFromStr("2")).Quo(numeric.MustNewDecFromStr("8"))
	if !expectedNewReward.Equal(TwoSecondsBaseStakedReward) {
		t.Errorf(
			"Expected: %s, Got: %s", TwoSecondsBaseStakedReward.String(), expectedNewReward.String(),
		)
	}
}
