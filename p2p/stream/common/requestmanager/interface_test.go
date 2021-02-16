package requestmanager

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

var testProtoID = sttypes.ProtoID("harmony/sync/unitest/0/1.0.0")

type testStreamManager struct {
	newStreamFeed event.Feed
	rmStreamFeed  event.Feed
}

func newTestStreamManager() *testStreamManager {
	return &testStreamManager{}
}

func (sm *testStreamManager) addNewStream(st sttypes.Stream) {
	sm.newStreamFeed.Send(streammanager.EvtStreamAdded{Stream: st})
}

func (sm *testStreamManager) rmStream(stid sttypes.StreamID) {
	sm.rmStreamFeed.Send(streammanager.EvtStreamRemoved{ID: stid})
}

func (sm *testStreamManager) SubscribeAddStreamEvent(ch chan<- streammanager.EvtStreamAdded) event.Subscription {
	return sm.newStreamFeed.Subscribe(ch)
}

func (sm *testStreamManager) SubscribeRemoveStreamEvent(ch chan<- streammanager.EvtStreamRemoved) event.Subscription {
	return sm.rmStreamFeed.Subscribe(ch)
}

type testStream struct {
	id      sttypes.StreamID
	rm      *requestManager
	deliver func(*testRequest) // use goroutine inside this function
}

func (st *testStream) ID() sttypes.StreamID {
	return st.id
}

func (st *testStream) ProtoID() sttypes.ProtoID {
	return testProtoID
}

func (st *testStream) WriteBytes(b []byte) error {
	req, err := decodeTestRequest(b)
	if err != nil {
		return err
	}
	if st.rm != nil && st.deliver != nil {
		st.deliver(req)
	}
	return nil
}

func (st *testStream) ReadBytes() ([]byte, error) {
	return nil, nil
}

func (st *testStream) ProtoSpec() (sttypes.ProtoSpec, error) {
	return sttypes.ProtoIDToProtoSpec(testProtoID)
}

func (st *testStream) Close() error {
	return nil
}

func (st *testStream) ResetOnClose() error {
	return nil
}

func makeStreamID(index int) sttypes.StreamID {
	return sttypes.StreamID(strconv.Itoa(index))
}

type testRequest struct {
	reqID uint64
	index uint64
}

func makeTestRequest(index uint64) *testRequest {
	return &testRequest{
		reqID: 0,
		index: index,
	}
}

func (req *testRequest) ReqID() uint64 {
	return req.reqID
}

func (req *testRequest) SetReqID(rid uint64) {
	req.reqID = rid
}

func (req *testRequest) String() string {
	return fmt.Sprintf("test request %v", req.index)
}

func (req *testRequest) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(struct {
		ReqID uint64
		Index uint64
	}{
		ReqID: req.reqID,
		Index: req.index,
	})
}

func (req *testRequest) checkResponse(rawResp sttypes.Response) error {
	resp, ok := rawResp.(*testResponse)
	if !ok || resp == nil {
		return errors.New("not test Response")
	}
	if req.reqID != resp.reqID {
		return errors.New("request id not expected")
	}
	if req.index != resp.index {
		return errors.New("response id not expected")
	}
	return nil
}

func decodeTestRequest(b []byte) (*testRequest, error) {
	type SerRequest struct {
		ReqID uint64
		Index uint64
	}
	var sr SerRequest
	if err := rlp.DecodeBytes(b, &sr); err != nil {
		return nil, err
	}
	return &testRequest{
		reqID: sr.ReqID,
		index: sr.Index,
	}, nil
}

func (req *testRequest) IsSupportedByProto(spec sttypes.ProtoSpec) bool {
	return true
}

func (req *testRequest) getResponse() *testResponse {
	return &testResponse{
		reqID: req.reqID,
		index: req.index,
	}
}

type testResponse struct {
	reqID uint64
	index uint64
}

func (tr *testResponse) ReqID() uint64 {
	return tr.reqID
}

func (tr *testResponse) String() string {
	return fmt.Sprintf("test response %v", tr.index)
}
