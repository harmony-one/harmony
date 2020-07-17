package service

import (
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

// NodeConfig defines a structure of node configuration
// that can be used in services.
// This is to pass node configuration to services and prvent
// cyclic imports
type NodeConfig struct {
	// The three groupID design, please refer to https://github.com/harmony-one/harmony/blob/master/node/node.md#libp2p-integration
	Beacon       nodeconfig.GroupID                           // the beacon group ID
	ShardGroupID nodeconfig.GroupID                           // the group ID of the shard
	Client       nodeconfig.GroupID                           // the client group ID of the shard
	IsBeacon     bool                                         // whether this node is a beacon node or not
	ShardID      uint32                                       // shardID of this node
	Actions      map[nodeconfig.GroupID]nodeconfig.ActionType // actions on the groups
}

// GroupIDShards is a map of ShardGroupID ID
// key is the shard ID
// value is the corresponding group ID
var (
	GroupIDShards       = map[nodeconfig.ShardID]nodeconfig.GroupID{}
	GroupIDShardClients = map[nodeconfig.ShardID]nodeconfig.GroupID{}
)

func init() {
	// init beacon chain group IDs
	GroupIDShards[0] = nodeconfig.NewGroupIDByShardID(0)
	GroupIDShardClients[0] = nodeconfig.NewClientGroupIDByShardID(0)

	for i := 1; i < nodeconfig.MaxShards; i++ {
		sid := nodeconfig.ShardID(i)
		GroupIDShards[sid] = nodeconfig.NewGroupIDByShardID(sid)
		GroupIDShardClients[sid] = nodeconfig.NewClientGroupIDByShardID(sid)
	}
}
