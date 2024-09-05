// Package nodeconfig includes all the configuration variables for a node.
// It is a global configuration for node and other services.
// It will be included in node module, and other modules.
package bootnode

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/webhooks"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

// Role defines a role of a node.
type Role byte

// All constants for different node roles.
const (
	Unknown Role = iota
	BootNode
)

func (role Role) String() string {
	switch role {
	case BootNode:
		return "BootNode"
	default:
		return "Unknown"
	}
}

// NetworkType describes the type of Harmony network
type NetworkType string

// Constants for NetworkType
// TODO: replace this with iota. Leave the string parsing in command line
const (
	Mainnet   = "mainnet"
	Testnet   = "testnet"
	Pangaea   = "pangaea"
	Partner   = "partner"
	Stressnet = "stressnet"
	Devnet    = "devnet"
	Localnet  = "localnet"
)

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

func (n NetworkType) String() string {
	if n == "" {
		return Testnet // default to testnet
	}
	return string(n)
}

var version string
var peerID peer.ID // PeerID of the node

// ConfigType is the structure of all node related configuration variables
type ConfigType struct {
	ShardID   uint32          // ShardID of this node; TODO ek – revisit when resharding
	role      Role            // Role of the node
	Port      string          // Port of the node.
	IP        string          // IP of the node.
	RPCServer RPCServerConfig // RPC server port and ip
	DebugMode bool            // log every single process and error to help to debug the syncing issues
	NtpServer string

	shardingSchedule shardingconfig.Schedule
	networkType      NetworkType
	DNSZone          string
	WebHooks         struct {
		Hooks *webhooks.Hooks
	}
	TraceEnable bool
}

// RPCServerConfig is the config for rpc listen addresses
type RPCServerConfig struct {
	HTTPEnabled bool
	HTTPIp      string
	HTTPPort    int

	HTTPTimeoutRead  time.Duration
	HTTPTimeoutWrite time.Duration
	HTTPTimeoutIdle  time.Duration

	WSEnabled bool
	WSIp      string
	WSPort    int

	DebugEnabled bool

	NetworkRPCsEnabled bool

	RpcFilterFile string

	RateLimiterEnabled bool
	RequestsPerSecond  int
}

// configs is a list of node configuration.
// It has at least one configuration.
// The first one is the default, global node configuration
var shardConfigs []ConfigType
var defaultConfig ConfigType

// GetDefaultConfig returns default config.
func GetDefaultConfig() *ConfigType {
	return &defaultConfig
}

// SetDefaultRole ..
func SetDefaultRole(r Role) {
	defaultConfig.role = r
}

func (conf *ConfigType) String() string {
	return fmt.Sprintf("%v", conf.ShardID)
}

// SetShardID set the ShardID
func (conf *ConfigType) SetShardID(s uint32) {
	conf.ShardID = s
}

// SetRole set the role
func (conf *ConfigType) SetRole(r Role) {
	conf.role = r
}

// GetShardID returns the shardID.
func (conf *ConfigType) GetShardID() uint32 {
	return conf.ShardID
}

// Role returns the role
func (conf *ConfigType) Role() Role {
	return conf.role
}

// SetNetworkType set the networkType
func SetNetworkType(networkType NetworkType) {
	defaultConfig.networkType = networkType
	for i := range shardConfigs {
		shardConfigs[i].networkType = networkType
	}
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

// SetPeerID set the peer ID of the node
func SetPeerID(pid peer.ID) {
	peerID = pid
}

// GetPeerID returns the peer ID of the node
func GetPeerID() peer.ID {
	return peerID
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
	defaultConfig.SetShardingSchedule(schedule)
	for _, config := range shardConfigs {
		config.SetShardingSchedule(schedule)
	}
}

// ShardIDFromKey returns the shard ID statically determined from the input key
func (conf *ConfigType) ShardIDFromKey(key *bls_core.PublicKey) (uint32, error) {
	var pubKey bls.SerializedPublicKey
	if err := pubKey.FromLibBLSPublicKey(key); err != nil {
		return 0, errors.Wrapf(err,
			"cannot convert libbls public key %s to internal form",
			key.SerializeToHexStr())
	}
	epoch := conf.networkType.ChainConfig().StakingEpoch
	numShards := conf.shardingSchedule.InstanceForEpoch(epoch).NumShards()
	shardID := new(big.Int).Mod(pubKey.Big(), big.NewInt(int64(numShards)))
	return uint32(shardID.Uint64()), nil
}

// ValidateConsensusKeysForSameShard checks if all consensus public keys belong to the same shard
func (conf *ConfigType) ValidateConsensusKeysForSameShard(pubkeys multibls.PublicKeys, sID uint32) error {
	keyShardStrs := []string{}
	isSameShard := true
	for _, key := range pubkeys {
		shardID, err := conf.ShardIDFromKey(key.Object)
		if err != nil {
			return err
		}
		if shardID != sID {
			isSameShard = false
		}
		keyShardStrs = append(
			keyShardStrs,
			fmt.Sprintf("key: %s, shard id: %d", key.Bytes.Hex(), shardID),
		)
	}
	if !isSameShard {
		return errors.Errorf("bls keys do not belong to same shard\n%s", strings.Join(keyShardStrs, "\n"))
	}
	return nil
}
