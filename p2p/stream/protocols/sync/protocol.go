package sync

import (
	"context"
	"strconv"
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
	libp2p_host "github.com/libp2p/go-libp2p-core/host"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"
)

const (
	// serviceSpecifier is the specifier for the service.
	serviceSpecifier = "sync"
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

		config Config
		logger zerolog.Logger

		ctx    context.Context
		cancel func()
		closeC chan struct{}
	}

	// Config is the sync protocol config
	Config struct {
		Chain     engine.ChainReader
		Host      libp2p_host.Host
		Discovery discovery.Discovery
		ShardID   nodeconfig.ShardID
		Network   nodeconfig.NetworkType

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

// Specifier return the specifier for the protocol
func (p *Protocol) Specifier() string {
	return serviceSpecifier + "/" + strconv.Itoa(int(p.config.ShardID))
}

// ProtoID return the ProtoID of the sync protocol
func (p *Protocol) ProtoID() sttypes.ProtoID {
	return p.protoIDByVersion(MyVersion)
}

// Version returns the sync protocol version
func (p *Protocol) Version() *version.Version {
	return MyVersion
}

// Match checks the compatibility to the target protocol ID.
func (p *Protocol) Match(targetID string) bool {
	target, err := sttypes.ProtoIDToProtoSpec(sttypes.ProtoID(targetID))
	if err != nil {
		return false
	}
	if target.Service != serviceSpecifier {
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
		p.logger.Warn().Err(err).Str("stream ID", string(st.ID())).
			Msg("failed to add new stream")
		return
	}
	st.run()
}

func (p *Protocol) advertiseLoop() {
	for {
		sleep := p.advertise()
		select {
		case <-time.After(sleep):
		case <-p.closeC:
			return
		}
	}
}

// advertise will advertise all compatible protocol versions for helping nodes running low
// version
func (p *Protocol) advertise() time.Duration {
	var nextWait time.Duration

	pids := p.supportedProtoIDs()
	for _, pid := range pids {
		w, e := p.disc.Advertise(p.ctx, string(pid))
		if e != nil {
			p.logger.Warn().Err(e).Str("protocol", string(pid)).
				Msg("cannot advertise sync protocol")
			continue
		}
		if nextWait == 0 || nextWait > w {
			nextWait = w
		}
	}
	if nextWait < minAdvertiseInterval {
		nextWait = minAdvertiseInterval
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
		Service:     serviceSpecifier,
		NetworkType: p.config.Network,
		ShardID:     p.config.ShardID,
		Version:     v,
	}
	return spec.ToProtoID()
}

// RemoveStream removes the stream of the given stream ID
func (p *Protocol) RemoveStream(stID sttypes.StreamID) {
	if stID == "" {
		return
	}
	st, exist := p.sm.GetStreamByID(stID)
	if exist && st != nil {
		st.Close()
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
