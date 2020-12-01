package shardingconfig

import (
	"fmt"
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
			big.NewInt(12),
			mainnetV1,
		},
		{
			big.NewInt(19),
			mainnetV1_1,
		},
		{
			big.NewInt(25),
			mainnetV1_2,
		},
		{
			big.NewInt(36),
			mainnetV1_3,
		},
		{
			big.NewInt(46),
			mainnetV1_4,
		},
		{
			big.NewInt(54),
			mainnetV1_5,
		},
		{
			big.NewInt(365),
			mainnetV2_2,
		},
		{
			big.NewInt(366),
			mainnetV3,
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
		{
			6207973,
			big.NewInt(358),
		},
		{
			6324223, // last block before 2s
			big.NewInt(365),
		},
		{
			6324224,
			big.NewInt(366),
		},
		{
			6389759,
			big.NewInt(367),
		},
		{
			6389777,
			big.NewInt(368),
		},
	}

	for i, test := range tests {
		ep := MainnetSchedule.CalcEpochNumber(test.block)
		if ep.Cmp(test.epoch) != 0 {
			t.Errorf("CalcEpochNumber error: index %v, got %v, expect %v\n", i, ep, test.epoch)
		}
	}
}

func TestIsLastBlock(t *testing.T) {
	tests := []struct {
		block  uint64
		result bool
	}{
		{
			0,
			false,
		},
		{
			1,
			false,
		},
		{
			327679,
			false,
		},
		{
			344063,
			true,
		},
		{
			344064,
			false,
		},
		{
			360447,
			true,
		},
		{
			360448,
			false,
		},
		{
			6207973,
			false,
		},
		{
			6324223, // last block of first 2s epoch
			true,
		},
		{
			6324224,
			false,
		},
		{
			6356991,
			true,
		},
		{
			6356992,
			false,
		},
		{
			6389759,
			true,
		},
	}

	for i, test := range tests {
		ep := MainnetSchedule.IsLastBlock(test.block)
		if test.result != ep {
			t.Errorf("IsLastBlock error: index %v, got %v, expect %v\n", i, ep, test.result)
		}
	}
}
func TestEpochLastBlock(t *testing.T) {
	tests := []struct {
		epoch     uint64
		lastBlock uint64
	}{
		{
			0,
			344063,
		},
		{
			1,
			360447,
		},
		{
			2,
			376831,
		},
		{
			3,
			393215,
		},
		{
			358,
			6209535,
		},
		{
			365,
			6324223, // last block before 2s
		},
		{
			366,
			6356991, // last block of first 2s epoch
		},
		{
			367,
			6389759, // last block of second 2s epoch
		},
	}

	for i, test := range tests {
		ep := MainnetSchedule.EpochLastBlock(test.epoch)
		if test.lastBlock != ep {
			t.Errorf("EpochLastBlock error: index %v, got %v, expect %v\n", i, ep, test.lastBlock)
		}
	}
}

func TestTwoSecondsFirstBlock(t *testing.T) {
	if MainnetSchedule.twoSecondsFirstBlock() != 6324224 {
		t.Errorf("twoSecondsFirstBlock error: got %v, expect %v\n", MainnetSchedule.twoSecondsFirstBlock(), 6324224)
	}
}

func TestGetShardingStructure(t *testing.T) {
	shardID := 0
	numShard := 4
	res := genShardingStructure(numShard, shardID, "http://s%d.t.hmy.io:9500", "ws://s%d.t.hmy.io:9800")
	if len(res) != 4 || !res[0]["current"].(bool) || res[1]["current"].(bool) || res[2]["current"].(bool) || res[3]["current"].(bool) {
		t.Error("Error when generating sharding structure")
	}
	for i := 0; i < numShard; i++ {
		if res[i]["current"].(bool) != (i == shardID) {
			t.Error("Error when generating sharding structure")
		}
		if res[i]["shardID"].(int) != i {
			t.Error("Error when generating sharding structure")
		}
		if res[i]["http"].(string) != fmt.Sprintf("http://s%d.t.hmy.io:9500", i) {
			t.Error("Error when generating sharding structure")
		}
		if res[i]["ws"].(string) != fmt.Sprintf("ws://s%d.t.hmy.io:9800", i) {
			t.Error("Error when generating sharding structure")
		}
	}
}
