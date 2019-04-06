package service

import (
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
)

// NodeConfig defines a structure of node configuration
// that can be used in services.
// This is to pass node configuration to services and prvent
// cyclic imports
type NodeConfig struct {
	// The three groupID design, please refer to https://github.com/harmony-one/harmony/blob/master/node/node.md#libp2p-integration
	Beacon       p2p.GroupID                    // the beacon group ID
	ShardGroupID p2p.GroupID                    // the group ID of the shard
	Client       p2p.GroupID                    // the client group ID of the shard
	IsClient     bool                           // whether this node is a client node, such as wallet/txgen
	IsBeacon     bool                           // whether this node is a beacon node or not
	IsLeader     bool                           // whether this node is a leader or not
	ShardID      uint32                         // shardID of this node
	Actions      map[p2p.GroupID]p2p.ActionType // actions on the groups
}

// GroupIDShards is a map of ShardGroupID ID
// key is the shard ID
// value is the corresponding group ID
var (
	GroupIDShards       map[p2p.ShardID]p2p.GroupID
	GroupIDShardClients map[p2p.ShardID]p2p.GroupID
)

func init() {
	GroupIDShards = make(map[p2p.ShardID]p2p.GroupID)
	GroupIDShardClients = make(map[p2p.ShardID]p2p.GroupID)

	// init beacon chain group IDs
	GroupIDShards[0] = p2p.GroupIDBeacon
	GroupIDShardClients[0] = p2p.GroupIDBeaconClient

	for i := 1; i < nodeconfig.MaxShards; i++ {
		sid := p2p.ShardID(i)
		GroupIDShards[sid] = p2p.NewGroupIDByShardID(sid)
		GroupIDShardClients[sid] = p2p.NewClientGroupIDByShardID(sid)
	}
}
