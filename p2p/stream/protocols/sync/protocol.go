package sync

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/consensus/engine"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/discovery"
	"github.com/harmony-one/harmony/p2p/stream/common/ratelimiter"
	"github.com/harmony-one/harmony/p2p/stream/common/requestmanager"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/hashicorp/go-version"
	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
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

		config Config
		logger zerolog.Logger

		ctx    context.Context
		cancel func()
		closeC chan struct{}
	}

	// Config is the sync protocol config
	Config struct {
		Chain      engine.ChainReader
		Host       libp2p_host.Host
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
		chain:  config.Chain,
		disc:   config.Discovery,
		config: config,
		ctx:    ctx,
		cancel: cancel,
		closeC: make(chan struct{}),
	}
	smConfig := streammanager.Config{
		SoftLoCap: config.SmSoftLowCap,
		HardLoCap: config.SmHardLowCap,
		HiCap:     config.SmHiCap,
		DiscBatch: config.DiscBatch,
	}
	sp.sm = streammanager.NewStreamManager(sp.ProtoID(), config.Host, config.Discovery,
		sp.HandleStream, smConfig)

	sp.rl = ratelimiter.NewRateLimiter(sp.sm, rateLimiterGlobalRequestPerSecond, rateLimiterSingleRequestsPerSecond)

	sp.rm = requestmanager.NewRequestManager(sp.sm)

	// if it is not epoch chain, print the peer id and proto id
	if !config.EpochChain {
		fmt.Println("My peer id: ", config.Host.ID().String())
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
		if !errors.Is(err, streammanager.ErrStreamAlreadyExist) {
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
	minSleepTime := 30 * time.Second
	maxSleepTime := time.Duration(p.config.MaxAdvertiseWaitTime) * time.Minute

	for {
		sleep := p.advertise()

		// Adaptive sleep: Increase if new peers were found, decrease otherwise
		if p.recentPeerDiscoveryCount > 0 {
			sleep += time.Duration(p.recentPeerDiscoveryCount) * time.Second
		} else {
			sleep /= 2
		}

		// Enforce sleep boundaries
		if sleep < minSleepTime {
			sleep = minSleepTime
		} else if sleep > maxSleepTime {
			sleep = maxSleepTime
		}

		// Add jitter to prevent synchronized advertisements
		jitter := time.Duration(rand.Intn(30)) * time.Second
		sleep += jitter

		select {
		case <-p.closeC:
			return
		case <-time.After(sleep):
		}
	}
}

// advertise will advertise all compatible protocol versions for helping nodes running low version
func (p *Protocol) advertise() time.Duration {
	var nextWait time.Duration
	newPeersDiscovered := false
	maxRetries := 3

	// Constants for timeout adjustments
	baseTimeout := 300 * time.Second         // Initial timeout for the advertise call
	timeoutIncrementStep := 30 * time.Second // Increase timeout if context deadline is exceeded
	maxTimeout := 600 * time.Second          // Maximum allowed timeout
	backoffTimeRatio := 5 * time.Second      // Base time for exponential backoff
	maxBackoff := 30 * time.Second           // Cap for the exponential backoff delay

	timeout := baseTimeout

	// Adjust timeout if the last advertise call took longer
	if p.lastAdvertiseDuration > timeout {
		timeout = p.lastAdvertiseDuration + timeoutIncrementStep
	}

	for _, pid := range p.supportedProtoIDs() {
		retries := 0
		var err error
		var w time.Duration

		for retries < maxRetries {
			ctx, cancel := context.WithTimeout(p.ctx, timeout)
			start := time.Now()
			w, err = p.disc.Advertise(ctx, string(pid))
			elapsed := time.Since(start)
			cancel()

			// Store the last advertisement duration
			p.lastAdvertiseDuration = elapsed

			if err == nil {
				newPeersDiscovered = true
				p.logger.Debug().
					Str("protocol", string(pid)).
					Dur("elapsed(sec)", time.Duration(elapsed.Seconds())).
					Int("retry", retries).
					Msg("Advertise call completed")
				break
			}

			p.logger.Debug().Err(err).
				Str("protocol", string(pid)).
				Int("retry", retries).
				Dur("elapsed(sec)", time.Duration(elapsed.Seconds())).
				Msg("Advertise failed, retrying")

			// If the error is a timeout, increase the timeout duration
			if errors.Is(err, context.DeadlineExceeded) {
				p.logger.Debug().
					Str("protocol", string(pid)).
					Dur("elapsed(sec)", time.Duration(elapsed.Seconds())).
					Msg("Advertise failed due to timeout, increasing timeout")

				timeout += timeoutIncrementStep
				if timeout > maxTimeout {
					timeout = maxTimeout
				}
			}

			retries++

			// Dynamic backoff to avoid excessive retries
			backoff := time.Duration(retries) * backoffTimeRatio
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			time.Sleep(backoff)
		}

		if err != nil {
			p.logger.Debug().Err(err).
				Str("protocol", string(pid)).
				Msg("Advertise failed after retries")
			continue
		}

		// Set the next wait time based on the response
		if nextWait == 0 || nextWait > w {
			nextWait = w
		}
	}

	// Ensure a minimum advertise interval
	if nextWait < minAdvertiseInterval {
		nextWait = minAdvertiseInterval
	}

	// Adjust next wait time based on success/failure
	if newPeersDiscovered {
		nextWait += 10 * time.Second
	} else {
		nextWait /= 2
		if nextWait < minAdvertiseInterval {
			nextWait = minAdvertiseInterval
		}
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
		st.Close(reason)
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
			st.Close("too many failures")
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

// GetStreamManager get the underlying stream manager for upper level stream operations
func (p *Protocol) GetStreamManager() streammanager.StreamManager {
	return p.sm
}

// SubscribeAddStreamEvent subscribe the stream add event
func (p *Protocol) SubscribeAddStreamEvent(ch chan<- streammanager.EvtStreamAdded) event.Subscription {
	return p.sm.SubscribeAddStreamEvent(ch)
}
