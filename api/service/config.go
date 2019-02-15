package service

import (
	"github.com/harmony-one/harmony/p2p"
)

// NodeConfig defines a structure of node configuration
// that can be used in services.
// This is to pass node configuration to services and prvent
// cyclic imports
type NodeConfig struct {
	Beacon   p2p.GroupID                    // the beacon group ID
	Group    p2p.GroupID                    // the group ID of the shard
	IsClient bool                           // whether this node is a client node, such as wallet/txgen
	IsBeacon bool                           // whether this node is a beacon node or not
	Actions  map[p2p.GroupID]p2p.ActionType // actions on the groups
}
