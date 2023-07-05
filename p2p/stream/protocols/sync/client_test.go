package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	headerV3 "github.com/harmony-one/harmony/block/v3"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/stream/common/ratelimiter"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

var (
	_ sttypes.Request  = &getBlocksByNumberRequest{}
	_ sttypes.Request  = &getBlockNumberRequest{}
	_ sttypes.Request  = &getReceiptsRequest{}
	_ sttypes.Response = &syncResponse{&syncpb.Response{}}
)

var (
	initStreamIDs = []sttypes.StreamID{
		makeTestStreamID(0),
		makeTestStreamID(1),
		makeTestStreamID(2),
		makeTestStreamID(3),
	}
)

var (
	testHeader  = &block.Header{Header: headerV3.NewHeader()}
	testBlock   = types.NewBlockWithHeader(testHeader)
	testReceipt = &types.Receipt{
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 0x888888888,
		Logs:              []*types.Log{},
	}
	testNodeData         = numberToHash(123456789).Bytes()
	testHeaderBytes, _   = rlp.EncodeToBytes(testHeader)
	testBlockBytes, _    = rlp.EncodeToBytes(testBlock)
	testReceiptBytes, _  = rlp.EncodeToBytes(testReceipt)
	testNodeDataBytes, _ = rlp.EncodeToBytes(testNodeData)

	testBlockResponse = syncpb.MakeGetBlocksByNumResponse(0, [][]byte{testBlockBytes}, make([][]byte, 1))

	testCurBlockNumber      uint64 = 100
	testBlockNumberResponse        = syncpb.MakeGetBlockNumberResponse(0, testCurBlockNumber)

	testHash                = numberToHash(100)
	testBlockHashesResponse = syncpb.MakeGetBlockHashesResponse(0, []common.Hash{testHash})

	testBlocksByHashesResponse = syncpb.MakeGetBlocksByHashesResponse(0, [][]byte{testBlockBytes}, make([][]byte, 1))

	testReceipsMap = map[uint64]*message.Receipts{
		0: {ReceiptBytes: [][]byte{testReceiptBytes}},
	}
	testReceiptResponse = syncpb.MakeGetReceiptsResponse(0, testReceipsMap)

	testNodeDataResponse = syncpb.MakeGetNodeDataResponse(0, [][]byte{testNodeDataBytes})

	testErrorResponse = syncpb.MakeErrorResponse(0, errors.New("test error"))
)

func TestProtocol_GetBlocksByNumber(t *testing.T) {
	tests := []struct {
		getResponse getResponseFn
		expErr      error
		expStID     sttypes.StreamID
	}{
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockResponse,
				}, makeTestStreamID(0)
			},
			expErr:  nil,
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockNumberResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("not GetBlockByNumber"),
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: nil,
			expErr:      errors.New("get response error"),
			expStID:     "",
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testErrorResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("test error"),
			expStID: makeTestStreamID(0),
		},
	}

	for i, test := range tests {
		protocol := makeTestProtocol(test.getResponse)
		blocks, stid, err := protocol.GetBlocksByNumber(context.Background(), []uint64{0})

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if stid != test.expStID {
			t.Errorf("Test %v: unexpected st id: %v / %v", i, stid, test.expStID)
		}
		if test.expErr == nil && (len(blocks) == 0) {
			t.Errorf("Test %v: zero blocks delivered", i)
		}
	}
}

func TestProtocol_GetCurrentBlockNumber(t *testing.T) {
	tests := []struct {
		getResponse getResponseFn
		expErr      error
		expStID     sttypes.StreamID
	}{
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockNumberResponse,
				}, makeTestStreamID(0)
			},
			expErr:  nil,
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("not GetBlockNumber"),
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: nil,
			expErr:      errors.New("get response error"),
			expStID:     "",
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testErrorResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("test error"),
			expStID: makeTestStreamID(0),
		},
	}

	for i, test := range tests {
		protocol := makeTestProtocol(test.getResponse)
		res, stid, err := protocol.GetCurrentBlockNumber(context.Background())

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if stid != test.expStID {
			t.Errorf("Test %v: unexpected st id: %v / %v", i, stid, test.expStID)
		}
		if test.expErr == nil {
			if res != testCurBlockNumber {
				t.Errorf("Test %v: block number not expected: %v / %v", i, res, testCurBlockNumber)
			}
		}
	}
}

func TestProtocol_GetBlockHashes(t *testing.T) {
	tests := []struct {
		getResponse getResponseFn
		expErr      error
		expStID     sttypes.StreamID
	}{
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockHashesResponse,
				}, makeTestStreamID(0)
			},
			expErr:  nil,
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("not GetBlockHashes"),
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: nil,
			expErr:      errors.New("get response error"),
			expStID:     "",
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testErrorResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("test error"),
			expStID: makeTestStreamID(0),
		},
	}

	for i, test := range tests {
		protocol := makeTestProtocol(test.getResponse)
		res, stid, err := protocol.GetBlockHashes(context.Background(), []uint64{100})

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if stid != test.expStID {
			t.Errorf("Test %v: unexpected st id: %v / %v", i, stid, test.expStID)
		}
		if test.expErr == nil {
			if len(res) != 1 {
				t.Errorf("Test %v: size not 1", i)
			}
			if res[0] != testHash {
				t.Errorf("Test %v: hash not expected", i)
			}
		}
	}
}

func TestProtocol_GetBlocksByHashes(t *testing.T) {
	tests := []struct {
		getResponse getResponseFn
		expErr      error
		expStID     sttypes.StreamID
	}{
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlocksByHashesResponse,
				}, makeTestStreamID(0)
			},
			expErr:  nil,
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("not GetBlocksByHashes"),
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: nil,
			expErr:      errors.New("get response error"),
			expStID:     "",
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testErrorResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("test error"),
			expStID: makeTestStreamID(0),
		},
	}

	for i, test := range tests {
		protocol := makeTestProtocol(test.getResponse)
		blocks, stid, err := protocol.GetBlocksByHashes(context.Background(), []common.Hash{numberToHash(100)})

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if stid != test.expStID {
			t.Errorf("Test %v: unexpected st id: %v / %v", i, stid, test.expStID)
		}
		if test.expErr == nil {
			if len(blocks) != 1 {
				t.Errorf("Test %v: size not 1", i)
			}
		}
	}
}

func TestProtocol_GetReceipts(t *testing.T) {
	tests := []struct {
		getResponse getResponseFn
		expErr      error
		expStID     sttypes.StreamID
	}{
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testReceiptResponse,
				}, makeTestStreamID(0)
			},
			expErr:  nil,
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("response not GetReceipts"),
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: nil,
			expErr:      errors.New("get response error"),
			expStID:     "",
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testErrorResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("test error"),
			expStID: makeTestStreamID(0),
		},
	}

	for i, test := range tests {
		protocol := makeTestProtocol(test.getResponse)
		receipts, stid, err := protocol.GetReceipts(context.Background(), []common.Hash{numberToHash(100)})

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if stid != test.expStID {
			t.Errorf("Test %v: unexpected st id: %v / %v", i, stid, test.expStID)
		}
		if test.expErr == nil {
			if len(receipts) != 1 {
				t.Errorf("Test %v: size not 1", i)
			}
			if len(receipts[0]) != 1 {
				t.Errorf("Test %v: block receipts size not 1", i)
			}
		}
	}
}

func TestProtocol_GetNodeData(t *testing.T) {
	tests := []struct {
		getResponse getResponseFn
		expErr      error
		expStID     sttypes.StreamID
	}{
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testNodeDataResponse,
				}, makeTestStreamID(0)
			},
			expErr:  nil,
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testBlockResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("response not GetNodeData"),
			expStID: makeTestStreamID(0),
		},
		{
			getResponse: nil,
			expErr:      errors.New("get response error"),
			expStID:     "",
		},
		{
			getResponse: func(request sttypes.Request) (sttypes.Response, sttypes.StreamID) {
				return &syncResponse{
					pb: testErrorResponse,
				}, makeTestStreamID(0)
			},
			expErr:  errors.New("test error"),
			expStID: makeTestStreamID(0),
		},
	}

	for i, test := range tests {
		protocol := makeTestProtocol(test.getResponse)
		receipts, stid, err := protocol.GetNodeData(context.Background(), []common.Hash{numberToHash(100)})

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if stid != test.expStID {
			t.Errorf("Test %v: unexpected st id: %v / %v", i, stid, test.expStID)
		}
		if test.expErr == nil {
			if len(receipts) != 1 {
				t.Errorf("Test %v: size not 1", i)
			}
		}
	}
}

type getResponseFn func(request sttypes.Request) (sttypes.Response, sttypes.StreamID)

type testHostRequestManager struct {
	getResponse getResponseFn
}

func makeTestProtocol(f getResponseFn) *Protocol {
	rm := &testHostRequestManager{f}

	streamIDs := make([]sttypes.StreamID, len(initStreamIDs))
	copy(streamIDs, initStreamIDs)
	sm := &testStreamManager{streamIDs}

	rl := ratelimiter.NewRateLimiter(sm, 10, 10)

	return &Protocol{
		rm: rm,
		rl: rl,
		sm: sm,
	}
}

func (rm *testHostRequestManager) Start()                                             {}
func (rm *testHostRequestManager) Close()                                             {}
func (rm *testHostRequestManager) DeliverResponse(sttypes.StreamID, sttypes.Response) {}

func (rm *testHostRequestManager) DoRequest(ctx context.Context, request sttypes.Request, opts ...Option) (sttypes.Response, sttypes.StreamID, error) {
	if rm.getResponse == nil {
		return nil, "", errors.New("get response error")
	}
	resp, stid := rm.getResponse(request)
	return resp, stid, nil
}

func makeTestStreamID(index int) sttypes.StreamID {
	id := fmt.Sprintf("[test stream %v]", index)
	return sttypes.StreamID(id)
}

// mock stream manager
type testStreamManager struct {
	streamIDs []sttypes.StreamID
}

func (sm *testStreamManager) Start() {}
func (sm *testStreamManager) Close() {}
func (sm *testStreamManager) SubscribeAddStreamEvent(chan<- streammanager.EvtStreamAdded) event.Subscription {
	return nil
}
func (sm *testStreamManager) SubscribeRemoveStreamEvent(chan<- streammanager.EvtStreamRemoved) event.Subscription {
	return nil
}

func (sm *testStreamManager) NewStream(stream sttypes.Stream) error {
	stid := stream.ID()
	for _, id := range sm.streamIDs {
		if id == stid {
			return errors.New("stream already exist")
		}
	}
	sm.streamIDs = append(sm.streamIDs, stid)
	return nil
}

func (sm *testStreamManager) RemoveStream(stID sttypes.StreamID) error {
	for i, id := range sm.streamIDs {
		if id == stID {
			sm.streamIDs = append(sm.streamIDs[:i], sm.streamIDs[i+1:]...)
		}
	}
	return errors.New("stream not exist")
}

func (sm *testStreamManager) isStreamExist(stid sttypes.StreamID) bool {
	for _, id := range sm.streamIDs {
		if id == stid {
			return true
		}
	}
	return false
}

func (sm *testStreamManager) GetStreams() []sttypes.Stream {
	return nil
}

func (sm *testStreamManager) GetStreamByID(id sttypes.StreamID) (sttypes.Stream, bool) {
	return nil, false
}

func assertError(got, expect error) error {
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error: %v / %v", got, expect)
	}
	if got == nil {
		return nil
	}
	if !strings.Contains(got.Error(), expect.Error()) {
		return fmt.Errorf("unexpected error: %v/ %v", got, expect)
	}
	return nil
}
