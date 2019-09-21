// Package nodeconfig includes all the configuration variables for a node.
// It is a global configuration for node and other services.
// It will be included in node module, and other modules.
package nodeconfig

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"

	"github.com/harmony-one/bls/ffi/go/bls"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"

	"github.com/harmony-one/harmony/p2p"
)

// Role defines a role of a node.
type Role byte

// All constants for different node roles.
const (
	Unknown Role = iota
	Validator
	ClientNode
	WalletNode
	ExplorerNode
)

func (role Role) String() string {
	switch role {
	case Unknown:
		return "Unknown"
	case Validator:
		return "Validator"
	case ClientNode:
		return "ClientNode"
	case WalletNode:
		return "WalletNode"
	case ExplorerNode:
		return "ExplorerNode"
	}
	return "Unknown"
}

// NetworkType describes the type of Harmony network
type NetworkType string

// Constants for NetworkType
const (
	Mainnet  = "mainnet"
	Testnet  = "testnet"
	Pangaea  = "pangaea"
	Devnet   = "devnet"
	Localnet = "localnet"
)

// Global is the index of the global node configuration
const (
	Global    = 0
	MaxShards = 32 // maximum number of shards. It is also the maxium number of configs.
)

var version string
var publicRPC bool // enable public RPC access

// ConfigType is the structure of all node related configuration variables
type ConfigType struct {
	// The three groupID design, please refer to https://github.com/harmony-one/harmony/blob/master/node/node.md#libp2p-integration
	beacon   p2p.GroupID // the beacon group ID
	group    p2p.GroupID // the group ID of the shard (note: for beacon chain node, the beacon and shard group are the same)
	client   p2p.GroupID // the client group ID of the shard
	isClient bool        // whether this node is a client node, such as wallet/txgen
	isBeacon bool        // whether this node is beacon node doing consensus or not
	ShardID  uint32      // ShardID of this node; TODO ek – reviisit when resharding
	role     Role        // Role of the node
	Port     string      // Port of the node.
	IP       string      // IP of the node.

	MetricsFlag     bool   // collect and upload metrics flag
	PushgatewayIP   string // metrics pushgateway prometheus ip
	PushgatewayPort string // metrics pushgateway prometheus port
	StringRole      string
	Host            p2p.Host
	StakingPriKey   *ecdsa.PrivateKey
	P2pPriKey       p2p_crypto.PrivKey
	ConsensusPriKey *bls.SecretKey
	ConsensusPubKey *bls.PublicKey

	// Database directory
	DBDir string

	SelfPeer p2p.Peer
	Leader   p2p.Peer

	networkType NetworkType
}

// configs is a list of node configuration.
// It has at least one configuration.
// The first one is the default, global node configuration
var shardConfigs []ConfigType
var defaultConfig ConfigType
var onceForConfigs sync.Once

// GetShardConfig return the shard's ConfigType variable
func GetShardConfig(shardID uint32) *ConfigType {
	onceForConfigs.Do(func() {
		shardConfigs = make([]ConfigType, MaxShards)
		for i := range shardConfigs {
			shardConfigs[i].ShardID = uint32(i)
		}
	})
	if int(shardID) >= cap(shardConfigs) {
		return nil
	}
	return &shardConfigs[shardID]
}

// SetConfigs set ConfigType in the right index.
func SetConfigs(config ConfigType, shardID uint32) error {
	onceForConfigs.Do(func() {
		shardConfigs = make([]ConfigType, MaxShards)
	})
	if int(shardID) >= cap(shardConfigs) {
		return errors.New("Failed to set ConfigType")
	}
	shardConfigs[int(shardID)] = config
	return nil
}

// GetDefaultConfig returns default config.
func GetDefaultConfig() *ConfigType {
	return &defaultConfig
}

func (conf *ConfigType) String() string {
	return fmt.Sprintf("%s/%s/%s:%v,%v", conf.beacon, conf.group, conf.client, conf.isClient, conf.ShardID)
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

// SetShardID set the ShardID
func (conf *ConfigType) SetShardID(s uint32) {
	conf.ShardID = s
}

// SetRole set the role
func (conf *ConfigType) SetRole(r Role) {
	conf.role = r
}

// SetPushgatewayIP set the pushgateway ip
func (conf *ConfigType) SetPushgatewayIP(ip string) {
	conf.PushgatewayIP = ip
}

// SetPushgatewayPort set the pushgateway port
func (conf *ConfigType) SetPushgatewayPort(port string) {
	conf.PushgatewayPort = port
}

// SetMetricsFlag set the metrics flag
func (conf *ConfigType) SetMetricsFlag(flag bool) {
	conf.MetricsFlag = flag
}

// GetMetricsFlag get the metrics flag
func (conf *ConfigType) GetMetricsFlag() bool {
	return conf.MetricsFlag
}

// GetPushgatewayIP get the pushgateway ip
func (conf *ConfigType) GetPushgatewayIP() string {
	return conf.PushgatewayIP
}

// GetPushgatewayPort get the pushgateway port
func (conf *ConfigType) GetPushgatewayPort() string {
	return conf.PushgatewayPort
}

// GetBeaconGroupID returns the groupID for beacon group
func (conf *ConfigType) GetBeaconGroupID() p2p.GroupID {
	return conf.beacon
}

// GetShardGroupID returns the groupID for shard group
func (conf *ConfigType) GetShardGroupID() p2p.GroupID {
	return conf.group
}

// GetShardID returns the shardID.
func (conf *ConfigType) GetShardID() uint32 {
	return conf.ShardID
}

// GetClientGroupID returns the groupID for client group
func (conf *ConfigType) GetClientGroupID() p2p.GroupID {
	return conf.client
}

// IsClient returns the isClient configuration
func (conf *ConfigType) IsClient() bool {
	return conf.isClient
}

// Role returns the role
func (conf *ConfigType) Role() Role {
	return conf.role
}

// SetNetworkType set the networkType
func (conf *ConfigType) SetNetworkType(networkType NetworkType) {
	conf.networkType = networkType
}

// GetNetworkType gets the networkType
func (conf *ConfigType) GetNetworkType() NetworkType {
	return conf.networkType
}

// SetVersion set the version of the node binary
func SetVersion(ver string) {
	version = ver
}

// GetVersion return the version of the node binary
func GetVersion() string {
	return version
}

// SetPublicRPC set the boolean value of public RPC access
func SetPublicRPC(v bool) {
	publicRPC = v
}

// GetPublicRPC get the boolean value of public RPC access
func GetPublicRPC() bool {
	return publicRPC
}
