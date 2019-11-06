package shard

import (
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
)

const (
	// BeaconChainID is the ShardID of the BeaconChain
	BeaconChainID = 0
)

// ShardingSchedule is the sharding configuration schedule.
// Depends on the type of the network.  Defaults to the mainnet schedule.
var (
	Schedule shardingconfig.Schedule = shardingconfig.MainnetSchedule
)
