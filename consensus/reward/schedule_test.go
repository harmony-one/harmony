package reward

import (
	"testing"

	"github.com/harmony-one/harmony/numeric"
)

func TestPercentageForTimeStamp(t *testing.T) {
	testCases := []struct {
		time     string
		expected string
	}{
		{"2019-Jan-01", "0.2429"},
		{"2019-May-31", "0.2429"},
		{"2021-Nov-30", "0.8561"},
		{"2023-Apr-29", "0.9488"},
		{"2023-Apr-30", "0.9507"},
		{"2025-May-31", "1.0000"},
		{"2026-Jan-01", "1.0000"},
	}

	for _, tc := range testCases {
		result := PercentageForTimeStamp(mustParse(tc.time))
		expect := numeric.MustNewDecFromStr(tc.expected)
		if !result.Equal(expect) {
			t.Errorf("Time: %s, Chosen bucket percent: %s, Expected: %s",
				tc.time, result, expect)
		}
	}
}
