// Package harness constructs an offline core.BlockChain over an already-open
// database, with the process-global schedule state initialized from
// --network exactly as cmd/harmony/main.go does (plan WS1). It never
// initializes networking, RPC, txpool, consensus services, or BLS signing
// keys (in-place handoff §4 safety contract).
package harness

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
)

// Mode selects the cache configuration of the offline chain.
type Mode int

const (
	// ModeReadOnly opens for verification/inspection: archival-style, no
	// snapshots, and crucially Preimages=false because the preimage marker
	// write at open (core/blockchain_impl.go:376-380) would be refused by a
	// read-only handle.
	ModeReadOnly Mode = iota
	// ModeReplay opens the writable working copy archival-style with
	// preimages recorded, snapshots off (plan WS4 step 4).
	ModeReplay
)

// Networks supported by the harness. v1 is used on mainnet and localnet
// (fixtures); the remaining stock schedules are wired for completeness.
var schedules = map[string]shardingconfig.Schedule{
	"mainnet":   shardingconfig.MainnetSchedule,
	"testnet":   shardingconfig.TestnetSchedule,
	"pangaea":   shardingconfig.PangaeaSchedule,
	"partner":   shardingconfig.PartnerSchedule,
	"stressnet": shardingconfig.StressNetSchedule,
	"localnet":  shardingconfig.LocalnetSchedule,
}

var networkTypes = map[string]nodeconfig.NetworkType{
	"mainnet":   nodeconfig.Mainnet,
	"testnet":   nodeconfig.Testnet,
	"pangaea":   nodeconfig.Pangaea,
	"partner":   nodeconfig.Partner,
	"stressnet": nodeconfig.Stressnet,
	"localnet":  nodeconfig.Localnet,
}

// ensureLocalnet initializes the localnet instance config (once). Localnet
// schedule math (CalcEpochNumber/EpochLastBlock) panics with "localnet config
// is not set" until this runs, so both InitSchedule and the side-effect-free
// Schedule call it for localnet.
func ensureLocalnet(network string) {
	if network == "localnet" {
		shardingconfig.InitLocalnetConfig(
			nodeconfig.GetDefaultLocalnetBlocksPerEpoch(),
			nodeconfig.GetDefaultLocalnetBlocksPerEpochV2(),
		)
	}
}

// InitSchedule initializes the process-global schedule state (shard.Schedule,
// nodeconfig sharding schedule, localnet instance params) from the network
// name, mirroring cmd/harmony/main.go:188-195. Core execution reads these
// globals (e.g. core/offchain.go:46), so this must run before any chain use.
func InitSchedule(network string) (shardingconfig.Schedule, error) {
	sched, ok := schedules[network]
	if !ok {
		return nil, fmt.Errorf("harness: unsupported --network %q", network)
	}
	ensureLocalnet(network)
	shard.Schedule = sched
	nodeconfig.SetShardingSchedule(sched)
	return sched, nil
}

// Schedule returns the schedule for a network for read-only window math. For
// localnet it also initializes the localnet instance config (idempotent),
// which the schedule's epoch math requires; it does not set the process-wide
// shard.Schedule global (InitSchedule does that before any chain use).
func Schedule(network string) (shardingconfig.Schedule, error) {
	sched, ok := schedules[network]
	if !ok {
		return nil, fmt.Errorf("harness: unsupported --network %q", network)
	}
	ensureLocalnet(network)
	return sched, nil
}

// NetworkType maps the CLI network name to the node network type.
func NetworkType(network string) (nodeconfig.NetworkType, error) {
	nt, ok := networkTypes[network]
	if !ok {
		return "", fmt.Errorf("harness: unsupported --network %q", network)
	}
	return nt, nil
}

// ChainConfig returns a fresh copy of the built-in chain config for the
// network (shard-0 beacon semantics: EthCompatibleChainID reset like
// internal/shardchain/shardchains.go:130-133 does).
func ChainConfig(network string, shardID uint32) (*params.ChainConfig, error) {
	nt, err := NetworkType(network)
	if err != nil {
		return nil, err
	}
	cfg := nt.ChainConfig()
	if shardID == shard.BeaconChainShardID {
		cfg.EthCompatibleChainID = big.NewInt(cfg.EthCompatibleShard0ChainID.Int64())
	}
	return &cfg, nil
}

// OpenChain builds the offline BlockChain over db per
// internal/shardchain/shardchains.go:128-160 (shard 0, nil beacon = self).
// The caller owns db and must close it after the chain is done. No
// background services beyond BlockChainImpl's own internals are started.
func OpenChain(db ethdb.Database, network string, shardID uint32, mode Mode) (core.BlockChain, error) {
	if shardID != shard.BeaconChainShardID {
		return nil, fmt.Errorf("harness: v1 supports only shard 0 (got %d)", shardID)
	}
	if _, err := InitSchedule(network); err != nil {
		return nil, err
	}
	cfg, err := ChainConfig(network, shardID)
	if err != nil {
		return nil, err
	}
	var cacheConfig *core.CacheConfig
	switch mode {
	case ModeReadOnly:
		cacheConfig = &core.CacheConfig{
			Disabled:      true,
			Preimages:     false, // preimage marker write at open would be refused read-only
			SnapshotLimit: 0,
		}
	case ModeReplay:
		cacheConfig = &core.CacheConfig{
			Disabled:      true,
			Preimages:     true,
			SnapshotLimit: 0,
		}
	default:
		return nil, fmt.Errorf("harness: unknown mode %d", mode)
	}
	engine := chain.NewEngine()
	bc, err := core.NewBlockChainWithOptions(
		db, nil /* stateCache */, nil, /* beacon: shard 0 self-references */
		cacheConfig, cfg, engine, vm.Config{}, core.Options{},
	)
	if err != nil {
		return nil, fmt.Errorf("harness: open chain: %w", err)
	}
	if bc.ShardID() != shardID {
		return nil, fmt.Errorf("harness: database is shard %d, expected shard %d", bc.ShardID(), shardID)
	}
	return bc, nil
}
