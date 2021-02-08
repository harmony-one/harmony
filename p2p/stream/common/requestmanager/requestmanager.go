package requestmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

// requestManager implements RequestManager. It is responsible for matching response
// with requests.
// TODO: each peer is able to have a queue of requests instead of one request at a time.
// TODO: add QoS evaluation for each stream
type requestManager struct {
	streams   map[sttypes.StreamID]*stream  // All streams
	available map[sttypes.StreamID]struct{} // Streams that are available for request
	pendings  map[uint64]*request           // requests that are sent but not received response
	waitings  requestQueue                  // double linked list of requests that are on the waiting list

	// Stream events
	newStreamC <-chan streammanager.EvtStreamAdded
	rmStreamC  <-chan streammanager.EvtStreamRemoved
	// Request events
	cancelReqC  chan uint64 // request being canceled
	retryReqC   chan uint64 // request to be retried
	deliveryC   chan deliverData
	newRequestC chan *request

	subs   []event.Subscription
	logger zerolog.Logger
	stopC  chan struct{}
	lock   sync.Mutex
}

// NewRequestManager creates a new request manager
func NewRequestManager(sm streammanager.Subscriber) RequestManager {
	return newRequestManager(sm)
}

func newRequestManager(sm streammanager.Subscriber) *requestManager {
	// subscribe at initialize to prevent misuse of upper function which might cause
	// the bootstrap peers are ignored
	newStreamC := make(chan streammanager.EvtStreamAdded)
	rmStreamC := make(chan streammanager.EvtStreamRemoved)
	sub1 := sm.SubscribeAddStreamEvent(newStreamC)
	sub2 := sm.SubscribeRemoveStreamEvent(rmStreamC)

	logger := utils.Logger().With().Str("module", "request manager").Logger()

	return &requestManager{
		streams:   make(map[sttypes.StreamID]*stream),
		available: make(map[sttypes.StreamID]struct{}),
		pendings:  make(map[uint64]*request),
		waitings:  newRequestQueue(),

		newStreamC:  newStreamC,
		rmStreamC:   rmStreamC,
		cancelReqC:  make(chan uint64, 16),
		retryReqC:   make(chan uint64, 16),
		deliveryC:   make(chan deliverData, 128),
		newRequestC: make(chan *request, 128),

		subs:   []event.Subscription{sub1, sub2},
		logger: logger,
		stopC:  make(chan struct{}),
	}
}

func (rm *requestManager) Start() {
	go rm.loop()
}

func (rm *requestManager) Close() {
	rm.stopC <- struct{}{}
}

// DoRequest do the given request with a stream picked randomly. Return the response, stream id that
// is responsible for response, delivery and error.
func (rm *requestManager) DoRequest(ctx context.Context, raw sttypes.Request, options ...RequestOption) (sttypes.Response, sttypes.StreamID, error) {
	resp := <-rm.doRequestAsync(ctx, raw, options...)
	return resp.raw, resp.stID, resp.err
}

func (rm *requestManager) doRequestAsync(ctx context.Context, raw sttypes.Request, options ...RequestOption) <-chan response {
	req := &request{
		Request: raw,
		respC:   make(chan deliverData),
		waitCh:  make(chan struct{}),
	}
	for _, opt := range options {
		opt(req)
	}

	ctx, _ = context.WithTimeout(ctx, reqTimeOut)
	rm.newRequestC <- req

	resC := make(chan response, 1)

	go func() {
		defer req.cancel()
		select {
		case <-ctx.Done(): // canceled or timeout in upper function calls
			rm.cancelReqC <- req.ReqID()
			resC <- response{err: ctx.Err()}

		case data := <-req.respC:
			resC <- response{raw: data.resp, stID: data.stID, err: req.err}
		}
	}()
	return resC
}

// DeliverResponse delivers the response to the corresponding request.
// The function behaves non-block
func (rm *requestManager) DeliverResponse(stID sttypes.StreamID, resp sttypes.Response) {
	dlv := deliverData{
		resp: resp,
		stID: stID,
	}
	go func() {
		select {
		case rm.deliveryC <- dlv:
		case <-time.After(deliverTimeout):
			rm.logger.Error().Msg("WARNING: delivery timeout. Possible stuck in loop")
		}
	}()
}

func (rm *requestManager) loop() {
	var (
		throttleC = make(chan struct{}, 1) // throttle the waiting requests periodically
		ticker    = time.NewTicker(throttleInterval)
	)
	throttle := func() {
		select {
		case throttleC <- struct{}{}:
		default:
		}
	}

	for {
		select {
		case <-ticker.C:
			throttle()

		case <-throttleC:
		loop:
			for i := 0; i != throttleBatch; i++ {
				req, st := rm.getNextRequest()
				if req == nil {
					break loop
				}
				rm.addPendingRequest(req, st)
				b, err := req.Encode()
				if err != nil {
					rm.logger.Warn().Str("request", req.String()).Err(err).
						Msg("request encode error")
				}

				go func(reqID uint64) {
					if err := st.WriteBytes(b); err != nil {
						rm.logger.Warn().Str("streamID", string(st.ID())).Err(err).
							Msg("failed to send request")
						// TODO: Decide whether we also need to close the stream here based
						//   on the error and retry times.
						rm.retryReqC <- reqID
					}
					go func() {
						select {
						case <-time.After(reqRetryTimeOut):
							// request still not received after reqRetryTimeOut, try again.
							rm.retryReqC <- reqID
						case <-req.waitCh:
							// request cancelled or response received. Do nothing and return
						}
					}()
				}(req.ReqID())
			}

		case req := <-rm.newRequestC:
			added := rm.handleNewRequest(req)
			if added {
				throttle()
			}

		case data := <-rm.deliveryC:
			rm.handleDeliverData(data)

		case reqID := <-rm.retryReqC:
			addedBacked := rm.handleRetryRequest(reqID)
			if addedBacked {
				throttle()
			}

		case reqID := <-rm.cancelReqC:
			rm.handleCancelRequest(reqID)

		case evt := <-rm.newStreamC:
			rm.logger.Info().Str("streamID", string(evt.Stream.ID())).Msg("add new stream")
			rm.addNewStream(evt.Stream)

		case evt := <-rm.rmStreamC:
			rm.logger.Info().Str("streamID", string(evt.ID)).Msg("remove stream")
			reqCanceled := rm.removeStream(evt.ID)
			if reqCanceled {
				throttle()
			}

		case <-rm.stopC:
			rm.logger.Info().Msg("request manager stopped")
			rm.close()
			return
		}
	}
}

func (rm *requestManager) handleNewRequest(req *request) bool {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	err := rm.addRequestToWaitings(req, reqPriorityLow)
	if err != nil {
		rm.logger.Warn().Err(err).Msg("failed to add new request to waitings")
		req.err = errors.Wrap(err, "failed to add new request to waitings")
		req.respC <- deliverData{}
		return false
	}
	return true
}

func (rm *requestManager) handleDeliverData(data deliverData) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	if err := rm.validateDelivery(data); err != nil {
		// if error happens in delivery, most likely it's a stale delivery. No action needed
		// and return
		rm.logger.Info().Err(err).Str("response", data.resp.String()).Msg("unable to validate deliver")
		return
	}
	// req and st is ensured not to be empty in validateDelivery
	req := rm.pendings[data.resp.ReqID()]
	req.respC <- data
	rm.removePendingRequest(req)
}

func (rm *requestManager) validateDelivery(data deliverData) error {
	st := rm.streams[data.stID]
	if st == nil {
		return fmt.Errorf("data delivered from dead stream: %v", data.stID)
	}
	req := rm.pendings[data.resp.ReqID()]
	if req == nil {
		return fmt.Errorf("stale p2p response delivery")
	}
	if req.owner == nil || req.owner.ID() != data.stID {
		return fmt.Errorf("unexpected delivery stream")
	}
	if st.req == nil || st.req.ReqID() != data.resp.ReqID() {
		// Possible when request is canceled
		return fmt.Errorf("unexpected deliver request")
	}
	return nil
}

func (rm *requestManager) handleCancelRequest(reqID uint64) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	req, ok := rm.pendings[reqID]
	if !ok {
		return
	}
	rm.removePendingRequest(req)
}

func (rm *requestManager) handleRetryRequest(reqID uint64) bool {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	req, ok := rm.pendings[reqID]
	if !ok {
		return false
	}
	// sanity check. If a request is done before, its owner must not be nil.
	if req.owner != nil {
		req.addBlacklistedStream(req.owner.ID())
	}
	rm.removePendingRequest(req)

	if err := rm.addRequestToWaitings(req, reqPriorityMed); err != nil {
		rm.logger.Warn().Err(err).Msg("cannot add request to waitings during retry")
		req.err = errors.Wrap(err, "cannot add request to waitings during retry")
		req.respC <- deliverData{}
		return false
	}
	return true
}

func (rm *requestManager) getNextRequest() (*request, *stream) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	var req *request
	for {
		req = rm.waitings.Pop()
		if req == nil {
			return nil, nil
		}
		if !req.isCanceled() {
			break
		}
	}

	st, err := rm.pickAvailableStream(req)
	if err != nil {
		rm.logger.Debug().Msg("No available streams.")
		rm.addRequestToWaitings(req, reqPriorityHigh)
		return nil, nil
	}
	return req, st
}

func (rm *requestManager) genReqID() uint64 {
	for {
		rid := sttypes.GenReqID()
		if _, ok := rm.pendings[rid]; !ok {
			return rid
		}
	}
}

func (rm *requestManager) addPendingRequest(req *request, st *stream) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	reqID := rm.genReqID()
	req.SetReqID(reqID)

	req.owner = st
	st.req = req

	delete(rm.available, st.ID())
	rm.pendings[req.ReqID()] = req
}

func (rm *requestManager) removePendingRequest(req *request) {
	delete(rm.pendings, req.ReqID())

	if st := req.owner; st != nil {
		st.clearPendingRequest()
		req.clearOwner()
		rm.available[st.ID()] = struct{}{}
	}
}

func (rm *requestManager) pickAvailableStream(req *request) (*stream, error) {
	for id := range rm.available {
		if !req.isStreamAllowed(id) {
			continue
		}
		st, ok := rm.streams[id]
		if !ok {
			return nil, errors.New("sanity error: available stream not registered")
		}
		if st.req != nil {
			return nil, errors.New("sanity error: available stream has pending requests")
		}
		spec, _ := st.ProtoSpec()
		if req.Request.IsSupportedByProto(spec) {
			return st, nil
		}
	}
	return nil, errors.New("no more available streams")
}

func (rm *requestManager) addNewStream(st sttypes.Stream) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	if _, ok := rm.streams[st.ID()]; !ok {
		rm.streams[st.ID()] = &stream{Stream: st}
		rm.available[st.ID()] = struct{}{}
	}
}

// removeStream remove the stream from request manager, clear the pending request
// of the stream. Return whether a pending request is canceled in the stream,
func (rm *requestManager) removeStream(id sttypes.StreamID) bool {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	st, ok := rm.streams[id]
	if !ok {
		return false
	}
	delete(rm.available, id)
	delete(rm.streams, id)

	cleared := st.clearPendingRequest()
	if cleared != nil {
		cleared.clearOwner()
		if err := rm.addRequestToWaitings(cleared, reqPriorityMed); err != nil {
			rm.logger.Err(err).Msg("cannot add new request to waitings in removeStream")
			return false
		}
		return true
	}
	return false
}

func (rm *requestManager) close() {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	for _, sub := range rm.subs {
		sub.Unsubscribe()
	}
	for _, req := range rm.pendings {
		req.err = ErrClosed
		select {
		case req.respC <- deliverData{}:
		default:
		}
	}
	rm.pendings = make(map[uint64]*request)
	rm.available = make(map[sttypes.StreamID]struct{})
	rm.streams = make(map[sttypes.StreamID]*stream)
	rm.waitings = newRequestQueue()
	close(rm.stopC)
}

type reqPriority int

const (
	reqPriorityLow reqPriority = iota
	reqPriorityMed
	reqPriorityHigh
)

func (rm *requestManager) addRequestToWaitings(req *request, priority reqPriority) error {
	return rm.waitings.Push(req, priority)
}
