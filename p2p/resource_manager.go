package p2p

import (
	"math"

	"github.com/harmony-one/harmony/internal/utils"

	humanize "github.com/dustin/go-humanize"
	libp2p "github.com/libp2p/go-libp2p"
	network "github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	memory "github.com/pbnjay/memory"
)

// GetNumFDs returns the File Descriptors as MaxInt.
func GetNumFDs() uint64 {
	return uint64(math.MaxUint64)
}

// mostly copy/pasted from https://github.com/ipfs/rainbow/blob/main/rcmgr.go
// which is itself copy-pasted from Kubo, because libp2p does not have
// a sane way of doing this.
func makeResourceMgr(enabled bool, maxMemory, maxFD uint64, connMgrHighWater int) (network.ResourceManager, error) {
	if !enabled {
		rmgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
		utils.Logger().Info().Msg("go-libp2p Resource Manager DISABLED")
		return rmgr, err
	}

	// Auto-scaled limits based on available memory/fds.
	if maxMemory == 0 {
		maxMemory = uint64((float64(memory.TotalMemory()) * 0.25))
		if maxMemory < 1<<20 { // 1 MiB
			maxMemory = 1 << 20
		}
	}
	if maxFD == 0 {
		maxFD = GetNumFDs() / 2
	}

	infiniteResourceLimits := rcmgr.InfiniteLimits.ToPartialLimitConfig().System
	maxMemoryMB := maxMemory / (1024 * 1024)

	// At least as of 2023-01-25, it's possible to open a connection that
	// doesn't ask for any memory usage with the libp2p Resource Manager/Accountant
	// (see https://github.com/libp2p/go-libp2p/issues/2010#issuecomment-1404280736).
	// As a result, we can't currently rely on Memory limits to full protect us.
	// Until https://github.com/libp2p/go-libp2p/issues/2010 is addressed,
	// we take a proxy now of restricting to 1 inbound connection per MB.
	// Note: this is more generous than go-libp2p's default autoscaled limits which do
	// 64 connections per 1GB
	// (see https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/limit_defaults.go#L357 ).
	systemConnsInbound := int(1 * maxMemoryMB)

	partialLimits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Memory: rcmgr.LimitVal64(maxMemory),
			FD:     rcmgr.LimitVal(maxFD),

			Conns:         rcmgr.Unlimited,
			ConnsInbound:  rcmgr.LimitVal(systemConnsInbound),
			ConnsOutbound: rcmgr.Unlimited,

			Streams:         rcmgr.Unlimited,
			StreamsOutbound: rcmgr.Unlimited,
			StreamsInbound:  rcmgr.Unlimited,
		},

		// Transient connections won't cause any memory to be accounted for by the resource manager/accountant.
		// Only established connections do.
		// As a result, we can't rely on System.Memory to protect us from a bunch of transient connection being opened.
		// We limit the same values as the System scope, but only allow the Transient scope to take 25% of what is allowed for the System scope.
		Transient: rcmgr.ResourceLimits{
			Memory: rcmgr.LimitVal64(maxMemory / 4),
			FD:     rcmgr.LimitVal(maxFD / 4),

			Conns:         rcmgr.Unlimited,
			ConnsInbound:  rcmgr.LimitVal(systemConnsInbound / 4),
			ConnsOutbound: rcmgr.Unlimited,

			Streams:         rcmgr.Unlimited,
			StreamsInbound:  rcmgr.Unlimited,
			StreamsOutbound: rcmgr.Unlimited,
		},

		// Lets get out of the way of the allow list functionality.
		// If someone specified "Swarm.ResourceMgr.Allowlist" we should let it go through.
		AllowlistedSystem: infiniteResourceLimits,

		AllowlistedTransient: infiniteResourceLimits,

		// Keep it simple by not having Service, ServicePeer, Protocol, ProtocolPeer, Conn, or Stream limits.
		ServiceDefault: infiniteResourceLimits,

		ServicePeerDefault: infiniteResourceLimits,

		ProtocolDefault: infiniteResourceLimits,

		ProtocolPeerDefault: infiniteResourceLimits,

		Conn: infiniteResourceLimits,

		Stream: infiniteResourceLimits,

		// Limit the resources consumed by a peer.
		// This doesn't protect us against intentional DoS attacks since an attacker can easily spin up multiple peers.
		// We specify this limit against unintentional DoS attacks (e.g., a peer has a bug and is sending too much traffic intentionally).
		// In that case we want to keep that peer's resource consumption contained.
		// To keep this simple, we only constrain inbound connections and streams.
		PeerDefault: rcmgr.ResourceLimits{
			Memory:          rcmgr.Unlimited64,
			FD:              rcmgr.Unlimited,
			Conns:           rcmgr.Unlimited,
			ConnsInbound:    rcmgr.DefaultLimit,
			ConnsOutbound:   rcmgr.Unlimited,
			Streams:         rcmgr.Unlimited,
			StreamsInbound:  rcmgr.DefaultLimit,
			StreamsOutbound: rcmgr.Unlimited,
		},
	}

	scalingLimitConfig := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&scalingLimitConfig)

	// Anything set above in partialLimits that had a value of rcmgr.DefaultLimit will be overridden.
	// Anything in scalingLimitConfig that wasn't defined in partialLimits above will be added (e.g., libp2p's default service limits).
	partialLimits = partialLimits.Build(scalingLimitConfig.Scale(int64(maxMemory), int(maxFD))).ToPartialLimitConfig()

	// Simple checks to override autoscaling ensuring limits make sense versus the connmgr values.
	// There are ways to break this, but this should catch most problems already.
	// We might improve this in the future.
	// See: https://github.com/ipfs/kubo/issues/9545
	if partialLimits.System.ConnsInbound > rcmgr.DefaultLimit {
		maxInboundConns := int(partialLimits.System.ConnsInbound)
		if connmgrHighWaterTimesTwo := connMgrHighWater * 2; maxInboundConns < connmgrHighWaterTimesTwo {
			maxInboundConns = connmgrHighWaterTimesTwo
		}

		if maxInboundConns < 800 {
			maxInboundConns = 800
		}

		// Scale System.StreamsInbound as well, but use the existing ratio of StreamsInbound to ConnsInbound
		if partialLimits.System.StreamsInbound > rcmgr.DefaultLimit {
			partialLimits.System.StreamsInbound = rcmgr.LimitVal(int64(maxInboundConns) * int64(partialLimits.System.StreamsInbound) / int64(partialLimits.System.ConnsInbound))
		}
		partialLimits.System.ConnsInbound = rcmgr.LimitVal(maxInboundConns)
	}

	utils.Logger().Info().Msgf("go-libp2p Resource Manager ENABLED: Limits based on max_memory: %s, max_fds: %d", humanize.Bytes(maxMemory), maxFD)

	// We already have a complete value thus pass in an empty ConcreteLimitConfig.
	limitCfg := partialLimits.Build(rcmgr.ConcreteLimitConfig{})

	mgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limitCfg))
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
