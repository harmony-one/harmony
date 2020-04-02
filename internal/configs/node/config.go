// Package nodeconfig includes all the configuration variables for a node.
// It is a global configuration for node and other services.
// It will be included in node module, and other modules.
package nodeconfig

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/harmony-one/bls/ffi/go/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/webhooks"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
)

// Role defines a role of a node.
type Role byte

// All constants for different node roles.
const (
	Unknown Role = iota
	Validator
	ExplorerNode
)

func (role Role) String() string {
	switch role {
	case Validator:
		return "Validator"
	case ExplorerNode:
		return "ExplorerNode"
	default:
		return "Unknown"
	}
}

// NetworkType describes the type of Harmony network
type NetworkType string

// Constants for NetworkType
const (
	Mainnet   = "mainnet"
	Testnet   = "testnet"
	Pangaea   = "pangaea"
	Partner   = "partner"
	Stressnet = "stressnet"
	Devnet    = "devnet"
	Localnet  = "localnet"
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
	beacon          GroupID // the beacon group ID
	group           GroupID // the group ID of the shard (note: for beacon chain node, the beacon and shard group are the same)
	client          GroupID // the client group ID of the shard
	isClient        bool    // whether this node is a client node, such as wallet
	isBeacon        bool    // whether this node is beacon node doing consensus or not
	ShardID         uint32  // ShardID of this node; TODO ek – revisit when resharding
	role            Role    // Role of the node
	Port            string  // Port of the node.
	IP              string  // IP of the node.
	StringRole      string
	P2PPriKey       p2p_crypto.PrivKey
	ConsensusPriKey *multibls.PrivateKey
	ConsensusPubKey *multibls.PublicKey
	// Database directory
	DBDir            string
	networkType      NetworkType
	shardingSchedule shardingconfig.Schedule
	DNSZone          string
	isArchival       bool
	WebHooks         struct {
		Hooks *webhooks.Hooks
	}
}

// configs is a list of node configuration.
// It has at least one configuration.
// The first one is the default, global node configuration
var shardConfigs []ConfigType
var defaultConfig ConfigType
var onceForConfigs sync.Once

func ensureShardConfigs() {
	onceForConfigs.Do(func() {
		shardConfigs = make([]ConfigType, MaxShards)
		for i := range shardConfigs {
			shardConfigs[i].ShardID = uint32(i)
		}
	})
}

// GetShardConfig return the shard's ConfigType variable
func GetShardConfig(shardID uint32) *ConfigType {
	ensureShardConfigs()
	if int(shardID) >= cap(shardConfigs) {
		return nil
	}
	return &shardConfigs[shardID]
}

// GetDefaultConfig returns default config.
func GetDefaultConfig() *ConfigType {
	return &defaultConfig
}

// SetDefaultRole ..
func SetDefaultRole(r Role) {
	defaultConfig.role = r
}

func (conf *ConfigType) String() string {
	return fmt.Sprintf("%s/%s/%s:%v,%v", conf.beacon, conf.group, conf.client, conf.isClient, conf.ShardID)
}

// SetBeaconGroupID set the groupID for beacon group
func (conf *ConfigType) SetBeaconGroupID(g GroupID) {
	conf.beacon = g
}

// SetShardGroupID set the groupID for shard group
func (conf *ConfigType) SetShardGroupID(g GroupID) {
	conf.group = g
}

// SetClientGroupID set the groupID for client group
func (conf *ConfigType) SetClientGroupID(g GroupID) {
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

// GetBeaconGroupID returns the groupID for beacon group
func (conf *ConfigType) GetBeaconGroupID() GroupID {
	return conf.beacon
}

// GetShardGroupID returns the groupID for shard group
func (conf *ConfigType) GetShardGroupID() GroupID {
	return conf.group
}

// GetShardID returns the shardID.
func (conf *ConfigType) GetShardID() uint32 {
	return conf.ShardID
}

// GetClientGroupID returns the groupID for client group
func (conf *ConfigType) GetClientGroupID() GroupID {
	return conf.client
}

// GetArchival returns archival mode
func (conf *ConfigType) GetArchival() bool {
	return conf.isArchival
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
func SetNetworkType(networkType NetworkType) {
	ensureShardConfigs()
	defaultConfig.networkType = networkType
	for i := range shardConfigs {
		shardConfigs[i].networkType = networkType
	}
}

// SetArchival set archival mode
func (conf *ConfigType) SetArchival(archival bool) {
	defaultConfig.isArchival = archival
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

// ShardingSchedule returns the sharding schedule for this node config.
func (conf *ConfigType) ShardingSchedule() shardingconfig.Schedule {
	return conf.shardingSchedule
}

// SetShardingSchedule sets the sharding schedule for this node config.
func (conf *ConfigType) SetShardingSchedule(schedule shardingconfig.Schedule) {
	conf.shardingSchedule = schedule
}

// SetShardingSchedule sets the sharding schedule for all config instances.
func SetShardingSchedule(schedule shardingconfig.Schedule) {
	ensureShardConfigs()
	defaultConfig.SetShardingSchedule(schedule)
	for _, config := range shardConfigs {
		config.SetShardingSchedule(schedule)
	}
}

// ShardIDFromConsensusKey returns the shard ID statically determined from the
// consensus key.
func (conf *ConfigType) ShardIDFromConsensusKey() (uint32, error) {
	var pubKey shard.BLSPublicKey
	// all keys belong to same shard
	if err := pubKey.FromLibBLSPublicKey(conf.ConsensusPubKey.PublicKey[0]); err != nil {
		return 0, errors.Wrapf(err,
			"cannot convert libbls public key %s to internal form",
			conf.ConsensusPubKey.SerializeToHexStr())
	}
	epoch := conf.networkType.ChainConfig().StakingEpoch
	numShards := conf.shardingSchedule.InstanceForEpoch(epoch).NumShards()
	shardID := new(big.Int).Mod(pubKey.Big(), big.NewInt(int64(numShards)))
	return uint32(shardID.Uint64()), nil
}

// ValidateConsensusKeysForSameShard checks if all consensus public keys belong to the same shard
func (conf *ConfigType) ValidateConsensusKeysForSameShard(pubkeys []*bls.PublicKey, sID uint32) error {
	var pubKey shard.BLSPublicKey
	for _, key := range pubkeys {
		if err := pubKey.FromLibBLSPublicKey(key); err != nil {
			return errors.Wrapf(err,
				"cannot convert libbls public key %s to internal form",
				key.SerializeToHexStr())
		}
		epoch := conf.networkType.ChainConfig().StakingEpoch
		numShards := conf.shardingSchedule.InstanceForEpoch(epoch).NumShards()
		shardID := new(big.Int).Mod(pubKey.Big(), big.NewInt(int64(numShards)))
		if uint32(shardID.Uint64()) != sID {
			return errors.New("bls keys do not belong to the same shard")
		}
	}
	return nil
}

// ChainConfig returns the chain configuration for the network type.
func (t NetworkType) ChainConfig() params.ChainConfig {
	switch t {
	case Mainnet:
		return *params.MainnetChainConfig
	case Pangaea:
		return *params.PangaeaChainConfig
	case Partner:
		return *params.PartnerChainConfig
	case Stressnet:
		return *params.StressnetChainConfig
	case Localnet:
		return *params.LocalnetChainConfig
	default:
		return *params.TestnetChainConfig
	}
}
