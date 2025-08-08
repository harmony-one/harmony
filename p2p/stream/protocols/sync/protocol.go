package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/consensus/engine"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/discovery"
	"github.com/harmony-one/harmony/p2p/stream/common/ratelimiter"
	"github.com/harmony-one/harmony/p2p/stream/common/requestmanager"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/hashicorp/go-version"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"
)

const (
	// serviceSpecifier is the specifier for the service.
	SyncServiceSpecifier      = "sync"
	EpochSyncServiceSpecifier = "epochsync"
)

var (
	version100, _ = version.NewVersion("1.0.0")

	// MyVersion is the version of sync protocol of the local node
	MyVersion = version100

	// MinVersion is the minimum version for matching function
	MinVersion = version100
)

type (
	// Protocol is the protocol for sync streaming
	Protocol struct {
		chain    engine.ChainReader            // provide SYNC data
		schedule shardingconfig.Schedule       // provide schedule information
		rl       ratelimiter.RateLimiter       // limit the incoming request rate
		sm       streammanager.StreamManager   // stream management
		rm       requestmanager.RequestManager // deliver the response from stream
		disc     discovery.Discovery

		lastAdvertiseDuration    time.Duration // last advertise duration to adjust it dynamically
		recentPeerDiscoveryCount int           // recent peer discovery count
		startupMode              bool          // true during first 10 minutes for faster advertisement
		startupStartTime         time.Time     // when startup mode began

		config Config
		logger zerolog.Logger

		ctx    context.Context
		cancel func()
		closeC chan struct{}
	}

	// Config is the sync protocol config
	Config struct {
		Chain      engine.ChainReader
		Host       p2p_host.Host
		Discovery  discovery.Discovery
		ShardID    nodeconfig.ShardID
		Network    nodeconfig.NetworkType
		BeaconNode bool
		Validator  bool
		Explorer   bool
		EpochChain bool

		MaxAdvertiseWaitTime int
		// stream manager config
		SmSoftLowCap int
		SmHardLowCap int
		SmHiCap      int
		DiscBatch    int
	}
)

// NewProtocol creates a new sync protocol
func NewProtocol(config Config) *Protocol {
	ctx, cancel := context.WithCancel(context.Background())

	sp := &Protocol{
		chain:            config.Chain,
		disc:             config.Discovery,
		config:           config,
		ctx:              ctx,
		cancel:           cancel,
		closeC:           make(chan struct{}),
		startupMode:      true,
		startupStartTime: time.Now(),
	}
	smConfig := streammanager.Config{
		SoftLoCap: config.SmSoftLowCap,
		HardLoCap: config.SmHardLowCap,
		HiCap:     config.SmHiCap,
		DiscBatch: config.DiscBatch,
		TrustedPeers: func() map[libp2p_peer.ID]struct{} {
			tmp := make(map[libp2p_peer.ID]struct{})
			h := config.Host.(p2p.Host)
			for _, id := range h.TrustedPeers() {
				tmp[id] = struct{}{}
			}
			return tmp
		}(),
	}
	sp.sm = streammanager.NewStreamManager(sp.ProtoID(), config.Host.GetP2PHost(), config.Discovery,
		sp.HandleStream, smConfig)

	// Set callback to exit startup mode when enough streams are found
	sp.sm.SetEnoughStreamsCallback(func() {
		sp.ExitStartupMode()
	})

	sp.rl = ratelimiter.NewRateLimiter(sp.sm, rateLimiterGlobalRequestPerSecond, rateLimiterSingleRequestsPerSecond)

	sp.rm = requestmanager.NewRequestManager(sp.sm, sp.ProtoID())

	// if it is not epoch chain, print the peer id and proto id
	if !config.EpochChain {
		fmt.Println("My peer id: ", config.Host.GetID().String())
		fmt.Println("My proto id: ", sp.ProtoID())
	}

	sp.logger = utils.Logger().With().Str("Protocol", string(sp.ProtoID())).Logger()
	return sp
}

// Start starts the sync protocol
func (p *Protocol) Start() {
	p.sm.Start()
	p.rm.Start()
	p.rl.Start()
	go p.advertiseLoop()
}

// Close close the protocol
func (p *Protocol) Close() {
	p.rl.Close()
	p.rm.Close()
	p.sm.Close()
	p.cancel()
	close(p.closeC)
}

// ProtoID return the ProtoID of the sync protocol
func (p *Protocol) ProtoID() sttypes.ProtoID {
	return p.protoIDByVersion(MyVersion)
}

// ServiceID returns the service ID of the sync protocol
func (p *Protocol) ServiceID() string {
	serviceID := SyncServiceSpecifier
	if p.config.EpochChain {
		serviceID = EpochSyncServiceSpecifier
	}
	return serviceID
}

// Version returns the sync protocol version
func (p *Protocol) Version() *version.Version {
	return MyVersion
}

// IsBeaconValidator returns true if it is a beacon chain validator
func (p *Protocol) IsBeaconValidator() bool {
	return p.config.BeaconNode && p.config.Validator
}

// IsEpochChain returns true if it is a epoch chain
func (p *Protocol) IsEpochChain() bool {
	return p.config.EpochChain
}

// IsValidator returns true if it is a validator node
func (p *Protocol) IsValidator() bool {
	return p.config.Validator
}

// IsExplorer returns true if it is an explorer node
func (p *Protocol) IsExplorer() bool {
	return p.config.Explorer
}

// Match checks the compatibility to the target protocol ID.
func (p *Protocol) Match(targetID protocol.ID) bool {
	target, err := sttypes.ProtoIDToProtoSpec(sttypes.ProtoID(targetID))
	if err != nil {
		return false
	}
	if target.Service != p.ServiceID() {
		return false
	}
	if target.NetworkType != p.config.Network {
		return false
	}
	if target.ShardID != p.config.ShardID {
		return false
	}
	if target.Version.LessThan(MinVersion) {
		return false
	}
	return true
}

// HandleStream is the stream handle function being registered to libp2p.
func (p *Protocol) HandleStream(raw libp2p_network.Stream) {
	p.logger.Info().Str("stream", raw.ID()).Msg("handle new sync stream")
	st := p.wrapStream(raw)
	if err := p.sm.NewStream(st); err != nil {
		// Possibly we have reach the hard limit of the stream
		if !errors.Is(err, streammanager.ErrStreamAlreadyExist) && !errors.Is(err, streammanager.ErrStreamRemovalNotExpired) {
			p.logger.Warn().Err(err).Str("stream ID", string(st.ID())).
				Msg("failed to add new stream")
		}
		return
	}
	//to get my ID use raw.Conn().LocalPeer().String()
	p.logger.Info().Msgf("Connected to %s (%s)", raw.Conn().RemotePeer().String(), st.ProtoID())
	st.run()
}

func (p *Protocol) advertiseLoop() {
	// Use constants for sleep time boundaries
	var minSleepTime, maxSleepTime time.Duration
	if p.startupMode {
		minSleepTime = MinSleepTimeStartup
		maxSleepTime = MaxSleepTimeStartup
	} else {
		minSleepTime = MinSleepTimeNormal
		maxSleepTime = MaxSleepTimeNormal
	}

	for {
		sleep := p.advertise()

		// Adaptive sleep: Increase if new peers were found, decrease otherwise
		if p.recentPeerDiscoveryCount > 0 {
			// Increase sleep time based on number of peers found
			sleep += time.Duration(p.recentPeerDiscoveryCount) * SleepIncreasePerPeer
		} else {
			// Decrease sleep time by 30% when no peers found (better than /= 2)
			sleep = time.Duration(float64(sleep) * SleepDecreaseRatio)
		}

		// Enforce sleep boundaries based on startup mode
		if sleep < minSleepTime {
			sleep = minSleepTime
		} else if sleep > maxSleepTime {
			sleep = maxSleepTime
		}

		p.logger.Debug().
			Dur("sleep", sleep).
			Int("peersFound", p.recentPeerDiscoveryCount).
			Bool("startupMode", p.startupMode).
			Msg("[Protocol] advertisement loop sleeping")

		select {
		case <-time.After(sleep):
			// Continue to next iteration
		case <-p.closeC:
			return
		}
	}
}

// isValidPeer checks if a discovered peer is valid for our use case
// TODO: Implement more sophisticated validation logic
func (p *Protocol) isValidPeer(peer libp2p_peer.AddrInfo) bool {
	// For now, consider all non-self peers as potentially valid
	// In the future, we could add validation for:
	// - Same network type (mainnet vs testnet)
	// - Same shard ID
	// - Active status (not banned/offline)
	// - Protocol compatibility
	// - Geographic location (for latency optimization)
	// - Node type (validator vs non-validator)

	// Basic validation: peer should have addresses
	if len(peer.Addrs) == 0 {
		return false
	}

	// TODO: Add more validation logic here
	// Example validations:
	// - Check if peer is from same network
	// - Check if peer is from same shard
	// - Check if peer is active (not banned)
	// - Check protocol compatibility

	return true
}

// getDHTRequestLimit returns how many peers to request from DHT
// This should be higher than target because DHT may return invalid peers
func (p *Protocol) getDHTRequestLimit() int {
	switch p.config.Network {
	case nodeconfig.Mainnet:
		return DHTRequestLimitMainnet
	case nodeconfig.Testnet:
		return DHTRequestLimitTestnet
	case nodeconfig.Pangaea:
		return DHTRequestLimitPangaea
	case nodeconfig.Partner:
		return DHTRequestLimitPartner
	case nodeconfig.Stressnet:
		return DHTRequestLimitStressnet
	case nodeconfig.Devnet:
		return DHTRequestLimitDevnet
	case nodeconfig.Localnet:
		return DHTRequestLimitLocalnet
	default:
		return DHTRequestLimitDevnet
	}
}

// getTargetValidPeers returns how many valid peers we want to find
func (p *Protocol) getTargetValidPeers() int {
	switch p.config.Network {
	case nodeconfig.Mainnet:
		return TargetValidPeersMainnet
	case nodeconfig.Testnet:
		return TargetValidPeersTestnet
	case nodeconfig.Pangaea:
		return TargetValidPeersPangaea
	case nodeconfig.Partner:
		return TargetValidPeersPartner
	case nodeconfig.Stressnet:
		return TargetValidPeersStressnet
	case nodeconfig.Devnet:
		return TargetValidPeersDevnet
	case nodeconfig.Localnet:
		return TargetValidPeersLocalnet
	default:
		return TargetValidPeersDevnet
	}
}

// getPeerDiscoveryLimit returns the appropriate peer discovery limit based on network type
// DEPRECATED: Use getDHTRequestLimit() and getTargetValidPeers() instead
func (p *Protocol) getPeerDiscoveryLimit() int {
	return p.getDHTRequestLimit()
}

// advertise will advertise all compatible protocol versions for helping nodes running low version
func (p *Protocol) advertise() time.Duration {
	var nextWait time.Duration
	newPeersDiscovered := false
	maxRetries := 3

	// Check if we should exit startup mode (after 10 minutes)
	if p.startupMode && time.Since(p.startupStartTime) > StartupModeDuration {
		p.startupMode = false
		p.logger.Info().Msg("Exiting startup mode, switching to normal advertisement timing")
	}

	// Use timing constants based on startup mode
	var baseTimeout, timeoutIncrementStep, maxTimeout, backoffTimeRatio, maxBackoff time.Duration
	if p.startupMode {
		baseTimeout = BaseTimeoutStartup
		timeoutIncrementStep = TimeoutIncrementStepStartup
		maxTimeout = MaxTimeoutStartup
		backoffTimeRatio = BackoffTimeRatioStartup
		maxBackoff = MaxBackoffStartup
	} else {
		baseTimeout = BaseTimeoutNormal
		timeoutIncrementStep = TimeoutIncrementStepNormal
		maxTimeout = MaxTimeoutNormal
		backoffTimeRatio = BackoffTimeRatioNormal
		maxBackoff = MaxBackoffNormal
	}

	// First, advertise our service for each supported protocol version
	for _, pid := range p.supportedProtoIDs() {
		ctx, cancel := context.WithTimeout(p.ctx, baseTimeout)
		w, err := p.disc.Advertise(ctx, string(pid))
		cancel()

		if err != nil {
			p.logger.Debug().Err(err).Str("protocol", string(pid)).Msg("[Protocol] advertise failed")
		} else {
			p.logger.Debug().Str("protocol", string(pid)).Msg("[Protocol] advertise successful")
		}

		// Set the next wait time based on the response
		if nextWait == 0 || nextWait > w {
			nextWait = w
		}
	}

	// Then try to find peers with exponential backoff
	for i := 0; i < maxRetries; i++ {
		timeout := baseTimeout + time.Duration(i)*timeoutIncrementStep
		if timeout > maxTimeout {
			timeout = maxTimeout
		}

		ctx, cancel := context.WithTimeout(p.ctx, timeout)
		peers, err := p.disc.FindPeers(ctx, string(p.ProtoID()), p.getDHTRequestLimit())
		cancel()

		if err != nil {
			p.logger.Warn().Err(err).Msg("[Protocol] failed to find peers")
			nextWait = backoffTimeRatio + time.Duration(i)*timeoutIncrementStep
			if nextWait > maxBackoff {
				nextWait = maxBackoff
			}
			continue
		}

		// Count new peers discovered and filter for valid peers
		newPeersCount := 0
		validPeersFound := 0
		targetValidPeers := p.getTargetValidPeers()

		for peer := range peers {
			// Skip self if host is available (nil check for test scenarios)
			if p.config.Host != nil && p.config.Host.GetP2PHost() != nil {
				if peer.ID == p.config.Host.GetP2PHost().ID() {
					continue // Skip self
				}
			}

			newPeersCount++

			// TODO: Add more sophisticated peer validation here
			// For now, we consider all non-self peers as potentially valid
			// In the future, we could add validation for:
			// - Same network type
			// - Same shard
			// - Active status
			// - Protocol compatibility
			if p.isValidPeer(peer) {
				validPeersFound++
			}
		}

		// Log discovery results
		p.logger.Debug().
			Int("totalPeers", newPeersCount).
			Int("validPeers", validPeersFound).
			Int("targetValid", targetValidPeers).
			Msg("[Protocol] peer discovery results")

		if validPeersFound >= targetValidPeers {
			newPeersDiscovered = true
			p.recentPeerDiscoveryCount = validPeersFound
			p.logger.Info().Int("validPeers", validPeersFound).Msg("[Protocol] found sufficient valid peers")
			break
		} else if newPeersCount > 0 {
			// Found some peers but not enough valid ones
			p.recentPeerDiscoveryCount = validPeersFound
			p.logger.Debug().Int("validPeers", validPeersFound).Int("target", targetValidPeers).Msg("[Protocol] found peers but not enough valid ones")
		}

		// No peers found, use exponential backoff
		nextWait = backoffTimeRatio + time.Duration(i)*timeoutIncrementStep
		if nextWait > maxBackoff {
			nextWait = maxBackoff
		}
	}

	if !newPeersDiscovered {
		p.recentPeerDiscoveryCount = 0
		p.logger.Debug().Msg("[Protocol] no new peers found during advertisement")
	}

	return nextWait
}

func (p *Protocol) supportedProtoIDs() []sttypes.ProtoID {
	vs := p.supportedVersions()

	pids := make([]sttypes.ProtoID, 0, len(vs))
	for _, v := range vs {
		pids = append(pids, p.protoIDByVersion(v))
	}
	return pids
}

func (p *Protocol) supportedVersions() []*version.Version {
	return []*version.Version{version100}
}

func (p *Protocol) protoIDByVersion(v *version.Version) sttypes.ProtoID {
	spec := sttypes.ProtoSpec{
		Service:     p.ServiceID(),
		NetworkType: p.config.Network,
		ShardID:     p.config.ShardID,
		Version:     v,
	}
	return spec.ToProtoID()
}

// RemoveStream removes the stream of the given stream ID
// TODO: add reason to parameters
func (p *Protocol) RemoveStream(stID sttypes.StreamID, reason string) {
	st, exist := p.sm.GetStreamByID(stID)
	if exist && st != nil {
		//TODO: log this incident with reason
		st.Close(reason, true)
		p.logger.Info().
			Str("stream ID", string(stID)).
			Str("reason", reason).
			Msg("stream removed")
	}
}

func (p *Protocol) StreamFailed(stID sttypes.StreamID, reason string) {
	st, exist := p.sm.GetStreamByID(stID)
	if exist && st != nil {
		st.AddFailedTimes(FaultRecoveryThreshold)
		p.logger.Info().
			Str("stream ID", string(st.ID())).
			Int32("num failures", st.Failures()).
			Str("reason", reason).
			Msg("stream failed")
		if st.Failures() >= MaxStreamFailures {
			st.Close("too many failures", true)
			p.logger.Warn().
				Str("stream ID", string(st.ID())).
				Str("reason", "too many failures").
				Msg("stream removed")
		}
	}
}

// NumStreams return the streams with minimum version.
// Note: nodes with sync version smaller than minVersion is not counted.
func (p *Protocol) NumStreams() int {
	res := 0
	sts := p.sm.GetStreams()

	for _, st := range sts {
		ps, _ := st.ProtoSpec()
		if ps.Version.GreaterThanOrEqual(MinVersion) {
			res++
		}
	}
	return res
}

// GetStreamIDs returns the stream IDs of the streams with minimum version.
func (p *Protocol) GetStreamIDs() []sttypes.StreamID {
	ids := make([]sttypes.StreamID, 0, p.NumStreams())
	sts := p.sm.GetStreams()
	for _, st := range sts {
		ids = append(ids, st.ID())
	}
	return ids
}

// GetStreamManager get the underlying stream manager for upper level stream operations
func (p *Protocol) GetStreamManager() streammanager.StreamManager {
	return p.sm
}

// SubscribeAddStreamEvent subscribe the stream add event
func (p *Protocol) SubscribeAddStreamEvent(ch chan<- streammanager.EvtStreamAdded) event.Subscription {
	return p.sm.SubscribeAddStreamEvent(ch)
}

// ExitStartupMode exits startup mode early when enough peers are found
func (p *Protocol) ExitStartupMode() {
	if p.startupMode {
		p.startupMode = false
		p.logger.Info().Msg("Exiting startup mode early - sufficient peers found")
	}
}

// IsInStartupMode returns true if the protocol is in startup mode
func (p *Protocol) IsInStartupMode() bool {
	return p.startupMode
}
