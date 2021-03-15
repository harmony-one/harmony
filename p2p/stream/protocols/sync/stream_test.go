package sync

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

var _ sttypes.Protocol = &Protocol{}

var (
	testGetBlockNumbers    = []uint64{1, 2, 3, 4, 5}
	testGetBlockRequest    = syncpb.MakeGetBlocksByNumRequest(testGetBlockNumbers)
	testGetBlockRequestMsg = syncpb.MakeMessageFromRequest(testGetBlockRequest)

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
	st, remoteSt := makeTestSyncStream()

	go st.run()
	defer close(st.closeC)

	req := testGetBlockRequestMsg
	b, _ := protobuf.Marshal(req)
	err := remoteSt.WriteBytes(b)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	receivedBytes, _ := remoteSt.ReadBytes()

	if err := checkBlocksResult(testGetBlockNumbers, receivedBytes); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleCurrentBlockNumber(t *testing.T) {
	st, remoteSt := makeTestSyncStream()

	go st.run()
	defer close(st.closeC)

	req := testCurrentNumberRequestMsg
	b, _ := protobuf.Marshal(req)
	err := remoteSt.WriteBytes(b)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	receivedBytes, _ := remoteSt.ReadBytes()

	if err := checkBlockNumberResult(receivedBytes); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleGetBlockHashes(t *testing.T) {
	st, remoteSt := makeTestSyncStream()

	go st.run()
	defer close(st.closeC)

	req := testGetBlockHashesRequestMsg
	b, _ := protobuf.Marshal(req)
	err := remoteSt.WriteBytes(b)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	receivedBytes, _ := remoteSt.ReadBytes()

	if err := checkBlockHashesResult(receivedBytes, testGetBlockNumbers); err != nil {
		t.Fatal(err)
	}
}

func TestSyncStream_HandleGetBlocksByHashes(t *testing.T) {
	st, remoteSt := makeTestSyncStream()

	go st.run()
	defer close(st.closeC)

	req := testGetBlocksByHashesRequestMsg
	b, _ := protobuf.Marshal(req)
	err := remoteSt.WriteBytes(b)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	receivedBytes, _ := remoteSt.ReadBytes()

	if err := checkBlocksByHashesResult(receivedBytes, testGetBlockByHashes); err != nil {
		t.Fatal(err)
	}
}

func makeTestSyncStream() (*syncStream, *testRemoteBaseStream) {
	localRaw, remoteRaw := makePairP2PStreams()
	remote := newTestRemoteBaseStream(remoteRaw)

	bs := sttypes.NewBaseStream(localRaw)

	return &syncStream{
		BaseStream: bs,
		chain:      &testChainHelper{},
		protocol:   makeTestProtocol(nil),
		reqC:       make(chan *syncpb.Request, 100),
		respC:      make(chan *syncpb.Response, 100),
		closeC:     make(chan struct{}),
		closeStat:  0,
	}, remote
}

type testP2PStream struct {
	readBuf  *bytes.Buffer
	inC      chan struct{}
	identity string

	writeHook func([]byte) (int, error)
}

func makePairP2PStreams() (*testP2PStream, *testP2PStream) {
	buf1 := bytes.NewBuffer(nil)
	buf2 := bytes.NewBuffer(nil)

	st1 := &testP2PStream{
		readBuf:  buf1,
		inC:      make(chan struct{}, 1),
		identity: "local",
	}
	st2 := &testP2PStream{
		readBuf:  buf2,
		inC:      make(chan struct{}, 1),
		identity: "remote",
	}
	st1.writeHook = st2.receiveBytes
	st2.writeHook = st1.receiveBytes
	return st1, st2
}

func (st *testP2PStream) Read(b []byte) (n int, err error) {
	<-st.inC
	n, err = st.readBuf.Read(b)
	if st.readBuf.Len() != 0 {
		select {
		case st.inC <- struct{}{}:
		default:
		}
	}
	return
}

func (st *testP2PStream) Write(b []byte) (n int, err error) {
	return st.writeHook(b)
}

func (st *testP2PStream) receiveBytes(b []byte) (n int, err error) {
	n, err = st.readBuf.Write(b)
	select {
	case st.inC <- struct{}{}:
	default:
	}
	return
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
func (st *testP2PStream) Conn() libp2p_network.Conn        { return &fakeConn{} }

type testRemoteBaseStream struct {
	base *sttypes.BaseStream
}

func newTestRemoteBaseStream(st *testP2PStream) *testRemoteBaseStream {
	rst := &testRemoteBaseStream{
		base: sttypes.NewBaseStream(st),
	}
	return rst
}

func (st *testRemoteBaseStream) ReadBytes() ([]byte, error) {
	return st.base.ReadBytes()
}

func (st *testRemoteBaseStream) WriteBytes(b []byte) error {
	return st.base.WriteBytes(b)
}

type fakeConn struct{}

func (conn *fakeConn) Close() error                                             { return nil }
func (conn *fakeConn) LocalPeer() peer.ID                                       { return "" }
func (conn *fakeConn) LocalPrivateKey() ic.PrivKey                              { return nil }
func (conn *fakeConn) RemotePeer() peer.ID                                      { return "" }
func (conn *fakeConn) RemotePublicKey() ic.PubKey                               { return nil }
func (conn *fakeConn) LocalMultiaddr() ma.Multiaddr                             { return nil }
func (conn *fakeConn) RemoteMultiaddr() ma.Multiaddr                            { return nil }
func (conn *fakeConn) ID() string                                               { return "" }
func (conn *fakeConn) NewStream(context.Context) (libp2p_network.Stream, error) { return nil, nil }
func (conn *fakeConn) GetStreams() []libp2p_network.Stream                      { return nil }
func (conn *fakeConn) Stat() libp2p_network.Stat                                { return libp2p_network.Stat{} }
