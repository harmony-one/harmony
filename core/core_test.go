package core

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/core/types"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
)

func TestIsEpochBlock(t *testing.T) {
	block1 := types.NewBlock(&types.Header{Number: big.NewInt(10)}, nil, nil)
	block2 := types.NewBlock(&types.Header{Number: big.NewInt(0)}, nil, nil)
	block3 := types.NewBlock(&types.Header{Number: big.NewInt(344064)}, nil, nil)
	block4 := types.NewBlock(&types.Header{Number: big.NewInt(77)}, nil, nil)
	block5 := types.NewBlock(&types.Header{Number: big.NewInt(78)}, nil, nil)
	block6 := types.NewBlock(&types.Header{Number: big.NewInt(188)}, nil, nil)
	block7 := types.NewBlock(&types.Header{Number: big.NewInt(189)}, nil, nil)
	tests := []struct {
		schedule shardingconfig.Schedule
		block    *types.Block
		expected bool
	}{
		{
			shardingconfig.MainnetSchedule,
			block1,
			false,
		},
		{
			shardingconfig.MainnetSchedule,
			block2,
			true,
		},
		{
			shardingconfig.MainnetSchedule,
			block3,
			true,
		},
		{
			shardingconfig.TestnetSchedule,
			block4,
			false,
		},
		{
			shardingconfig.TestnetSchedule,
			block5,
			true,
		},
		{
			shardingconfig.TestnetSchedule,
			block6,
			false,
		},
		{
			shardingconfig.TestnetSchedule,
			block7,
			true,
		},
	}
	for i, test := range tests {
		ShardingSchedule = test.schedule
		r := IsEpochBlock(test.block)
		if r != test.expected {
			t.Errorf("index: %v, expected: %v, got: %v\n", i, test.expected, r)
		}
	}
}
