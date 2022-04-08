package requestmanager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
)

var (
	defTestSleep = 50 * time.Millisecond
)

// Request is delivered right away as expected
func TestRequestManager_Request_Normal(t *testing.T) {
	delayF := makeDefaultDelayFunc(150 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	res := <-ts.rm.doRequestAsync(ctx, req)

	if res.err != nil {
		t.Errorf("unexpected error: %v", res.err)
		return
	}
	if err := req.checkResponse(res.resp); err != nil {
		t.Error(err)
	}
	if res.stID == "" {
		t.Errorf("unexpected stid")
	}
}

// The request is canceled by context
func TestRequestManager_Request_Cancel(t *testing.T) {
	delayF := makeDefaultDelayFunc(500 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	resC := ts.rm.doRequestAsync(ctx, req)

	time.Sleep(defTestSleep)
	cancel()

	res := <-resC
	if res.err != context.Canceled {
		t.Errorf("unexpected error: %v", res.err)
	}
	if res.stID == "" {
		t.Errorf("unexpected canceled request should also have stid")
	}
}

// error happens when adding request to waiting list
func TestRequestManager_NewStream(t *testing.T) {
	delayF := makeDefaultDelayFunc(500 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()
	defer ts.Close()

	ts.sm.addNewStream(ts.makeTestStream(3))

	time.Sleep(defTestSleep)

	ts.rm.lock.Lock()
	if len(ts.rm.streams) != 4 || len(ts.rm.available) != 4 {
		t.Errorf("unexpected stream size")
	}
	ts.rm.lock.Unlock()
}

// For request assigned to the stream being removed, the request will be rescheduled.
func TestRequestManager_RemoveStream(t *testing.T) {
	delayF := makeOnceBlockDelayFunc(150 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resC := ts.rm.doRequestAsync(ctx, req)
	time.Sleep(defTestSleep)

	// remove the stream which is responsible for the request
	idToRemove := ts.pickOneOccupiedStream()
	ts.sm.rmStream(idToRemove)

	// the request is rescheduled thus there is supposed to be no errors
	res := <-resC
	if res.err == nil {
		t.Errorf("unexpected error: %v", errors.New("stream removed when doing request"))
	}

	ts.rm.lock.Lock()
	if len(ts.rm.streams) != 2 || len(ts.rm.available) != 2 {
		t.Errorf("unexpected stream size")
	}
	ts.rm.lock.Unlock()
}

// stream delivers an unknown request ID
func TestRequestManager_UnknownDelivery(t *testing.T) {
	delayF := makeDefaultDelayFunc(150 * time.Millisecond)
	respF := func(req *testRequest) *testResponse {
		var rid uint64
		for rid == req.reqID {
			rid++
		}
		return &testResponse{
			reqID: rid,
			index: 0,
		}
	}
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	resC := ts.rm.doRequestAsync(ctx, req)
	time.Sleep(2 * time.Second)
	cancel()

	// Since the reqID is not delivered, the result is not delivered to the request
	// and be canceled
	res := <-resC
	if res.err != context.Canceled {
		t.Errorf("unexpected error: %v", res.err)
	}
}

// stream delivers a response for a canceled request
func TestRequestManager_StaleDelivery(t *testing.T) {
	delayF := makeDefaultDelayFunc(1 * time.Second)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	resC := ts.rm.doRequestAsync(ctx, req)
	time.Sleep(2 * time.Second)

	// Since the reqID is not delivered, the result is not delivered to the request
	// and be canceled
	res := <-resC
	if res.err != context.DeadlineExceeded {
		t.Errorf("unexpected error: %v", res.err)
	}
}

// TestRequestManager_cancelWaitings test the scenario of request being canceled
// while still in waitings. In order to do this,
// 1. Set number of streams to 1
// 2. Occupy the stream with a request, and block
// 3. Do the second request. This request will be in waitings.
// 4. Cancel the second request. Request shall be removed from waitings.
// 5. Unblock the first request
// 6. Request 1 finished, request 2 canceled
func TestRequestManager_cancelWaitings(t *testing.T) {
	req1 := makeTestRequest(1)
	req2 := makeTestRequest(2)

	var req1Block sync.Mutex
	req1Block.Lock()
	unblockReq1 := func() { req1Block.Unlock() }

	delayF := makeDefaultDelayFunc(150 * time.Millisecond)
	respF := func(req *testRequest) *testResponse {
		if req.index == req1.index {
			req1Block.Lock()
		}
		return makeDefaultResponseFunc()(req)
	}
	ts := newTestSuite(delayF, respF, 1)
	ts.Start()
	defer ts.Close()

	ctx1, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	resC1 := ts.rm.doRequestAsync(ctx1, req1)
	resC2 := ts.rm.doRequestAsync(ctx2, req2)

	cancel2()
	unblockReq1()

	var (
		res1 responseData
		res2 responseData
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		select {
		case res1 = <-resC1:
		case <-time.After(1 * time.Second):
			t.Errorf("req1 timed out")
		}
	}()
	go func() {
		defer wg.Done()

		select {
		case res2 = <-resC2:
		case <-time.After(1 * time.Second):
			t.Errorf("req2 timed out")
		}
	}()
	wg.Wait()

	if res1.err != nil {
		t.Errorf("request 1 shall return nil error")
	}
	if res2.err != context.Canceled {
		t.Errorf("request 2 shall be canceled")
	}
	if ts.rm.waitings.reqsPLow.len() != 0 || ts.rm.waitings.reqsPHigh.len() != 0 {
		t.Errorf("waitings shall be clean")
	}
}

// closing request manager will also close all
func TestRequestManager_Close(t *testing.T) {
	delayF := makeDefaultDelayFunc(1 * time.Second)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 3)
	ts.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resC := ts.rm.doRequestAsync(ctx, makeTestRequest(0))
	time.Sleep(100 * time.Millisecond)
	ts.Close()

	// Since the reqID is not delivered, the result is not delivered to the request
	// and be canceled
	res := <-resC
	if assErr := assertError(res.err, errors.New("request manager module closed")); assErr != nil {
		t.Errorf("unexpected error: %v", assErr)
	}
}

func TestRequestManager_Request_Blacklist(t *testing.T) {
	delayF := makeDefaultDelayFunc(150 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 4)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	res := <-ts.rm.doRequestAsync(ctx, req, WithBlacklist([]sttypes.StreamID{
		makeStreamID(0),
		makeStreamID(1),
		makeStreamID(2),
	}))

	if res.err != nil {
		t.Errorf("unexpected error: %v", res.err)
		return
	}
	if err := req.checkResponse(res.resp); err != nil {
		t.Error(err)
	}
	if res.stID != makeStreamID(3) {
		t.Errorf("unexpected stid")
	}
}

func TestRequestManager_Request_Whitelist(t *testing.T) {
	delayF := makeDefaultDelayFunc(150 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 4)
	ts.Start()
	defer ts.Close()

	req := makeTestRequest(100)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	res := <-ts.rm.doRequestAsync(ctx, req, WithWhitelist([]sttypes.StreamID{
		makeStreamID(3),
	}))

	if res.err != nil {
		t.Errorf("unexpected error: %v", res.err)
		return
	}
	if err := req.checkResponse(res.resp); err != nil {
		t.Error(err)
	}
	if res.stID != makeStreamID(3) {
		t.Errorf("unexpected stid")
	}
}

// test the race condition by spinning up a lot of goroutines
func TestRequestManager_Concurrency(t *testing.T) {
	var (
		testDuration = 10 * time.Second
		numThreads   = 25
	)
	delayF := makeDefaultDelayFunc(100 * time.Millisecond)
	respF := makeDefaultResponseFunc()
	ts := newTestSuite(delayF, respF, 18)
	ts.Start()

	stopC := make(chan struct{})
	var (
		aErr    atomic.Value
		numReqs uint64
		wg      sync.WaitGroup
	)
	wg.Add(numThreads)
	for i := 0; i != numThreads; i++ {
		go func() {
			defer wg.Done()
			for {
				resC := ts.rm.doRequestAsync(context.Background(), makeTestRequest(1000))
				select {
				case res := <-resC:
					if res.err == nil {
						atomic.AddUint64(&numReqs, 1)
						continue
					}
					if res.err.Error() == "request manager module closed" {
						return
					}
					aErr.Store(res.err.Error())
				case <-stopC:
					return
				}
			}
		}()
	}
	time.Sleep(testDuration)
	close(stopC)
	ts.Close()
	wg.Wait()

	if isNilErr := aErr.Load() == nil; !isNilErr {
		err := aErr.Load().(error)
		t.Errorf("unexpected error: %v", err)
	}
	num := atomic.LoadUint64(&numReqs)
	t.Logf("Mock processed requests: %v", num)
}

func TestGenReqID(t *testing.T) {
	retry := 100000
	rm := &requestManager{
		pendings: make(map[uint64]*request),
	}

	for i := 0; i != retry; i++ {
		rid := rm.genReqID()
		if _, ok := rm.pendings[rid]; ok {
			t.Errorf("rid collision")
		}
		rm.pendings[rid] = nil
	}
}

func TestCheckStreamUpdates(t *testing.T) {
	tests := []struct {
		exists            map[sttypes.StreamID]*stream
		targets           []sttypes.Stream
		expAddedIndexes   []int
		expRemovedIndexes []int
	}{
		{
			exists:            makeDummyStreamSets([]int{1, 2, 3, 4, 5}),
			targets:           makeDummyTestStreams([]int{2, 3, 4, 5}),
			expAddedIndexes:   []int{},
			expRemovedIndexes: []int{1},
		},
		{
			exists:            makeDummyStreamSets([]int{1, 2, 3, 4, 5}),
			targets:           makeDummyTestStreams([]int{1, 2, 3, 4, 5, 6}),
			expAddedIndexes:   []int{6},
			expRemovedIndexes: []int{},
		},
		{
			exists:            makeDummyStreamSets([]int{}),
			targets:           makeDummyTestStreams([]int{}),
			expAddedIndexes:   []int{},
			expRemovedIndexes: []int{},
		},
		{
			exists:            makeDummyStreamSets([]int{}),
			targets:           makeDummyTestStreams([]int{1, 2, 3, 4, 5}),
			expAddedIndexes:   []int{1, 2, 3, 4, 5},
			expRemovedIndexes: []int{},
		},
		{
			exists:            makeDummyStreamSets([]int{1, 2, 3, 4, 5}),
			targets:           makeDummyTestStreams([]int{}),
			expAddedIndexes:   []int{},
			expRemovedIndexes: []int{1, 2, 3, 4, 5},
		},
		{
			exists:            makeDummyStreamSets([]int{1, 2, 3, 4, 5}),
			targets:           makeDummyTestStreams([]int{6, 7, 8, 9, 10}),
			expAddedIndexes:   []int{6, 7, 8, 9, 10},
			expRemovedIndexes: []int{1, 2, 3, 4, 5},
		},
	}

	for i, test := range tests {
		added, removed := checkStreamUpdates(test.exists, test.targets)

		if err := checkStreamIDsEqual(added, test.expAddedIndexes); err != nil {
			t.Errorf("Test %v: check added: %v", i, err)
		}
		if err := checkStreamIDsEqual2(removed, test.expRemovedIndexes); err != nil {
			t.Errorf("Test %v: check removed: %v", i, err)
		}
	}
}

func checkStreamIDsEqual(sts []sttypes.Stream, expIndexes []int) error {
	if len(sts) != len(expIndexes) {
		return fmt.Errorf("size not equal")
	}
	expM := make(map[sttypes.StreamID]struct{})
	for _, index := range expIndexes {
		expM[makeStreamID(index)] = struct{}{}
	}
	for _, st := range sts {
		if _, ok := expM[st.ID()]; !ok {
			return fmt.Errorf("stream not exist in exp: %v", st.ID())
		}
	}
	return nil
}

func checkStreamIDsEqual2(sts []*stream, expIndexes []int) error {
	if len(sts) != len(expIndexes) {
		return fmt.Errorf("size not equal")
	}
	expM := make(map[sttypes.StreamID]struct{})
	for _, index := range expIndexes {
		expM[makeStreamID(index)] = struct{}{}
	}
	for _, st := range sts {
		if _, ok := expM[st.ID()]; !ok {
			return fmt.Errorf("stream not exist in exp: %v", st.ID())
		}
	}
	return nil
}

type testSuite struct {
	rm          *requestManager
	sm          *testStreamManager
	bootStreams []*testStream

	delayFunc delayFunc
	respFunc  responseFunc

	ctx    context.Context
	cancel func()
}

func newTestSuite(delayF delayFunc, respF responseFunc, numStreams int) *testSuite {
	sm := newTestStreamManager()
	rm := newRequestManager(sm)
	ctx, cancel := context.WithCancel(context.Background())

	ts := &testSuite{
		rm:          rm,
		sm:          sm,
		bootStreams: make([]*testStream, 0, numStreams),
		delayFunc:   delayF,
		respFunc:    respF,
		ctx:         ctx,
		cancel:      cancel,
	}
	for i := 0; i != numStreams; i++ {
		st := ts.makeTestStream(i)
		ts.bootStreams = append(ts.bootStreams, st)
	}
	return ts
}

func (ts *testSuite) Start() {
	ts.rm.Start()
	for _, st := range ts.bootStreams {
		ts.sm.addNewStream(st)
	}
}

func (ts *testSuite) Close() {
	ts.rm.Close()
	ts.cancel()
}

func (ts *testSuite) pickOneOccupiedStream() sttypes.StreamID {
	ts.rm.lock.Lock()
	defer ts.rm.lock.Unlock()

	for _, req := range ts.rm.pendings {
		return req.owner.ID()
	}
	return ""
}

type (
	// responseFunc is the function to compose a response
	responseFunc func(request *testRequest) *testResponse

	// delayFunc is the function to determine the delay to deliver a response
	delayFunc func() time.Duration
)

func makeDefaultResponseFunc() responseFunc {
	return func(request *testRequest) *testResponse {
		resp := &testResponse{
			reqID: request.reqID,
			index: request.index,
		}
		return resp
	}
}

func checkResponseMessage(request sttypes.Request, response sttypes.Response) error {
	tReq, ok := request.(*testRequest)
	if !ok || tReq == nil {
		return errors.New("request not testRequest")
	}
	tResp, ok := response.(*testResponse)
	if !ok || tResp == nil {
		return errors.New("response not testResponse")
	}
	return tReq.checkResponse(tResp)
}

func makeDefaultDelayFunc(delay time.Duration) delayFunc {
	return func() time.Duration {
		return delay
	}
}

func makeOnceBlockDelayFunc(normalDelay time.Duration) delayFunc {
	// This usage of once is nasty. Avoid using once like this in production code.
	var once sync.Once
	return func() time.Duration {
		var block bool
		once.Do(func() {
			block = true
		})
		if block {
			return time.Hour
		}
		return normalDelay
	}
}

func (ts *testSuite) makeTestStream(index int) *testStream {
	stid := makeStreamID(index)
	return &testStream{
		id: stid,
		rm: ts.rm,
		deliver: func(req *testRequest) {
			delay := ts.delayFunc()
			resp := ts.respFunc(req)
			go func() {
				select {
				case <-ts.ctx.Done():
				case <-time.After(delay):
					ts.rm.DeliverResponse(stid, resp)
				}
			}()
		},
	}
}
