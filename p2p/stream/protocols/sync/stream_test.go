package sync

import (
	"io"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	protobuf "github.com/golang/protobuf/proto"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var _ sttypes.Protocol = &Protocol{}

var (
	testGetBlockNumbers    = []uint64{1, 2, 3, 4, 5}
	testGetBlockRequest    = syncpb.MakeGetBlocksByNumRequest(testGetBlockNumbers)
	testGetBlockRequestMsg = syncpb.MakeMessageFromRequest(testGetBlockRequest)

	testEpoch                uint64 = 20
	testEpochStateRequest           = syncpb.MakeGetEpochStateRequest(testEpoch)
	testEpochStateRequestMsg        = syncpb.MakeMessageFromRequest(testEpochStateRequest)

	testCurrentNumberRequest    = syncpb.MakeGetBlockNumberRequest()
	testCurrentNumberRequestMsg = syncpb.MakeMessageFromRequest(testCurrentNumberRequest)

	testGetBlockHashNums         = []uint64{1, 2, 3, 4, 5}
	testGetBlockHashesRequest    = syncpb.MakeGetBlockHashesRequest(testGetBlockHashNums)
	testGetBlockHashesRequestMsg = syncpb.MakeMessageFromRequest(testGetBlockHashesRequest)

	testGetBlockByHashes = []common.Hash{
		numberToHash(1),
		numberToHash(2),
		numberToHash(3),
		numberToHash(4),
		numberToHash(5),
	}
	testGetBlocksByHashesRequest    = syncpb.MakeGetBlocksByHashesRequest(testGetBlockByHashes)
	testGetBlocksByHashesRequestMsg = syncpb.MakeMessageFromRequest(testGetBlocksByHashesRequest)
)

func TestSyncStream_HandleGetBlocksByRequest(t *testing.T) {
	st, inC, outC := makeTestSyncStream()

	st.run()
	defer close(st.closeC)

	req := testGetBlockRequestMsg
	b, _ := protobuf.Marshal(req)
	outC <- b

	var receivedBytes []byte
	select {
	case receivedBytes = <-inC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out")
	}

	if err := checkBlocksResult(testGetBlockNumbers, receivedBytes); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleEpochStateRequest(t *testing.T) {
	st, inC, outC := makeTestSyncStream()
	st.run()
	defer close(st.closeC)

	req := testEpochStateRequestMsg
	b, _ := protobuf.Marshal(req)
	outC <- b

	var receivedBytes []byte
	select {
	case receivedBytes = <-inC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out")
	}

	if err := checkEpochStateResult(testEpoch, receivedBytes); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleCurrentBlockNumber(t *testing.T) {
	st, inC, outC := makeTestSyncStream()
	st.run()
	defer close(st.closeC)

	req := testCurrentNumberRequestMsg
	b, _ := protobuf.Marshal(req)
	outC <- b

	var receivedBytes []byte
	select {
	case receivedBytes = <-inC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out")
	}

	if err := checkBlockNumberResult(receivedBytes); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleGetBlockHashes(t *testing.T) {
	st, inC, outC := makeTestSyncStream()
	st.run()
	defer close(st.closeC)

	req := testGetBlockHashesRequestMsg
	b, _ := protobuf.Marshal(req)
	outC <- b

	var receivedBytes []byte
	select {
	case receivedBytes = <-inC:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out")
	}

	if err := checkBlockHashesResult(receivedBytes, testGetBlockNumbers); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleGetBlocksByHashes(t *testing.T) {
	st, inC, outC := makeTestSyncStream()
	st.run()
	defer close(st.closeC)

	req := testGetBlocksByHashesRequestMsg
	b, _ := protobuf.Marshal(req)
	outC <- b

	var receivedBytes []byte
	select {
	case receivedBytes = <-inC:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out")
	}

	if err := checkBlocksByHashesResult(receivedBytes, testGetBlockByHashes); err != nil {
		t.Fatal(err)
	}
}

func makeTestSyncStream() (st *syncStream, inC, outC chan []byte) {
	inC = make(chan []byte, 1)
	outC = make(chan []byte, 1)

	bs := sttypes.NewBaseStream(newTestP2PStream(inC, outC))

	return &syncStream{
		BaseStream: bs,
		chain:      &testChainHelper{},
		protocol:   makeTestProtocol(nil),
		reqC:       make(chan *syncpb.Request, 100),
		respC:      make(chan *syncpb.Response, 100),
		closeC:     make(chan struct{}),
		closeStat:  0,
	}, inC, outC
}

type testP2PStream struct {
	inMsgC  chan []byte // message sent to remote stream
	outMsgC chan []byte // message received from remote stream
}

func newTestP2PStream(inC, outC chan []byte) *testP2PStream {
	return &testP2PStream{
		inMsgC:  inC,
		outMsgC: outC,
	}
}

func (st *testP2PStream) Read(p []byte) (n int, err error) {
	b := <-st.outMsgC
	copy(p, b)
	return len(b), io.EOF
}

func (st *testP2PStream) Write(p []byte) (n int, err error) {
	st.inMsgC <- p
	return len(p), nil
}

func (st *testP2PStream) Close() error                     { return nil }
func (st *testP2PStream) CloseRead() error                 { return nil }
func (st *testP2PStream) CloseWrite() error                { return nil }
func (st *testP2PStream) Reset() error                     { return nil }
func (st *testP2PStream) SetDeadline(time.Time) error      { return nil }
func (st *testP2PStream) SetReadDeadline(time.Time) error  { return nil }
func (st *testP2PStream) SetWriteDeadline(time.Time) error { return nil }
func (st *testP2PStream) ID() string                       { return "" }
func (st *testP2PStream) Protocol() protocol.ID            { return "" }
func (st *testP2PStream) SetProtocol(protocol.ID)          {}
func (st *testP2PStream) Stat() libp2p_network.Stat        { return libp2p_network.Stat{} }
func (st *testP2PStream) Conn() libp2p_network.Conn        { return nil }
