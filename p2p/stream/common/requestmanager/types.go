package requestmanager

import (
	"container/list"
	"sync"
	"sync/atomic"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
)

var (
	// ErrQueueFull is the error happens when the waiting queue is already full
	ErrQueueFull = errors.New("waiting request queue already full")

	// ErrClosed is request error that the module is closed during request
	ErrClosed = errors.New("request manager module closed")
)

// stream is the wrapped version of sttypes.Stream.
// TODO: enable stream handle multiple pending requests at the same time
type stream struct {
	sttypes.Stream
	req *request // currently one stream is dealing with one request
}

// request is the wrapped request within module
type request struct {
	sttypes.Request // underlying request
	// result field
	respC chan responseData // channel to receive response from delivered message
	// concurrency control
	atmDone uint32
	doneC   chan struct{}
	// stream info
	owner *stream // Current owner
	// utils
	lock sync.RWMutex
	raw  *interface{}
	// options
	priority  reqPriority
	blacklist map[sttypes.StreamID]struct{} // banned streams
	whitelist map[sttypes.StreamID]struct{} // allowed streams
}

func (req *request) ReqID() uint64 {
	req.lock.RLock()
	defer req.lock.RUnlock()

	return req.Request.ReqID()
}

func (req *request) SetReqID(val uint64) {
	req.lock.Lock()
	defer req.lock.Unlock()

	req.Request.SetReqID(val)
}

func (req *request) doneWithResponse(resp responseData) {
	notDone := atomic.CompareAndSwapUint32(&req.atmDone, 0, 1)
	if notDone {
		req.respC <- resp
		close(req.respC)
		close(req.doneC)
	}
}

func (req *request) isDone() bool {
	return atomic.LoadUint32(&req.atmDone) == 1
}

func (req *request) isStreamAllowed(stid sttypes.StreamID) bool {
	return req.isStreamWhitelisted(stid) && !req.isStreamBlacklisted(stid)
}

func (req *request) addBlacklistedStream(stid sttypes.StreamID) {
	if req.blacklist == nil {
		req.blacklist = make(map[sttypes.StreamID]struct{})
	}
	req.blacklist[stid] = struct{}{}
}

func (req *request) isStreamBlacklisted(stid sttypes.StreamID) bool {
	if req.blacklist == nil {
		return false
	}
	_, ok := req.blacklist[stid]
	return ok
}

func (req *request) addWhiteListStream(stid sttypes.StreamID) {
	if req.whitelist == nil {
		req.whitelist = make(map[sttypes.StreamID]struct{})
	}
	req.whitelist[stid] = struct{}{}
}

func (req *request) isStreamWhitelisted(stid sttypes.StreamID) bool {
	if req.whitelist == nil {
		return true
	}
	_, ok := req.whitelist[stid]
	return ok
}

func (st *stream) clearPendingRequest() *request {
	req := st.req
	if req == nil {
		return nil
	}
	st.req = nil
	return req
}

type cancelReqData struct {
	reqID uint64
	err   error
}

// responseData is the wrapped response for stream requests
type responseData struct {
	resp sttypes.Response
	stID sttypes.StreamID
	err  error
}

// requestQueue is a wrapper of double linked list with Request as type
type requestQueue struct {
	reqsPHigh *list.List // high priority, currently defined by upper function calls
	reqsPLow  *list.List // low priority, applied to all normal requests
	lock      sync.Mutex
}

func newRequestQueue() requestQueue {
	return requestQueue{
		reqsPHigh: list.New(),
		reqsPLow:  list.New(),
	}
}

// Push add a new request to requestQueue.
func (q *requestQueue) Push(req *request, priority reqPriority) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	if priority == reqPriorityHigh || req.priority == reqPriorityHigh {
		return pushRequestToList(q.reqsPHigh, req)
	}
	if priority == reqPriorityLow {
		return pushRequestToList(q.reqsPLow, req)
	}
	return nil
}

// Pop will first pop the request from high priority, and then pop from low priority
func (q *requestQueue) Pop() *request {
	q.lock.Lock()
	defer q.lock.Unlock()

	if req := popRequestFromList(q.reqsPHigh); req != nil {
		return req
	}
	return popRequestFromList(q.reqsPLow)
}

func pushRequestToList(l *list.List, req *request) error {
	if l.Len() >= maxWaitingSize {
		return ErrQueueFull
	}
	l.PushBack(req)
	return nil
}

func popRequestFromList(l *list.List) *request {
	elem := l.Front()
	if elem == nil {
		return nil
	}
	l.Remove(elem)
	return elem.Value.(*request)
}
