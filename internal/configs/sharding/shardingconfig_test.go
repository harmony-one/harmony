package shardingconfig

import (
	"math/big"
	"testing"
)

func TestMainnetInstanceForEpoch(t *testing.T) {
	tests := []struct {
		epoch    *big.Int
		instance Instance
	}{
		{
			big.NewInt(0),
			mainnetV0,
		},
		{
			big.NewInt(1),
			mainnetV1,
		},
		{
			big.NewInt(2),
			mainnetV1,
		},
	}

	for _, test := range tests {
		in := MainnetSchedule.InstanceForEpoch(test.epoch)
		if in.NumShards() != test.instance.NumShards() || in.NumNodesPerShard() != test.instance.NumNodesPerShard() {
			t.Errorf("can't get the right instane for epoch: %v\n", test.epoch)
		}
	}
}

func TestCalcEpochNumber(t *testing.T) {
	tests := []struct {
		block uint64
		epoch *big.Int
	}{
		{
			0,
			big.NewInt(0),
		},
		{
			1,
			big.NewInt(0),
		},
		{
			327679,
			big.NewInt(0),
		},
		{
			327680,
			big.NewInt(0),
		},
		{
			344064,
			big.NewInt(1),
		},
		{
			344063,
			big.NewInt(0),
		},
		{
			344065,
			big.NewInt(1),
		},
		{
			360448,
			big.NewInt(2),
		},
	}

	for i, test := range tests {
		ep := MainnetSchedule.CalcEpochNumber(test.block)
		if ep.Cmp(test.epoch) != 0 {
			t.Errorf("CalcEpochNumber error: index %v, got %v, expect %v\n", i, ep, test.epoch)
		}
	}
}
