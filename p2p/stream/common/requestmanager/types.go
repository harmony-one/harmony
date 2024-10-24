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

	// ErrNotAvailableCapacity is the error that stream has fully occupied with requests
	ErrNotAvailableCapacity = errors.New("not available capacity")
)

var (
	MaxWorkerStreamCapacity = 10
)

// WorkerStream is the wrapped version of sttypes.Stream.
type WorkerStream struct {
	sttypes.Stream
	//req *WorkerRequest // currently one stream is dealing with one request
	reqs *sttypes.SafeMap[uint64, *WorkerRequest]
}

func NewWorkerStream(st sttypes.Stream) *WorkerStream {
	return &WorkerStream{
		Stream: st,
		reqs:   sttypes.NewSafeMap[uint64, *WorkerRequest](),
	}
}

func (ws *WorkerStream) GetRequest(ID uint64) (*WorkerRequest, bool) {
	return ws.reqs.Get(ID)
}

func (ws *WorkerStream) AvailableCapacity() int {
	cap := MaxWorkerStreamCapacity - ws.reqs.Length()
	if cap < 0 {
		cap = 0
	}
	return cap
}

func (ws *WorkerStream) AssignRequest(req *WorkerRequest) error {
	cap := ws.AvailableCapacity()
	if cap <= 0 {
		return ErrNotAvailableCapacity
	}
	ws.reqs.Set(req.ID(), req)
	return nil
}

func (ws *WorkerStream) RemoveRequest(req *WorkerRequest) error {
	ws.reqs.Delete(req.ID())
	return nil
}

// request is the wrapped request within module
type WorkerRequest struct {
	sttypes.Request // underlying request
	// result field
	respC chan responseData // channel to receive response from delivered message
	// concurrency control
	atmDone uint32
	doneC   chan struct{}
	// stream info
	ownerID sttypes.StreamID // Current owner
	// utils
	lock sync.RWMutex
	raw  *interface{}
	// options
	priority  reqPriority
	blacklist map[sttypes.StreamID]struct{} // banned streams
	whitelist map[sttypes.StreamID]struct{} // allowed streams
}

func (req *WorkerRequest) ID() uint64 {
	req.lock.RLock()
	defer req.lock.RUnlock()

	return req.Request.ID()
}

func (req *WorkerRequest) SetID(val uint64) {
	req.lock.Lock()
	defer req.lock.Unlock()

	req.Request.SetID(val)
}

func (req *WorkerRequest) OwnerID() sttypes.StreamID {
	req.lock.RLock()
	defer req.lock.RUnlock()

	return req.ownerID
}

func (req *WorkerRequest) SetOwnerID(id sttypes.StreamID) {
	req.lock.Lock()
	defer req.lock.Unlock()

	req.ownerID = id
}

func (req *WorkerRequest) doneWithResponse(resp responseData) {
	notDone := atomic.CompareAndSwapUint32(&req.atmDone, 0, 1)
	if notDone {
		req.respC <- resp
		close(req.respC)
		close(req.doneC)
	}
}

func (req *WorkerRequest) isDone() bool {
	return atomic.LoadUint32(&req.atmDone) == 1
}

func (req *WorkerRequest) isStreamAllowed(stid sttypes.StreamID) bool {
	return req.isStreamWhitelisted(stid) && !req.isStreamBlacklisted(stid)
}

func (req *WorkerRequest) addBlacklistedStream(stid sttypes.StreamID) {
	if req.blacklist == nil {
		req.blacklist = make(map[sttypes.StreamID]struct{})
	}
	req.blacklist[stid] = struct{}{}
}

func (req *WorkerRequest) isStreamBlacklisted(stid sttypes.StreamID) bool {
	if req.blacklist == nil {
		return false
	}
	_, ok := req.blacklist[stid]
	return ok
}

func (req *WorkerRequest) addWhiteListStream(stid sttypes.StreamID) {
	if req.whitelist == nil {
		req.whitelist = make(map[sttypes.StreamID]struct{})
	}
	req.whitelist[stid] = struct{}{}
}

func (req *WorkerRequest) isStreamWhitelisted(stid sttypes.StreamID) bool {
	if req.whitelist == nil {
		return true
	}
	_, ok := req.whitelist[stid]
	return ok
}

func (st *WorkerStream) clearPendingRequest() *sttypes.SafeMap[uint64, *WorkerRequest] {
	reqs := st.reqs
	if reqs == nil {
		return nil
	}
	st.reqs = sttypes.NewSafeMap[uint64, *WorkerRequest]()
	return reqs
}

type cancelReqData struct {
	req *WorkerRequest
	err error
}

// responseData is the wrapped response for stream requests
type responseData struct {
	resp sttypes.Response
	stID sttypes.StreamID
	err  error
}

// requestQueues is a wrapper of double linked list with Request as type
type requestQueues struct {
	reqsPHigh *requestQueue // high priority, currently defined by upper function calls
	reqsPLow  *requestQueue // low priority, applied to all normal requests
}

func newRequestQueues() requestQueues {
	return requestQueues{
		reqsPHigh: newRequestQueue(),
		reqsPLow:  newRequestQueue(),
	}
}

// Push add a new request to requestQueues.
func (q *requestQueues) Push(req *WorkerRequest, priority reqPriority) error {
	if priority == reqPriorityHigh || req.priority == reqPriorityHigh {
		return q.reqsPHigh.push(req)
	}
	return q.reqsPLow.push(req)
}

// Pop will first pop the request from high priority, and then pop from low priority
func (q *requestQueues) Pop() *WorkerRequest {
	if req := q.reqsPHigh.pop(); req != nil {
		return req
	}
	return q.reqsPLow.pop()
}

func (q *requestQueues) Remove(req *WorkerRequest) {
	q.reqsPHigh.remove(req)
	q.reqsPLow.remove(req)
}

// requestQueue is a thread safe request double linked list
type requestQueue struct {
	l     *list.List
	elemM map[*WorkerRequest]*list.Element // Yes, pointer as map key
	lock  sync.Mutex
}

func newRequestQueue() *requestQueue {
	return &requestQueue{
		l:     list.New(),
		elemM: make(map[*WorkerRequest]*list.Element),
	}
}

func (rl *requestQueue) push(req *WorkerRequest) error {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	if rl.l.Len() >= maxWaitingSize {
		return ErrQueueFull
	}
	elem := rl.l.PushBack(req)
	rl.elemM[req] = elem
	return nil
}

func (rl *requestQueue) pop() *WorkerRequest {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	elem := rl.l.Front()
	if elem == nil {
		return nil
	}
	rl.l.Remove(elem)

	req := elem.Value.(*WorkerRequest)
	delete(rl.elemM, req)
	return req
}

func (rl *requestQueue) remove(req *WorkerRequest) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	elem := rl.elemM[req]
	if elem == nil {
		// Already removed
		return
	}
	rl.l.Remove(elem)
	delete(rl.elemM, req)
}

func (rl *requestQueue) len() int {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	return rl.l.Len()
}
