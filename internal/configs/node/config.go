// Package nodeconfig includes all the configuration variables for a node.
// It is a global configuration for node and other services.
// It will be included in node module, and other modules.
package nodeconfig

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/p2p"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
)

// Role defines a role of a node.
type Role byte

// All constants for different node roles.
const (
	Unknown Role = iota
	ShardLeader
	ShardValidator
	BeaconLeader
	BeaconValidator
	NewNode
	ClientNode
	WalletNode
	ArchivalNode
)

func (role Role) String() string {
	switch role {
	case Unknown:
		return "Unknown"
	case ShardLeader:
		return "ShardLeader"
	case ShardValidator:
		return "ShardValidator"
	case BeaconLeader:
		return "BeaconLeader"
	case BeaconValidator:
		return "BeaconValidator"
	case NewNode:
		return "NewNode"
	case ClientNode:
		return "ClientNode"
	case WalletNode:
		return "WalletNode"
	case ArchivalNode:
		return "ArchivalNode"
	}
	return "Unknown"
}

// Global is the index of the global node configuration
const (
	Global    = 0
	MaxShards = 32 // maximum number of shards. It is also the maxium number of configs.
)

// ConfigType is the structure of all node related configuration variables
type ConfigType struct {
	// The three groupID design, please refer to https://github.com/harmony-one/harmony/blob/master/node/node.md#libp2p-integration
	beacon     p2p.GroupID // the beacon group ID
	group      p2p.GroupID // the group ID of the shard
	client     p2p.GroupID // the client group ID of the shard
	isClient   bool        // whether this node is a client node, such as wallet/txgen
	isBeacon   bool        // whether this node is a beacon node or not
	isLeader   bool        // whether this node is a leader or not
	isArchival bool        // whether this node is a archival node. archival node backups all blockchain information.
	shardID    uint32      // shardID of this node
	role       Role        // Role of the node

	ShardIDString   string
	StringRole      string
	Host            p2p.Host
	StakingPriKey   *ecdsa.PrivateKey
	P2pPriKey       p2p_crypto.PrivKey
	ConsensusPriKey *bls.SecretKey
	ConsensusPubKey *bls.PublicKey
	MainDB          *ethdb.LDBDatabase
	BeaconDB        *ethdb.LDBDatabase

	SelfPeer p2p.Peer
	Leader   p2p.Peer
}

// configs is a list of node configuration.
// It has at least one configuration.
// The first one is the default, global node configuration
var configs []ConfigType
var onceForConfigs sync.Once

// GetConfigs return the indexed ConfigType variable
func GetConfigs(index int) *ConfigType {
	onceForConfigs.Do(func() {
		configs = make([]ConfigType, MaxShards)
	})
	if index > cap(configs) {
		return nil
	}
	return &configs[index]
}

// GetGlobalConfig returns global config.
func GetGlobalConfig() *ConfigType {
	return GetConfigs(Global)
}

func (conf *ConfigType) String() string {
	return fmt.Sprintf("%s/%s/%s:%v,%v,%v:%v", conf.beacon, conf.group, conf.client, conf.isClient, conf.isBeacon, conf.isLeader, conf.shardID)
}

// SetBeaconGroupID set the groupID for beacon group
func (conf *ConfigType) SetBeaconGroupID(g p2p.GroupID) {
	conf.beacon = g
}

// SetShardGroupID set the groupID for shard group
func (conf *ConfigType) SetShardGroupID(g p2p.GroupID) {
	conf.group = g
}

// SetClientGroupID set the groupID for client group
func (conf *ConfigType) SetClientGroupID(g p2p.GroupID) {
	conf.client = g
}

// SetIsClient set the isClient configuration
func (conf *ConfigType) SetIsClient(b bool) {
	conf.isClient = b
}

// SetIsBeacon set the isBeacon configuration
func (conf *ConfigType) SetIsBeacon(b bool) {
	conf.isBeacon = b
}

// SetIsLeader set the isLeader configuration
func (conf *ConfigType) SetIsLeader(b bool) {
	conf.isLeader = b
}

// SetIsArchival set the isArchival configuration
func (conf *ConfigType) SetIsArchival(b bool) {
	conf.isArchival = b
}

// SetShardID set the shardID
func (conf *ConfigType) SetShardID(s uint32) {
	conf.shardID = s
}

// SetRole set the role
func (conf *ConfigType) SetRole(r Role) {
	conf.role = r
}

// GetBeaconGroupID returns the groupID for beacon group
func (conf *ConfigType) GetBeaconGroupID() p2p.GroupID {
	return conf.beacon
}

// GetShardGroupID returns the groupID for shard group
func (conf *ConfigType) GetShardGroupID() p2p.GroupID {
	return conf.group
}

// GetClientGroupID returns the groupID for client group
func (conf *ConfigType) GetClientGroupID() p2p.GroupID {
	return conf.client
}

// IsClient returns the isClient configuration
func (conf *ConfigType) IsClient() bool {
	return conf.isClient
}

// IsBeacon returns the isBeacon configuration
func (conf *ConfigType) IsBeacon() bool {
	return conf.isBeacon
}

// IsLeader returns the isLeader configuration
func (conf *ConfigType) IsLeader() bool {
	return conf.isLeader
}

// IsArchival returns the isArchival configuration
func (conf *ConfigType) IsArchival() bool {
	return conf.isArchival
}

// ShardID returns the shardID
func (conf *ConfigType) ShardID() uint32 {
	return conf.shardID
}

// Role returns the role
func (conf *ConfigType) Role() Role {
	return conf.role
}
