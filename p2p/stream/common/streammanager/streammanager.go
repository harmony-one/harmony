package streammanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
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
	// reserved streams
	reservedStreams *streamSet
	// libp2p utilities
	host         host
	pf           peerFinder
	handleStream func(stream network.Stream)
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
}

// NewStreamManager creates a new stream manager for the given proto ID
func NewStreamManager(pid sttypes.ProtoID, host host, pf peerFinder, handleStream func(network.Stream), c Config) StreamManager {
	return newStreamManager(pid, host, pf, handleStream, c)
}

// newStreamManager creates a new stream manager
func newStreamManager(pid sttypes.ProtoID, host host, pf peerFinder, handleStream func(network.Stream), c Config) *streamManager {
	ctx, cancel := context.WithCancel(context.Background())

	logger := utils.Logger().With().Str("module", "stream manager").
		Str("protocol ID", string(pid)).Logger()

	protoSpec, _ := sttypes.ProtoIDToProtoSpec(pid)

	// if it is a beacon node or shard node, print the peer id and proto id
	if protoSpec.IsBeaconValidator || protoSpec.ShardID != shard.BeaconChainShardID {
		fmt.Println("My peer id: ", host.ID().String())
		fmt.Println("My proto id: ", pid)
	}

	return &streamManager{
		myProtoID:       pid,
		myProtoSpec:     protoSpec,
		config:          c,
		streams:         newStreamSet(),
		reservedStreams: newStreamSet(),
		host:            host,
		pf:              pf,
		handleStream:    handleStream,
		addStreamCh:     make(chan addStreamTask),
		rmStreamCh:      make(chan rmStreamTask),
		stopCh:          make(chan stopTask),
		discCh:          make(chan discTask, 1), // discCh is a buffered channel to avoid overuse of goroutine
		coolDown:        abool.New(),
		coolDownCache:   newCoolDownCache(),
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
	}
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
	// bootstrap discovery
	sm.discCh <- discTask{}

	for {
		select {
		case <-discTicker.C:
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
			err := sm.handleRemoveStream(rmStream.id)
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

// NewStream handles a new stream from stream handler protocol
func (sm *streamManager) NewStream(stream sttypes.Stream) error {
	if err := sm.sanityCheckStream(stream); err != nil {
		return errors.Wrap(err, "stream sanity check failed")
	}
	task := addStreamTask{
		st:   stream,
		errC: make(chan error),
	}
	sm.addStreamCh <- task
	return <-task.errC
}

// RemoveStream close and remove a stream from stream manager
func (sm *streamManager) RemoveStream(stID sttypes.StreamID) error {
	task := rmStreamTask{
		id:   stID,
		errC: make(chan error),
	}
	sm.rmStreamCh <- task
	return <-task.errC
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
		id   sttypes.StreamID
		errC chan error
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

	// If the stream list has sufficient capacity, the stream can be added to the reserved list
	if sm.streams.size() >= sm.config.HiCap {
		if sm.reservedStreams.size() < MaxReservedStreams {
			if _, ok := sm.reservedStreams.get(id); !ok {
				sm.reservedStreams.addStream(st)
				sm.logger.Info().
					Int("NumStreams", sm.streams.size()).
					Int("NumReservedStreams", sm.reservedStreams.size()).
					Interface("StreamID", id).
					Msg("[StreamManager] added new stream to reserved list")
			}
			return nil
		}
		return ErrTooManyStreams
	}

	sm.streams.addStream(st)
	sm.logger.Info().
		Int("NumStreams", sm.streams.size()).
		Interface("StreamID", id).
		Msg("[StreamManager] added new stream to main streams list")

	sm.addStreamFeed.Send(EvtStreamAdded{st})
	addedStreamsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
	numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
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
		added++
	}
	return added, nil
}

func (sm *streamManager) handleRemoveStream(id sttypes.StreamID) error {
	st, ok := sm.streams.get(id)
	if !ok {
		return ErrStreamAlreadyRemoved
	}

	sm.streams.deleteStream(st)

	// try to replace removed streams from reserved list
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

	sm.removeStreamFeed.Send(EvtStreamRemoved{id})
	removedStreamsCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()
	numStreamsGaugeVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Set(float64(sm.streams.size()))
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

func (sm *streamManager) discoverAndSetupStream(discCtx context.Context) (int, error) {
	peers, err := sm.discover(discCtx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to discover")
	}
	discoverCounterVec.With(prometheus.Labels{"topic": string(sm.myProtoID)}).Inc()

	connecting := 0
	for peer := range peers {
		if peer.ID == sm.host.ID() {
			continue
		}
		if sm.coolDownCache.Has(peer.ID) {
			// If the peer has the same ID and was just connected, skip.
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
			// The ctx here is using the module context instead of discover context
			err := sm.setupStreamWithPeer(sm.ctx, pid)
			if err != nil {
				sm.coolDownCache.Add(pid)
				sm.logger.Warn().Err(err).
					Interface("peerID", pid).
					Msg("failed to setup stream with peer")
				return
			}
		}(peer.ID)
	}
	return connecting, nil
}

func (sm *streamManager) discover(ctx context.Context) (<-chan libp2p_peer.AddrInfo, error) {
	numStreams := sm.streams.size()

	protoID := sm.targetProtoID()
	discBatch := sm.config.DiscBatch
	if sm.config.HiCap-numStreams < sm.config.DiscBatch {
		discBatch = sm.config.HiCap - numStreams
	}
	sm.logger.Debug().
		Interface("protoID", protoID).
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
	return sm.pf.FindPeers(ctx2, protoID, discBatch)
}

func (sm *streamManager) targetProtoID() string {
	targetSpec := sm.myProtoSpec
	if targetSpec.ShardID == shard.BeaconChainShardID { // for beacon chain, only connect to beacon nodes
		targetSpec.IsBeaconValidator = true
	}
	return string(targetSpec.ToProtoID())
}

func (sm *streamManager) setupStreamWithPeer(ctx context.Context, pid libp2p_peer.ID) error {
	timer := prometheus.NewTimer(setupStreamDuration.With(prometheus.Labels{"topic": string(sm.myProtoID)}))
	defer timer.ObserveDuration()

	nCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	st, err := sm.host.NewStream(nCtx, pid, protocol.ID(sm.targetProtoID()))
	if err != nil {
		return err
	}
	if sm.handleStream != nil {
		go sm.handleStream(st)
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
