package sttypes

import (
	"bufio"
	"encoding/binary"
	"io"
	"sync"
	"time"

	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// Stream is the interface for streams implemented in each service.
// The stream interface is used for stream management as well as rate limiters
type Stream interface {
	ID() StreamID
	ProtoID() ProtoID
	ProtoSpec() (ProtoSpec, error)
	WriteBytes([]byte) error
	ReadBytes() ([]byte, error)
	Close() error
	CloseOnExit() error
	Failures() int
	AddFailedTimes(faultRecoveryThreshold time.Duration)
	ResetFailedTimes()
}

// BaseStream is the wrapper around
type BaseStream struct {
	raw       libp2p_network.Stream
	reader    *bufio.Reader
	readLock  sync.Mutex
	writeLock sync.Mutex

	// parse protocol spec fields
	spec     ProtoSpec
	specErr  error
	specOnce sync.Once

	failures        int32
	lastFailureTime time.Time
	failureLock     sync.Mutex
}

// NewBaseStream creates BaseStream as the wrapper of libp2p Stream
func NewBaseStream(st libp2p_network.Stream) *BaseStream {
	reader := bufio.NewReader(st)
	return &BaseStream{
		raw:             st,
		reader:          reader,
		failures:        0,
		lastFailureTime: time.Now(),
	}
}

// StreamID is the unique identifier for the stream. It has the value of
// libp2p_network_peer.ID
type StreamID string

// ID return the StreamID of the stream
func (st *BaseStream) ID() StreamID {
	return StreamID(st.raw.Conn().RemotePeer().String())
}

// ProtoID return the remote protocol ID of the stream
func (st *BaseStream) ProtoID() ProtoID {
	return ProtoID(st.raw.Protocol())
}

// ProtoSpec get the parsed protocol Specifier of the stream
func (st *BaseStream) ProtoSpec() (ProtoSpec, error) {
	st.specOnce.Do(func() {
		st.spec, st.specErr = ProtoIDToProtoSpec(st.ProtoID())
	})
	return st.spec, st.specErr
}

// Close reset the stream, and close the connection for both sides.
func (st *BaseStream) Close() error {
	err := st.raw.Close()
	if err != nil {
		return st.raw.Reset()
	}
	return nil
}

func (st *BaseStream) Failures() int32 {
	st.failureLock.Lock()
	defer st.failureLock.Unlock()
	return st.failures
}

func (st *BaseStream) AddFailedTimes(faultRecoveryThreshold time.Duration) {
	st.failureLock.Lock()
	defer st.failureLock.Unlock()
	st.failures += 1
	st.lastFailureTime = time.Now()
}

func (st *BaseStream) ResetFailedTimes() {
	st.failureLock.Lock()
	defer st.failureLock.Unlock()
	st.failures = 0
}

const (
	maxMsgBytes = 20 * 1024 * 1024 // 20MB
	sizeBytes   = 4                // uint32
)

// WriteBytes writes the bytes to the stream.
// First 4 bytes is used as the size bytes, and the rest is the content
func (st *BaseStream) WriteBytes(b []byte) (err error) {
	defer func() {
		msgWriteCounter.Inc()
		if err != nil {
			msgWriteFailedCounterVec.With(prometheus.Labels{"error": err.Error()}).Inc()
		}
	}()

	if len(b) > maxMsgBytes {
		return errors.New("message too long")
	}

	size := sizeBytes + len(b)
	message := make([]byte, size)
	copy(message, intToBytes(len(b)))
	copy(message[sizeBytes:], b)

	st.writeLock.Lock()
	defer st.writeLock.Unlock()
	_, err = st.raw.Write(message[:size])
	if err != nil {
		st.raw.Reset()
		return err
	}
	bytesWriteCounter.Add(float64(size))
	return nil
}

// ReadBytes reads the bytes from the stream.
func (st *BaseStream) ReadBytes() (cb []byte, err error) {
	defer func() {
		msgReadCounter.Inc()
		if err != nil {
			msgReadFailedCounterVec.With(prometheus.Labels{"error": err.Error()}).Inc()
		}
	}()

	st.readLock.Lock()
	defer st.readLock.Unlock()

	sb := make([]byte, sizeBytes)
	_, err = st.reader.Read(sb)
	if err != nil {
		st.raw.Reset()
		err = errors.Wrap(err, "read size")
		return
	}
	bytesReadCounter.Add(sizeBytes)
	size := bytesToInt(sb)
	if size > maxMsgBytes {
		err = errors.New("message size exceed max")
		return nil, err
	}

	cb = make([]byte, size)

	n, err := io.ReadFull(st.reader, cb)
	if err != nil {
		st.raw.Reset()
		err = errors.Wrap(err, "read content")
		return
	}
	bytesReadCounter.Add(float64(n))
	if n != size {
		err = errors.Errorf("read %d bytes but expected %d", n, size)
		return
	}
	return
}

// CloseOnExit resets the stream during the shutdown of the node
func (st *BaseStream) CloseOnExit() error {
	return st.raw.Reset()
}

// reconnect attempts to reset the stream up to 3 times
func (st *BaseStream) reconnect() error {
	for i := 0; i < 3; i++ {
		err := st.raw.Reset()
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return errors.New("failed to reconnect after 3 attempts")
}

func intToBytes(val int) []byte {
	b := make([]byte, sizeBytes) // uint32
	binary.LittleEndian.PutUint32(b, uint32(val))
	return b
}

func bytesToInt(b []byte) int {
	val := binary.LittleEndian.Uint32(b)
	return int(val)
}
