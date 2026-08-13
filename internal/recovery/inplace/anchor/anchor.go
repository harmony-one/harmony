// Package anchor holds the compiled-in incident constants for the shard-0
// in-place recovery preflight and derives the epoch geometry from the
// network schedule at runtime.
//
// The anchor is deliberately minimal: network, shard, target height and the
// operator-provided target block hash. Everything else (state root, epoch,
// ViewID, parent hash) is read from the locally stored target header, which
// must recompute (header.Hash()) to the anchored hash - the externally known
// block hash pins the entire header content, so there is no manifest file
// and nothing for validators to fetch or verify.
package anchor

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
)

// Compiled-in incident constants (mainnet shard 0).
const (
	// MainnetTargetHeight is the recovery target block height.
	MainnetTargetHeight uint64 = 92730034
	// MainnetTargetHashHex is the operator-provided target block hash
	// (authoritative, cross-checked against explorer/API at release time via
	// `go test -tags releasecheck ./internal/recovery/inplace/anchor/...`).
	MainnetTargetHashHex = "0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d"

	// MainnetAbandonedChildHashHex is the memo's abandoned child block
	// 92,730,035 hash. Informational only: the head-sample receipt rows
	// compare against it; it never gates any check.
	MainnetAbandonedChildHashHex = "0x5de06979a333f20afb8b245a8cf44472dc5bfc7383a57ddee48e1809bcee7c5d"
)

// MainnetTargetHash is the parsed operator-provided target hash.
var MainnetTargetHash = common.HexToHash(MainnetTargetHashHex)

// Anchor is the resolved verification anchor: compiled constants plus
// schedule-derived epoch geometry.
type Anchor struct {
	Network      nodeconfig.NetworkType
	ShardID      uint32
	TargetHeight uint64
	TargetHash   common.Hash

	// Derived from the schedule (never compiled in):
	Epoch          *big.Int // CalcEpochNumber(TargetHeight)
	BoundaryHeight uint64   // EpochLastBlock(Epoch-1): carries ss<Epoch>

	ChainConfig *params.ChainConfig
	Schedule    shardingconfig.Schedule
}

// Overrides are the hidden, test-only anchor overrides. On mainnet they are
// refused: the compiled constants are authoritative.
type Overrides struct {
	TargetHeight uint64 // 0 = unset
	TargetHash   string // "" = unset
}

// Resolve builds the anchor for the given network, applying test-only
// overrides on non-mainnet networks. It also installs the process-global
// shard.Schedule required by the committee/quorum code.
func Resolve(network string, shardID uint32, ov Overrides) (*Anchor, error) {
	networkType := nodeconfig.NetworkType(network)
	schedule, chainConfig, err := scheduleForNetwork(networkType)
	if err != nil {
		return nil, err
	}
	// Process-global schedule initialization, as cmd/harmony/main.go does;
	// votepower.Compute and committee reads consult shard.Schedule.
	shard.Schedule = schedule

	a := &Anchor{
		Network:     networkType,
		ShardID:     shardID,
		ChainConfig: chainConfig,
		Schedule:    schedule,
	}
	if networkType == nodeconfig.Mainnet {
		if ov.TargetHeight != 0 || ov.TargetHash != "" {
			return nil, fmt.Errorf("--target-height/--target-hash are test-only overrides and are refused on --network mainnet (compiled constants are authoritative)")
		}
		if shardID != 0 {
			return nil, fmt.Errorf("the compiled mainnet anchor is for shard 0; --shard %d is not supported", shardID)
		}
		a.TargetHeight = MainnetTargetHeight
		a.TargetHash = MainnetTargetHash
	} else {
		if ov.TargetHeight == 0 || ov.TargetHash == "" {
			return nil, fmt.Errorf("--network %s requires the test-only --target-height and --target-hash overrides", network)
		}
		if len(common.FromHex(ov.TargetHash)) != common.HashLength {
			return nil, fmt.Errorf("--target-hash %q is not a 32-byte hex hash", ov.TargetHash)
		}
		a.TargetHeight = ov.TargetHeight
		a.TargetHash = common.HexToHash(ov.TargetHash)
	}

	a.Epoch = schedule.CalcEpochNumber(a.TargetHeight)
	if a.Epoch.Sign() <= 0 {
		return nil, fmt.Errorf("target height %d resolves to epoch %s; the preflight requires a target above epoch 0", a.TargetHeight, a.Epoch)
	}
	if !chainConfig.IsStaking(a.Epoch) {
		return nil, fmt.Errorf("target epoch %s is pre-staking on %s; the certificate check requires a staking-era committee", a.Epoch, network)
	}
	a.BoundaryHeight = schedule.EpochLastBlock(a.Epoch.Uint64() - 1)
	// Cross-check the derived geometry against the schedule.
	if a.BoundaryHeight >= a.TargetHeight {
		return nil, fmt.Errorf("schedule inconsistency: boundary %d >= target %d", a.BoundaryHeight, a.TargetHeight)
	}
	if last := schedule.EpochLastBlock(a.Epoch.Uint64()); last < a.TargetHeight {
		return nil, fmt.Errorf("schedule inconsistency: target %d beyond epoch %s last block %d", a.TargetHeight, a.Epoch, last)
	}
	return a, nil
}

func scheduleForNetwork(nt nodeconfig.NetworkType) (shardingconfig.Schedule, *params.ChainConfig, error) {
	switch nt {
	case nodeconfig.Mainnet:
		return shardingconfig.MainnetSchedule, params.MainnetChainConfig, nil
	case nodeconfig.Testnet:
		return shardingconfig.TestnetSchedule, params.TestnetChainConfig, nil
	case nodeconfig.Localnet:
		// The localnet schedule needs its blocks-per-epoch configuration
		// installed before use (panics otherwise). 16/16 are the harmony
		// config defaults; fixtures are generated with the same values.
		shardingconfig.InitLocalnetConfig(16, 16)
		return shardingconfig.LocalnetSchedule, params.LocalnetChainConfig, nil
	case nodeconfig.Partner:
		return shardingconfig.PartnerSchedule, params.PartnerChainConfig, nil
	case nodeconfig.Stressnet:
		return shardingconfig.StressNetSchedule, params.StressnetChainConfig, nil
	case nodeconfig.Pangaea:
		return shardingconfig.PangaeaSchedule, params.PangaeaChainConfig, nil
	default:
		return nil, nil, fmt.Errorf("unsupported --network %q (mainnet, testnet, localnet, partner, stressnet, pangaea)", string(nt))
	}
}
