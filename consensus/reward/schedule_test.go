package reward

import (
	"testing"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

func TestPercentageForTimeStamp(t *testing.T) {
	testCases := []struct {
		time     string
		expected string
	}{
		{"2019-Jan-01", "0.242864761904762"},
		{"2019-May-31", "0.242864761904762"},
		{"2021-Nov-30", "0.856135555555555"},
		{"2023-Apr-29", "0.948773809523808"},
		{"2023-Apr-30", "0.950744047619047"},
		{"2025-May-31", "1.000000000000000"},
		{"2026-Jan-01", "1.000000000000000"},
	}

	for _, tc := range testCases {
		result := PercentageForTimeStamp(shard.Schedule, mustParse(tc.time))
		expect := numeric.MustNewDecFromStr(tc.expected)
		if !result.Equal(expect) {
			t.Errorf("Time: %s, Chosen bucket percent: %s, Expected: %s",
				tc.time, result, expect)
		}
	}
}

// tests panic for invalid timestamp
func TestMustParse(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("mustParse did not panic")
		}
	}()
	mustParse("invalid-timestamp")
}

func TestPercentageForTimeStampForNonMainnet(t *testing.T) {
	if p := PercentageForTimeStamp(shardingconfig.LocalnetSchedule, 0); !p.Equal(numeric.NewDec(1)) {
		t.Errorf("Expected 1, got %s", p)
	}
}
