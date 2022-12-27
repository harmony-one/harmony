package streammanager

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/libp2p/go-libp2p/core/network"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var _ StreamManager = &streamManager{}

var (
	myPeerID    = makePeerID(0)
	testProtoID = sttypes.ProtoID("harmony/sync/unitest/0/1.0.0/1")
)

const (
	defHardLoCap = 16  // discovery trigger immediately when size smaller than this number
	defSoftLoCap = 32  // discovery trigger for routine check
	defHiCap     = 128 // Hard cap of the stream number
	defDiscBatch = 16  // batch size for discovery
)

var defConfig = Config{
	HardLoCap: defHardLoCap,
	SoftLoCap: defSoftLoCap,
	HiCap:     defHiCap,
	DiscBatch: defDiscBatch,
}

func newTestStreamManager() *streamManager {
	pid := testProtoID
	host := newTestHost()
	pf := newTestPeerFinder(makeRemotePeers(100), emptyDelayFunc)

	sm := newStreamManager(pid, host, pf, nil, defConfig)
	host.sm = sm
	return sm
}

type testStream struct {
	id     sttypes.StreamID
	proto  sttypes.ProtoID
	closed bool
}

func newTestStream(id sttypes.StreamID, proto sttypes.ProtoID) *testStream {
	return &testStream{id: id, proto: proto}
}

func (st *testStream) ID() sttypes.StreamID {
	return st.id
}

func (st *testStream) ProtoID() sttypes.ProtoID {
	return st.proto
}

func (st *testStream) WriteBytes([]byte) error {
	return nil
}

func (st *testStream) ReadBytes() ([]byte, error) {
	return nil, nil
}

func (st *testStream) FailedTimes() int {
	return 0
}

func (st *testStream) AddFailedTimes() {
	return
}

func (st *testStream) ResetFailedTimes() {
	return
}

func (st *testStream) Close() error {
	if st.closed {
		return errors.New("already closed")
	}
	st.closed = true
	return nil
}

func (st *testStream) CloseOnExit() error {
	if st.closed {
		return errors.New("already closed")
	}
	st.closed = true
	return nil
}

func (st *testStream) ProtoSpec() (sttypes.ProtoSpec, error) {
	return sttypes.ProtoIDToProtoSpec(st.ProtoID())
}

type testHost struct {
	sm      *streamManager
	streams map[sttypes.StreamID]*testStream
	lock    sync.Mutex

	errHook streamErrorHook
}

type streamErrorHook func(id sttypes.StreamID, err error)

func newTestHost() *testHost {
	return &testHost{
		streams: make(map[sttypes.StreamID]*testStream),
	}
}

func (h *testHost) ID() libp2p_peer.ID {
	return myPeerID
}

// NewStream mock the upper function logic. When stream setup and running protocol, the
// upper code logic will call StreamManager to add new stream
func (h *testHost) NewStream(ctx context.Context, p libp2p_peer.ID, pids ...protocol.ID) (network.Stream, error) {
	if len(pids) == 0 {
		return nil, errors.New("nil protocol ids")
	}
	var err error
	stid := sttypes.StreamID(p)
	defer func() {
		if err != nil && h.errHook != nil {
			h.errHook(stid, err)
		}
	}()

	st := newTestStream(stid, sttypes.ProtoID(pids[0]))
	h.lock.Lock()
	h.streams[stid] = st
	h.lock.Unlock()

	err = h.sm.NewStream(st)
	return nil, err
}

func makeStreamID(index int) sttypes.StreamID {
	return sttypes.StreamID(strconv.Itoa(index))
}

func makePeerID(index int) libp2p_peer.ID {
	return libp2p_peer.ID(strconv.Itoa(index))
}

func makeRemotePeers(size int) []libp2p_peer.ID {
	ids := make([]libp2p_peer.ID, 0, size)
	for i := 1; i != size+1; i++ {
		ids = append(ids, makePeerID(i))
	}
	return ids
}

type testPeerFinder struct {
	peerIDs  []libp2p_peer.ID
	curIndex int32
	fpHook   delayFunc
}

type delayFunc func(id libp2p_peer.ID) <-chan struct{}

func emptyDelayFunc(id libp2p_peer.ID) <-chan struct{} {
	c := make(chan struct{})
	go func() {
		c <- struct{}{}
	}()
	return c
}

func newTestPeerFinder(ids []libp2p_peer.ID, fpHook delayFunc) *testPeerFinder {
	return &testPeerFinder{
		peerIDs:  ids,
		curIndex: 0,
		fpHook:   fpHook,
	}
}

func (pf *testPeerFinder) FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error) {
	if peerLimit > len(pf.peerIDs) {
		peerLimit = len(pf.peerIDs)
	}
	resC := make(chan libp2p_peer.AddrInfo)

	go func() {
		defer close(resC)

		for i := 0; i != peerLimit; i++ {
			// hack to prevent race
			curIndex := atomic.LoadInt32(&pf.curIndex)
			pid := pf.peerIDs[curIndex]
			select {
			case <-ctx.Done():
				return
			case <-pf.fpHook(pid):
			}
			resC <- libp2p_peer.AddrInfo{ID: pid}
			atomic.AddInt32(&pf.curIndex, 1)
			if int(atomic.LoadInt32(&pf.curIndex)) == len(pf.peerIDs) {
				pf.curIndex = 0
			}
		}
	}()

	return resC, nil
}
