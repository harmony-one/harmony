// Package nodeconfig includes all the configuration variables for a node.
// It is a global configuration for node and other services.
// It will be included in node module, and other modules.
package nodeconfig

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/webhooks"
	p2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
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

// Global is the index of the global node configuration
const (
	Global    = 0
	MaxShards = 32 // maximum number of shards. It is also the maxium number of configs.
)

var version string
var peerID peer.ID // PeerID of the node

// ConfigType is the structure of all node related configuration variables
type ConfigType struct {
	// The three groupID design, please refer to https://github.com/harmony-one/harmony/blob/master/node/node.md#libp2p-integration
	beacon                 GroupID             // the beacon group ID
	group                  GroupID             // the group ID of the shard (note: for beacon chain node, the beacon and shard group are the same)
	client                 GroupID             // the client group ID of the shard
	isClient               bool                // whether this node is a client node, such as wallet
	ShardID                uint32              // ShardID of this node; TODO ek – revisit when resharding
	role                   Role                // Role of the node
	Port                   string              // Port of the node.
	IP                     string              // IP of the node.
	RPCServer              RPCServerConfig     // RPC server port and ip
	RosettaServer          RosettaServerConfig // rosetta server port and ip
	IsOffline              bool
	Downloader             bool // Whether stream downloader is running; TODO: remove this after sync up
	StagedSync             bool // use staged sync
	StagedSyncTurboMode    bool // use Turbo mode for staged sync
	UseMemDB               bool // use mem db for staged sync
	DoubleCheckBlockHashes bool
	MaxBlocksPerSyncCycle  uint64 // Maximum number of blocks per each cycle. if set to zero, all blocks will be  downloaded and synced in one full cycle.
	MaxMemSyncCycleSize    uint64 // max number of blocks to use a single transaction for staged sync
	MaxBackgroundBlocks    uint64 // max number of background blocks in turbo mode
	InsertChainBatchSize   int    // number of blocks to build a batch and insert to chain in staged sync
	VerifyAllSig           bool   // verify signatures for all blocks regardless of height and batch size
	VerifyHeaderBatchSize  uint64 // batch size to verify header before insert to chain
	LogProgress            bool   // log the full sync progress in console
	NtpServer              string
	StringRole             string
	P2PPriKey              p2p_crypto.PrivKey   `json:"-"`
	ConsensusPriKey        multibls.PrivateKeys `json:"-"`
	// Database directory
	DBDir            string
	networkType      NetworkType
	shardingSchedule shardingconfig.Schedule
	DNSZone          string
	isArchival       map[uint32]bool
	WebHooks         struct {
		Hooks *webhooks.Hooks
	}
	TraceEnable bool
}

// RPCServerConfig is the config for rpc listen addresses
type RPCServerConfig struct {
	HTTPEnabled  bool
	HTTPIp       string
	HTTPPort     int
	HTTPAuthPort int

	HTTPTimeoutRead  time.Duration
	HTTPTimeoutWrite time.Duration
	HTTPTimeoutIdle  time.Duration

	WSEnabled  bool
	WSIp       string
	WSPort     int
	WSAuthPort int

	DebugEnabled bool

	EthRPCsEnabled     bool
	StakingRPCsEnabled bool
	LegacyRPCsEnabled  bool

	RpcFilterFile string

	RateLimiterEnabled bool
	RequestsPerSecond  int

	EvmCallTimeout time.Duration
}

// RosettaServerConfig is the config for the rosetta server
type RosettaServerConfig struct {
	HTTPEnabled bool
	HTTPIp      string
	HTTPPort    int
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
	return conf.isArchival[conf.ShardID]
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
// for beacon chain node, the archival variable will
// overrdie bcArchival as the shardID is the same
func (conf *ConfigType) SetArchival(bcArchival, archival bool) {
	if conf.isArchival == nil {
		conf.isArchival = make(map[uint32]bool)
	}
	conf.isArchival[shard.BeaconChainShardID] = bcArchival
	conf.isArchival[conf.ShardID] = archival
}

// ArchiveModes return the map of the archive setting
func (conf *ConfigType) ArchiveModes() map[uint32]bool {
	return conf.isArchival
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
	ensureShardConfigs()
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

// ShardIDFromConsensusKey returns the shard ID statically determined from the
// consensus key.
func (conf *ConfigType) ShardIDFromConsensusKey() (uint32, error) {
	return conf.ShardIDFromKey(conf.ConsensusPriKey[0].Pub.Object)
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
