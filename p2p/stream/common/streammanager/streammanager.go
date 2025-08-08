package streammanager

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/libp2p/go-libp2p/core/network"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var (
	// ErrStreamAlreadyRemoved is the error that a stream has already been removed
	ErrStreamAlreadyRemoved = errors.New("stream already removed")
	// ErrStreamAlreadyExist is the error that a stream has already exist
	ErrStreamAlreadyExist = errors.New("stream already exist")
	// ErrTooManyStreams is the error that the number of streams is exceeded the capacity
	ErrTooManyStreams = errors.New("too many streams")
	// ErrStreamRemovalNotExpired is the error that the stream was removed before and can't be added yet
	ErrStreamRemovalNotExpired = errors.New("stream removal not expired yet")
)

// streamManager is the implementation of StreamManager. It manages streams on
// one single protocol. It does the following job:
// 1. add a new stream.
// 2. closes a stream.
// 3. discover and connect new streams when the number of streams is below threshold.
// 4. emit stream events to inform other modules.
// 5. reset all streams on close.
type streamManager struct {
	// streamManager only manages streams on one protocol.
	myProtoID   sttypes.ProtoID
	myProtoSpec sttypes.ProtoSpec
	config      Config
	// streams is the map of peer ID to stream
	// Note that it could happen that remote node does not share exactly the same
	// protocol ID (e.g. different version)
	streams *streamSet
	// tracks removed streams with cooldown
	removedStreams *sttypes.SafeMap[sttypes.StreamID, *RemovalInfo]
	// reserved streams
	reservedStreams *streamSet
	isTrustedPeer   func(libp2p_peer.ID) bool
	getTrustedPeers func() []libp2p_peer.ID
	// libp2p utilities
	host         host
	pf           peerFinder
	handleStream func(stream network.Stream, trusted bool)
	// incoming task channels
	addStreamCh chan addStreamTask
	rmStreamCh  chan rmStreamTask
	stopCh      chan stopTask
	discCh      chan discTask
	curTask     interface{}
	coolDown    *abool.AtomicBool
	// utils
	coolDownCache    *coolDownCache
	addStreamFeed    event.Feed
	removeStreamFeed event.Feed
	logger           zerolog.Logger
	ctx              context.Context
	cancel           func()

	// limit concurrent setup of streams
	setupSem chan struct{}
	// function to check if trusted peers initialization is complete
	trustedPeersInitiated func() bool
	// trustedPeersProcessed tracks if trusted peers have been processed during bootstrap.
	// This ensures trusted peers are only processed once during the initial bootstrap discovery.
	trustedPeersProcessed *abool.AtomicBool
	// trustedStreams tracks stream IDs of successfully established trusted peer streams
	// and their location (main or reserved list) for efficient counting
	// Value: true = main list, false = reserved list
	trustedStreams *sttypes.SafeMap[sttypes.StreamID, bool]
	// Atomic counters for trusted streams - optimized for O(1) counting
	numTrustedStreamsMain     int64 // Count of trusted streams in main list
	numTrustedStreamsReserved int64 // Count of trusted streams in reserved list

	// callback for when enough streams are found
	enoughStreamsCallback func()
}

type RemovalInfo struct {
	count     uint64
	removedAt time.Time
	expireAt  time.Time
	mu        sync.RWMutex
}

// MarkAsRemoved resets the removal time and increments the removal count.
func (rm *RemovalInfo) MarkAsRemoved(criticalErr bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	if rm.count > 0 && now.Sub(rm.removedAt) > MaxRemovalCooldownDuration {
		rm.count = 0
	}
	if criticalErr {
		rm.expireAt = now.Add(MaxRemovalCooldownDuration)
	} else {
		// First failure (count=0) gets no cooldown to avoid penalizing one-off disconnects.
		cooldownDur := RemovalCooldownDuration * time.Duration(rm.count)
		if cooldownDur > MaxRemovalCooldownDuration {
			cooldownDur = MaxRemovalCooldownDuration
		}
		rm.expireAt = now.Add(cooldownDur)
	}
	rm.removedAt = now
	rm.count++
}

// RemovedAt returns the timestamp when the stream was removed.
func (rm *RemovalInfo) RemovedAt() time.Time {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.removedAt
}

// HasExpired checks if the cooldown period has passed, allowing the stream to reconnect.
func (rm *RemovalInfo) HasExpired() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return time.Now().After(rm.expireAt)
}

// BumpCount increases the removal count.
func (rm *RemovalInfo) BumpCount() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.count++
}

// ResetCount resets the removal count.
func (rm *RemovalInfo) ResetCount() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.count = 0
}

// SetEnoughStreamsCallback sets the callback function to be called when enough streams are found
func (sm *streamManager) SetEnoughStreamsCallback(callback func()) {
	sm.enoughStreamsCallback = callback
}

// NewStreamManager creates a new stream manager for the given proto ID
func NewStreamManager(pid sttypes.ProtoID, host host, pf peerFinder, handleStream func(network.Stream, bool), c Config) StreamManager {
	return newStreamManager(pid, host, pf, handleStream, c)
}

// newStreamManager creates a new stream manager
func newStreamManager(pid sttypes.ProtoID, host host, pf peerFinder, handleStream func(network.Stream, bool), c Config) *streamManager {
	ctx, cancel := context.WithCancel(context.Background())

	logger := utils.Logger().With().Str("module", "stream manager").
		Str("protocol ID", string(pid)).Logger()

	protoSpec, _ := sttypes.ProtoIDToProtoSpec(pid)

	if c.IsTrustedPeer != nil {
		logger.Info().
			Msg("[StreamManager] initialized with trusted peer checking enabled")
	} else {
		logger.Info().
			Msg("[StreamManager] initialized with no trusted peer checking (or nil function)")
	}

	sm := &streamManager{
		myProtoID:             pid,
		myProtoSpec:           protoSpec,
		config:                c,
		streams:               newStreamSet(),
		reservedStreams:       newStreamSet(),
		removedStreams:        sttypes.NewSafeMap[sttypes.StreamID, *RemovalInfo](),
		isTrustedPeer:         c.IsTrustedPeer,
		getTrustedPeers:       c.GetTrustedPeers,
		host:                  host,
		pf:                    pf,
		handleStream:          handleStream,
		addStreamCh:           make(chan addStreamTask),
		rmStreamCh:            make(chan rmStreamTask),
		stopCh:                make(chan stopTask),
		discCh:                make(chan discTask, 1), // discCh is a buffered channel to avoid overuse of goroutine
		coolDown:              abool.New(),
		coolDownCache:         newCoolDownCache(),
		logger:                logger,
		ctx:                   ctx,
		cancel:                cancel,
		setupSem:              make(chan struct{}, setupConcurrency),
		trustedPeersInitiated: c.TrustedPeersInitiated,
		trustedPeersProcessed: abool.New(),
		trustedStreams:        sttypes.NewSafeMap[sttypes.StreamID, bool](),
	}

	// Initialize all stream metrics with this protocol ID

	// Initialize gauge metrics
	numStreamsGaugeVec.With(prometheus.Labels{"topic": string(pid)}).Set(0)
	numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(pid)}).Set(0)
	numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(pid)}).Set(0)
	numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(pid)}).Set(0)

	// Initialize counter metrics
	discoverCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	discoveredPeersCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	addedStreamsCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	removedStreamsCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	streamCriticalErrorCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	trustedPeerStreamsConnectFailuresCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	trustedPeerStreamsSetupAttemptsCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)
	trustedPeerStreamsAddedCounterVec.With(prometheus.Labels{"topic": string(pid)}).Add(0)

	// Note: streamRemovalReasonCounterVec and setupStreamDuration don't need initialization
	// - streamRemovalReasonCounterVec has dynamic labels (reason, critical) that can't be pre-initialized
	// - setupStreamDuration is a histogram that doesn't need initialization

	return sm
}

// Start starts the stream manager
func (sm *streamManager) Start() {
	go sm.loop()
}

// Close close the stream manager
func (sm *streamManager) Close() {
	task := stopTask{done: make(chan struct{})}
	sm.stopCh <- task

	<-task.done
}

func (sm *streamManager) loop() {
	var (
		discTicker = time.NewTicker(checkInterval)
		discCtx    context.Context
		discCancel func()
	)
	defer discTicker.Stop()

	// Wait for trusted peers to be initialized before bootstrap discovery.
	// This ensures trusted peers are available for the first discovery cycle.
	if sm.trustedPeersInitiated != nil {
		sm.waitForTrustedPeersInitialization()
	}

	// Initialize gauge metrics with current values (metrics already initialized to 0 in constructor)
	numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
	numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))
	numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
	numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))

	// bootstrap discovery
	sm.discCh <- discTask{}

	for {
		select {
		case <-discTicker.C:
			// Periodically refresh gauge metrics
			numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
			numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))
			numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
			numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))

			if !sm.softHaveEnoughStreams() {
				sm.discCh <- discTask{}
			}

		case <-sm.discCh:
			if sm.coolDown.IsSet() {
				sm.logger.Info().Msg("skipping discover for cool down")
				continue
			}
			if discCancel != nil {
				discCancel() // cancel last discovery
			}
			discCtx, discCancel = context.WithCancel(sm.ctx)
			go func(ctx context.Context) {
				discovered, err := sm.discoverAndSetupStream(ctx)
				if err != nil {
					sm.logger.Err(err)
				}
				// Update stream metrics after discovery completes
				numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
				numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))
				numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
				numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))
				if discovered == 0 {
					// start discover cool down
					sm.coolDown.Set()
					go func() {
						time.Sleep(coolDownPeriod)
						sm.coolDown.UnSet()
					}()
				}
			}(discCtx)

		case addStream := <-sm.addStreamCh:
			err := sm.handleAddStream(addStream.st)
			addStream.errC <- err

		case rmStream := <-sm.rmStreamCh:
			err := sm.handleRemoveStream(rmStream.id, rmStream.reason, rmStream.criticalErr)
			rmStream.errC <- err

		case stop := <-sm.stopCh:
			sm.coolDown.Set() // to immediately block discoveries
			if discCancel != nil {
				discCancel()
			}
			sm.cancel()
			sm.removeAllStreamOnClose()
			stop.done <- struct{}{}
			return
		}
	}
}

// NewStream registers a stream with the stream manager.
// On error the caller is responsible for closing the raw stream.
func (sm *streamManager) NewStream(stream sttypes.Stream) error {
	if err := sm.sanityCheckStream(stream); err != nil {
		return errors.Wrap(err, "stream sanity check failed")
	}
	task := addStreamTask{
		st:   stream,
		errC: make(chan error, 1),
	}
	select {
	case sm.addStreamCh <- task:
		select {
		case err := <-task.errC:
			return err
		case <-sm.ctx.Done():
			return sm.ctx.Err()
		}
	case <-sm.ctx.Done():
		return sm.ctx.Err()
	}
}

// RemoveStream removes a stream from the stream manager.
// Returns ctx.Err() if the stream manager has been shut down.
func (sm *streamManager) RemoveStream(stID sttypes.StreamID, reason string, criticalErr bool) error {
	task := rmStreamTask{
		id:          stID,
		reason:      reason,
		criticalErr: criticalErr,
		errC:        make(chan error, 1),
	}
	select {
	case sm.rmStreamCh <- task:
		select {
		case err := <-task.errC:
			return err
		case <-sm.ctx.Done():
			return sm.ctx.Err()
		}
	case <-sm.ctx.Done():
		return sm.ctx.Err()
	}
}

// GetStreams return the streams.
func (sm *streamManager) GetStreams() []sttypes.Stream {
	return sm.streams.getStreams()
}

// GetStreamByID return the stream with the given id.
func (sm *streamManager) GetStreamByID(id sttypes.StreamID) (sttypes.Stream, bool) {
	return sm.streams.get(id)
}

// GetReservedStreams return the reserved streams.
func (sm *streamManager) GetReservedStreams() []sttypes.Stream {
	return sm.reservedStreams.getStreams()
}

// NumReservedStreams return the number of reserved streams.
func (sm *streamManager) NumReservedStreams() int {
	return sm.reservedStreams.size()
}

type (
	addStreamTask struct {
		st   sttypes.Stream
		errC chan error
	}

	rmStreamTask struct {
		id          sttypes.StreamID
		reason      string
		criticalErr bool
		errC        chan error
	}

	discTask struct{}

	stopTask struct {
		done chan struct{}
	}
)

// sanity checks the service, network and shard ID
func (sm *streamManager) sanityCheckStream(st sttypes.Stream) error {
	mySpec := sm.myProtoSpec
	rmSpec, err := st.ProtoSpec()
	if err != nil {
		return err
	}
	if sttypes.StreamID(sm.host.ID()) == st.ID() {
		return fmt.Errorf("can't connect to itself")
	}
	if mySpec.Service != rmSpec.Service {
		return fmt.Errorf("unexpected service: %v/%v", rmSpec.Service, mySpec.Service)
	}
	if mySpec.NetworkType != rmSpec.NetworkType {
		return fmt.Errorf("unexpected network: %v/%v", rmSpec.NetworkType, mySpec.NetworkType)
	}
	if mySpec.ShardID != rmSpec.ShardID {
		return fmt.Errorf("unexpected shard ID: %v/%v", rmSpec.ShardID, mySpec.ShardID)
	}
	return nil
}

func (sm *streamManager) handleAddStream(st sttypes.Stream) error {
	id := st.ID()
	// check if stream exists
	if _, ok := sm.streams.get(id); ok {
		return ErrStreamAlreadyExist
	}
	if _, ok := sm.reservedStreams.get(id); ok {
		return ErrStreamAlreadyExist
	}
	// Check if stream was recently removed
	if removalInfo, exists := sm.removedStreams.Get(id); exists {
		if !removalInfo.HasExpired() {
			return ErrStreamRemovalNotExpired
		}
	}

	// Check if this is a trusted peer stream for metrics tracking
	isTrustedPeer := st.IsTrusted()

	// If the stream list has sufficient capacity, the stream can be added to the reserved list
	if sm.streams.size() >= sm.config.HiCap {
		if sm.reservedStreams.size() < MaxReservedStreams {
			if _, ok := sm.reservedStreams.get(id); !ok {
				sm.reservedStreams.addStream(st)
				sm.logger.Info().
					Str("protocolID", string(sm.myProtoID)).
					Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
					Int("NumStreams", sm.streams.size()).
					Int("NumReservedStreams", sm.reservedStreams.size()).
					Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
					Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
					Interface("StreamID", id).
					Bool("trusted", isTrustedPeer).
					Msg("[StreamManager] added new stream to reserved list")
				numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))
				// Update trusted peer metrics if this is a trusted peer in reserved list
				if isTrustedPeer {
					// Mark this stream as a successful trusted stream in reserved list
					sm.trustedStreams.Set(id, false) // false = reserved list
					atomic.AddInt64(&sm.numTrustedStreamsReserved, 1)
					trustedPeerStreamsAddedCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
					numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))
				}
			}
			return nil
		}
		return ErrTooManyStreams
	}

	sm.streams.addStream(st)
	sm.logger.Info().
		Str("protocolID", string(sm.myProtoID)).
		Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
		Int("NumStreams", sm.streams.size()).
		Int("NumReservedStreams", sm.reservedStreams.size()).
		Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
		Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
		Interface("StreamID", id).
		Bool("trusted", isTrustedPeer).
		Msg("[StreamManager] added new stream to main streams list")

	sm.addStreamFeed.Send(EvtStreamAdded{st})
	addedStreamsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
	numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))

	// Update trusted peer metrics if this is a trusted peer
	if isTrustedPeer {
		// Mark this stream as a successful trusted stream in main list
		sm.trustedStreams.Set(id, true) // true = main list
		atomic.AddInt64(&sm.numTrustedStreamsMain, 1)
		trustedPeerStreamsAddedCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
		numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
	}
	// Call callback if enough streams are found
	if sm.enoughStreamsCallback != nil && sm.softHaveEnoughStreams() {
		sm.enoughStreamsCallback()
	}

	return nil
}

func (sm *streamManager) addStreamFromReserved(count int) (int, error) {
	if sm.reservedStreams.size() == 0 {
		return 0, errors.New("reserved streams list is empty")
	}
	added := 0
	for added < count && sm.reservedStreams.size() > 0 {
		st, err := sm.reservedStreams.popStream()
		if err != nil {
			return added, err
		}
		sm.streams.addStream(st)
		isTrusted := st.IsTrusted()
		sm.logger.Info().
			Str("protocolID", string(sm.myProtoID)).
			Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
			Int("NumStreams", sm.streams.size()).
			Int("NumReservedStreams", sm.reservedStreams.size()).
			Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
			Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
			Interface("StreamID", st.ID()).
			Bool("trusted", isTrusted).
			Msg("[StreamManager] added new stream from reserved streams list")
		sm.addStreamFeed.Send(EvtStreamAdded{st})
		addedStreamsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
		numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
		numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))

		// Update trusted peer metrics if this is a trusted peer
		// Note: counter is NOT incremented here because the stream was already counted when first added to reserved list
		// Only update counters and gauges since the stream is moving from reserved to main list
		if inMain, exists := sm.trustedStreams.Get(st.ID()); exists && !inMain {
			// Stream was in reserved, now moving to main
			sm.trustedStreams.Set(st.ID(), true) // true = main list
			atomic.AddInt64(&sm.numTrustedStreamsReserved, -1)
			atomic.AddInt64(&sm.numTrustedStreamsMain, 1)
			numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
			numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))
		}
		added++
	}
	return added, nil
}

func (sm *streamManager) handleRemoveStream(id sttypes.StreamID, reason string, criticalErr bool) error {
	// Check which set contains the stream - only delete from the one that has it
	st, inMain := sm.streams.get(id)
	if !inMain {
		// Try reserved streams
		st, inReserved := sm.reservedStreams.get(id)
		if !inReserved {
			return ErrStreamAlreadyRemoved
		}
		// Stream is in reserved list
		if criticalErr {
			streamCriticalErrorCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
		}

		// Check if this is a trusted stream and handle accordingly
		_, isTrusted := sm.trustedStreams.Get(id)
		if isTrusted {
			if criticalErr {
				// Trusted streams with critical errors are not removed (protected)
				sm.logger.Info().
					Str("protocolID", string(sm.myProtoID)).
					Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
					Int("NumStreams", sm.streams.size()).
					Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
					Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
					Interface("StreamID", id).
					Bool("trusted", true).
					Str("reason", reason).
					Bool("criticalErr", criticalErr).
					Msg("[StreamManager] trusted peer got critical error but not removed")
				return nil
			}
			// Trusted stream with non-critical error: remove from trustedStreams map and update counters
			sm.trustedStreams.Delete(id)
			atomic.AddInt64(&sm.numTrustedStreamsReserved, -1)
		}

		sm.reservedStreams.deleteStream(st)
		sm.logger.Info().
			Str("protocolID", string(sm.myProtoID)).
			Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
			Int("NumStreams", sm.streams.size()).
			Int("NumReservedStreams", sm.reservedStreams.size()).
			Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
			Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
			Interface("StreamID", id).
			Str("reason", reason).
			Bool("criticalErr", criticalErr).
			Bool("trusted", isTrusted).
			Msg("[StreamManager] removed stream from reserved streams list")

		info, exist := sm.removedStreams.Get(id)
		if !exist {
			info = &RemovalInfo{count: 0}
			sm.removedStreams.Set(id, info)
		}
		info.MarkAsRemoved(criticalErr)

		sm.removeStreamFeed.Send(EvtStreamRemoved{id})
		removedStreamsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
		streamRemovalReasonCounterVec.With(prometheus.Labels{"reason": reason, "critical": strconv.FormatBool(criticalErr)}).Inc()
		numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
		numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))
		numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
		numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))

		sm.tryToReplaceRemovedStream()
		return nil
	}

	// Stream is in main list
	if criticalErr {
		streamCriticalErrorCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
	}

	// Check if this is a trusted stream and handle accordingly
	_, isTrusted := sm.trustedStreams.Get(id)
	if isTrusted {
		if criticalErr {
			// Trusted streams with critical errors are not removed (protected)
			sm.logger.Info().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Int("NumStreams", sm.streams.size()).
				Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
				Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
				Interface("StreamID", id).
				Bool("trusted", true).
				Str("reason", reason).
				Bool("criticalErr", criticalErr).
				Msg("[StreamManager] trusted peer got critical error but not removed")
			return nil
		}
		// Trusted stream with non-critical error: remove from trustedStreams map and update counters
		sm.trustedStreams.Delete(id)
		atomic.AddInt64(&sm.numTrustedStreamsMain, -1)
	}

	sm.streams.deleteStream(st)

	sm.logger.Info().
		Str("protocolID", string(sm.myProtoID)).
		Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
		Int("NumStreams", sm.streams.size()).
		Int("NumReservedStreams", sm.reservedStreams.size()).
		Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
		Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
		Interface("StreamID", id).
		Str("reason", reason).
		Bool("criticalErr", criticalErr).
		Bool("trusted", isTrusted).
		Msg("[StreamManager] removed stream from main streams list")

	info, exist := sm.removedStreams.Get(id)
	if !exist {
		info = &RemovalInfo{count: 0}
		sm.removedStreams.Set(id, info)
	}
	info.MarkAsRemoved(criticalErr)

	// try to replace removed streams from reserved list
	sm.removeStreamFeed.Send(EvtStreamRemoved{id})

	removedStreamsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
	streamRemovalReasonCounterVec.With(prometheus.Labels{"reason": reason, "critical": strconv.FormatBool(criticalErr)}).Inc()
	numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
	numReservedStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.reservedStreams.size()))
	numTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsMain)))
	numReservedTrustedPeerStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(atomic.LoadInt64(&sm.numTrustedStreamsReserved)))

	sm.tryToReplaceRemovedStream()

	return nil
}

func (sm *streamManager) tryToReplaceRemovedStream() error {
	// try to replace removed streams from the reserved list.
	requiredStreams := sm.hardRequiredStreams()

	if added, err := sm.addStreamFromReserved(requiredStreams); added > 0 {
		sm.logger.Info().
			Err(err). // in case if some new streams added and others failed
			Int("requiredStreams", requiredStreams).
			Int("added", added).
			Msg("added new streams from reserved list")
	}

	// if stream number is smaller than HardLoCap, spin up the discover
	if !sm.hardHaveEnoughStream() {
		select {
		case sm.discCh <- discTask{}:
		default:
		}
	}

	return nil
}

func (sm *streamManager) removeAllStreamOnClose() {
	var wg sync.WaitGroup

	for _, st := range sm.streams.slice() {
		wg.Add(1)
		go func(st sttypes.Stream) {
			defer wg.Done()
			err := st.CloseOnExit()
			if err != nil {
				sm.logger.Warn().Err(err).
					Interface("stream ID", st.ID()).
					Msg("failed to close stream")
			}
		}(st)
	}
	wg.Wait()

	// Be nice. after close, the field is still accessible to prevent potential panics
	sm.streams.Erase()
}

// waitForTrustedPeersInitialization waits for trusted peers to be initialized before starting discovery.
// It polls the host's TrustedPeersInitiated() function until it returns true, ensuring trusted peers
// are available for the first bootstrap discovery. The stream manager waits indefinitely (until context
// cancellation) for the host to complete initialization, as the host manages its own timeout.
func (sm *streamManager) waitForTrustedPeersInitialization() {
	if sm.trustedPeersInitiated == nil {
		return // No function provided, nothing to wait for
	}

	sm.logger.Info().
		Str("protocolID", string(sm.myProtoID)).
		Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
		Msg("[StreamManager] waiting for trusted peers initialization before starting discovery")

	// Check immediately first (host may have already completed initialization)
	if sm.trustedPeersInitiated() {
		sm.logger.Info().
			Str("protocolID", string(sm.myProtoID)).
			Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
			Msg("[StreamManager] trusted peers already initialized, proceeding with discovery")
		return
	}

	// Poll for trusted peers initialization to complete
	// The host manages its own timeout, so we wait indefinitely until host signals completion
	ticker := time.NewTicker(trustedPeersCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if sm.trustedPeersInitiated() {
				sm.logger.Info().
					Str("protocolID", string(sm.myProtoID)).
					Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
					Msg("[StreamManager] trusted peers initialization complete, proceeding with discovery")
				return
			}
		case <-sm.ctx.Done():
			sm.logger.Warn().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Msg("[StreamManager] context cancelled while waiting for trusted peers")
			return
		}
	}
}

// isStreamFromTrustedPeer checks if a stream is from a trusted peer
// It checks trustedStreams map first before falling back to function call
func (sm *streamManager) isStreamFromTrustedPeer(streamID sttypes.StreamID) bool {
	// Fast path: check if we already know this is a trusted stream
	if _, exists := sm.trustedStreams.Get(streamID); exists {
		return true
	}
	// Fallback: check using the function (for streams not yet added to trustedStreams map)
	if sm.isTrustedPeer == nil {
		return false
	}
	return sm.isTrustedPeer(libp2p_peer.ID(streamID))
}

// countTrustedPeerStreams returns the number of currently connected trusted peer streams in the main list
// Optimized: Uses atomic counter for O(1) performance instead of O(n) iteration
func (sm *streamManager) countTrustedPeerStreams() int {
	return int(atomic.LoadInt64(&sm.numTrustedStreamsMain))
}

// countTrustedPeerStreamsInReserved returns the number of currently connected trusted peer streams in the reserved list
// Optimized: Uses atomic counter for O(1) performance instead of O(n) iteration
func (sm *streamManager) countTrustedPeerStreamsInReserved() int {
	return int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))
}

// setupTrustedStreams attempts to establish exactly TrustedMinPeers trusted peer streams.
// It starts goroutines in batches, waits for them to complete, and retries if needed.
func (sm *streamManager) setupTrustedStreams(trustedPeers []libp2p_peer.ID, trustedMinPeers int) int {
	if trustedMinPeers <= 0 {
		return 0
	}

	// Pre-build a set of existing stream IDs for fast O(1) lookups
	// This avoids multiple map lookups per peer in the filtering loop
	existingStreamIDs := make(map[sttypes.StreamID]bool, sm.streams.size()+sm.reservedStreams.size())
	for _, st := range sm.streams.getStreams() {
		existingStreamIDs[st.ID()] = true
	}
	for _, st := range sm.reservedStreams.getStreams() {
		existingStreamIDs[st.ID()] = true
	}

	hostID := sm.host.ID()

	// Filter out peers that shouldn't be processed
	availablePeers := make([]libp2p_peer.ID, 0, len(trustedPeers))
	for _, pid := range trustedPeers {
		if pid == hostID {
			sm.logger.Debug().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Interface("peerID", pid).
				Msg("[setupTrustedStreams] skipping trusted peer (self)")
			continue
		}
		if sm.coolDownCache.Has(pid) {
			sm.logger.Debug().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Interface("peerID", pid).
				Msg("[setupTrustedStreams] skipping trusted peer (in cooldown)")
			continue
		}
		newStreamID := sttypes.StreamID(pid)
		if existingStreamIDs[newStreamID] {
			sm.logger.Debug().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Interface("peerID", pid).
				Msg("[setupTrustedStreams] skipping trusted peer (stream already exists)")
			continue
		}
		// Check if stream was recently removed
		if removalInfo, exists := sm.removedStreams.Get(newStreamID); exists {
			if !removalInfo.HasExpired() {
				sm.logger.Debug().
					Str("protocolID", string(sm.myProtoID)).
					Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
					Interface("peerID", pid).
					Time("removedAt", removalInfo.RemovedAt()).
					Msg("[setupTrustedStreams] skipping trusted peer (removal cooldown not expired)")
				continue
			}
		}
		availablePeers = append(availablePeers, pid)
	}

	if len(availablePeers) == 0 {
		sm.logger.Info().
			Str("protocolID", string(sm.myProtoID)).
			Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
			Int("totalTrustedPeers", len(trustedPeers)).
			Int("existingStreams", sm.streams.size()).
			Int("existingReservedStreams", sm.reservedStreams.size()).
			Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
			Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
			Msg("[setupTrustedStreams] no available trusted peers to connect")
		return 0
	}

	sm.logger.Info().
		Str("protocolID", string(sm.myProtoID)).
		Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
		Int("trustedMinPeers", trustedMinPeers).
		Int("totalTrustedPeers", len(trustedPeers)).
		Int("availablePeers", len(availablePeers)).
		Int("existingStreams", sm.streams.size()).
		Int("existingReservedStreams", sm.reservedStreams.size()).
		Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
		Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
		Msg("[setupTrustedStreams] starting trusted peer stream setup")

	peerIndex := 0
	var successCount int64 // Use int64 for atomic operations
	var attemptCount int64

	// Keep trying until we reach TrustedMinPeers or run out of available peers
	for int(atomic.LoadInt64(&successCount)) < trustedMinPeers && peerIndex < len(availablePeers) {
		// Calculate how many more we need
		currentSuccess := int(atomic.LoadInt64(&successCount))
		needed := trustedMinPeers - currentSuccess
		// Don't start more goroutines than we have peers left
		batchSize := needed
		if peerIndex+batchSize > len(availablePeers) {
			batchSize = len(availablePeers) - peerIndex
		}

		var wg sync.WaitGroup

		sm.logger.Info().
			Str("protocolID", string(sm.myProtoID)).
			Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
			Int("batchSize", batchSize).
			Int("currentSuccess", currentSuccess).
			Int("needed", needed).
			Int("trustedMinPeers", trustedMinPeers).
			Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
			Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
			Msg("[setupTrustedStreams] starting batch of trusted peer connections")

		for i := 0; i < batchSize && peerIndex < len(availablePeers); i++ {
			pid := availablePeers[peerIndex]
			peerIndex++
			atomic.AddInt64(&attemptCount, 1)

			wg.Add(1)
			sm.setupSem <- struct{}{}
			go func(pid libp2p_peer.ID) {
				defer func() {
					<-sm.setupSem
					wg.Done()
				}()

				discoveredPeersCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
				trustedPeerStreamsSetupAttemptsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()

				sm.logger.Info().
					Str("protocolID", string(sm.myProtoID)).
					Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
					Interface("peerID", pid).
					Msg("[setupTrustedStreams] attempting to setup stream with trusted peer")

				err := sm.setupStreamWithPeer(sm.ctx, pid, true)
				if err != nil {
					// Handle error directly in goroutine
					sm.coolDownCache.Add(pid)
					trustedPeerStreamsConnectFailuresCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
					sm.logger.Warn().Err(err).
						Str("protocolID", string(sm.myProtoID)).
						Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
						Interface("peerID", pid).
						Int64("attemptCount", atomic.LoadInt64(&attemptCount)).
						Int64("successCount", atomic.LoadInt64(&successCount)).
						Msg("[setupTrustedStreams] failed to setup stream with trusted peer")
				} else {
					// Stream setup was initiated successfully.
					newSuccessCount := atomic.AddInt64(&successCount, 1)
					sm.logger.Info().
						Str("protocolID", string(sm.myProtoID)).
						Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
						Interface("peerID", pid).
						Int64("successCount", newSuccessCount).
						Int64("attemptCount", atomic.LoadInt64(&attemptCount)).
						Int("trustedMinPeers", trustedMinPeers).
						Msg("[setupTrustedStreams] successfully initiated trusted peer stream setup")
				}
			}(pid)
		}

		// Wait for all goroutines in this batch to complete
		wg.Wait()
	}

	sm.logger.Info().
		Str("protocolID", string(sm.myProtoID)).
		Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
		Int64("successCount", atomic.LoadInt64(&successCount)).
		Int64("attemptCount", atomic.LoadInt64(&attemptCount)).
		Int("trustedMinPeers", trustedMinPeers).
		Int("totalTrustedPeers", len(trustedPeers)).
		Int("availablePeers", len(availablePeers)).
		Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
		Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
		Msg("[setupTrustedStreams] completed trusted peer stream setup")

	return int(atomic.LoadInt64(&successCount))
}

func (sm *streamManager) discoverAndSetupStream(discCtx context.Context) (int, error) {
	connectedTrustedStreams := 0

	// Process trusted peers only once during bootstrap discovery.
	// After bootstrap, trusted peers will be handled through normal discovery if they disconnect.
	// This prevents aggressive retries of failed trusted peer connections on every discovery cycle.
	if !sm.trustedPeersProcessed.IsSet() && sm.getTrustedPeers != nil {
		trustedPeers := sm.getTrustedPeers()
		trustedPeerCount := len(trustedPeers)
		if trustedPeerCount > 0 {
			sm.trustedPeersProcessed.Set()

			trustedMinPeers := sm.config.TrustedMinPeers
			if trustedMinPeers <= 0 {
				trustedMinPeers = trustedPeerCount // If 0 or negative, process all
			}

			sm.logger.Info().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Int("trustedPeerCount", trustedPeerCount).
				Int("trustedMinPeers", trustedMinPeers).
				Int("existingStreams", sm.streams.size()).
				Int("existingReservedStreams", sm.reservedStreams.size()).
				Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
				Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
				Msg("[discoverAndSetupStream] processing trusted peers for bootstrap stream setup")

			// Setup trusted streams - this function handles batching and waiting
			successCount := sm.setupTrustedStreams(trustedPeers, trustedMinPeers)
			connectedTrustedStreams = successCount

			sm.logger.Info().
				Str("protocolID", string(sm.myProtoID)).
				Uint32("shardID", uint32(sm.myProtoSpec.ShardID)).
				Int("successCount", successCount).
				Int("trustedMinPeers", trustedMinPeers).
				Int("NumTrustedStreamsMain", int(atomic.LoadInt64(&sm.numTrustedStreamsMain))).
				Int("NumTrustedStreamsReserved", int(atomic.LoadInt64(&sm.numTrustedStreamsReserved))).
				Msg("[discoverAndSetupStream] completed trusted peer stream setup, proceeding to discover other peers")
		}
	}

	if sm.streams.size()+connectedTrustedStreams >= sm.config.HardLoCap {
		return connectedTrustedStreams, nil
	}

	peers, err := sm.discover(discCtx)
	if err != nil {
		return connectedTrustedStreams, errors.Wrap(err, "failed to discover")
	}
	discoverCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()

	connecting := 0
	for peer := range peers {
		if peer.ID == sm.host.ID() {
			continue
		}
		if sm.coolDownCache.Has(peer.ID) {
			continue
		}
		if _, ok := sm.streams.get(sttypes.StreamID(peer.ID)); ok {
			continue
		}
		if _, ok := sm.reservedStreams.get(sttypes.StreamID(peer.ID)); ok {
			continue
		}
		discoveredPeersCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
		connecting += 1
		go func(pid libp2p_peer.ID) {
			err := sm.setupStreamWithPeer(sm.ctx, pid, false)
			if err != nil {
				sm.coolDownCache.Add(pid)
				sm.logger.Warn().Err(err).
					Interface("peerID", pid).
					Msg("failed to setup stream with peer")
				return
			}
			sm.logger.Info().
				Interface("peerID", pid).
				Msg("new stream set up with peer")
		}(peer.ID)
	}

	return connectedTrustedStreams + connecting, nil
}

func (sm *streamManager) discover(ctx context.Context) (<-chan libp2p_peer.AddrInfo, error) {
	numStreams := sm.streams.size()

	discBatch := sm.config.DiscBatch
	if sm.config.HiCap-numStreams < sm.config.DiscBatch {
		discBatch = sm.config.HiCap - numStreams
	}
	sm.logger.Debug().
		Interface("protoID", sm.myProtoID).
		Int("numStreams", numStreams).
		Int("discBatch", discBatch).
		Msg("[StreamManager] discovering")
	if discBatch < 0 {
		return nil, nil
	}

	ctx2, cancel := context.WithTimeout(ctx, discTimeout)
	go func() { // avoid context leak
		<-time.After(discTimeout)
		cancel()
	}()
	return sm.pf.FindPeers(ctx2, string(sm.myProtoID), discBatch)
}

func (sm *streamManager) setupStreamWithPeer(ctx context.Context, pid libp2p_peer.ID, trusted bool) error {
	timer := prometheus.NewTimer(setupStreamDuration.With(prometheus.Labels{"topic": string(sm.myProtoID)}))
	defer timer.ObserveDuration()

	nCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	st, err := sm.host.NewStream(nCtx, pid, protocol.ID(sm.myProtoID))
	if err != nil {
		return err
	}
	if sm.handleStream != nil {
		go sm.handleStream(st, trusted)
	}
	return nil
}

func (sm *streamManager) softHaveEnoughStreams() bool {
	availStreams := sm.streams.numStreamsWithMinProtoSpec(sm.myProtoSpec)
	return availStreams >= sm.config.SoftLoCap
}

func (sm *streamManager) hardHaveEnoughStream() bool {
	availStreams := sm.streams.numStreamsWithMinProtoSpec(sm.myProtoSpec)
	return availStreams >= sm.config.HardLoCap
}

func (sm *streamManager) hardRequiredStreams() int {
	availStreams := sm.streams.numStreamsWithMinProtoSpec(sm.myProtoSpec)
	if availStreams >= sm.config.HardLoCap {
		return 0
	}
	return sm.config.HardLoCap - availStreams
}
