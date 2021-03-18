package sttypes

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	libp2p_network "github.com/libp2p/go-libp2p-core/network"
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
	ResetOnClose() error
}

// BaseStream is the wrapper around
type BaseStream struct {
	raw libp2p_network.Stream
	rw  *bufio.ReadWriter

	// parse protocol spec fields
	spec     ProtoSpec
	specErr  error
	specOnce sync.Once
}

// NewBaseStream creates BaseStream as the wrapper of libp2p Stream
func NewBaseStream(st libp2p_network.Stream) *BaseStream {
	rw := bufio.NewReadWriter(bufio.NewReader(st), bufio.NewWriter(st))
	return &BaseStream{
		raw: st,
		rw:  rw,
	}
}

// StreamID is the unique identifier for the stream. It has the value of
// libp2p_network.Stream.ID()
type StreamID string

// Meta return the StreamID of the stream
func (st *BaseStream) ID() StreamID {
	return StreamID(st.raw.Conn().ID())
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

// Close close the stream on both sides.
func (st *BaseStream) Close() error {
	return st.raw.Close()
}

const (
	maxMsgBytes = 20 * 1024 * 1024 // 20MB
	sizeBytes   = 4                // uint32
)

// WriteBytes write the bytes to the stream.
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
	if _, err = st.rw.Write(intToBytes(len(b))); err != nil {
		return errors.Wrap(err, "write size bytes")
	}
	bytesWriteCounter.Add(sizeBytes)
	if _, err = st.rw.Write(b); err != nil {
		return errors.Wrap(err, "write content")
	}
	bytesWriteCounter.Add(float64(len(b)))
	return st.rw.Flush()
}

// ReadMsg read the bytes from the stream
func (st *BaseStream) ReadBytes() (b []byte, err error) {
	defer func() {
		msgReadCounter.Inc()
		if err != nil {
			msgReadFailedCounterVec.With(prometheus.Labels{"error": err.Error()}).Inc()
		}
	}()

	sb := make([]byte, sizeBytes)
	_, err = st.rw.Read(sb)
	if err != nil {
		return nil, errors.Wrap(err, "read size")
	}
	bytesReadCounter.Add(sizeBytes)
	size := bytesToInt(sb)
	if size > maxMsgBytes {
		return nil, fmt.Errorf("message size exceed max: %v > %v", size, maxMsgBytes)
	}

	cb := make([]byte, size)
	n, err := io.ReadFull(st.rw, cb)
	if err != nil {
		return nil, errors.Wrap(err, "read content")
	}
	bytesReadCounter.Add(float64(n))
	if n != size {
		return nil, errors.New("ReadBytes sanity failed: byte size")
	}
	return cb, nil
}

// ResetOnClose reset the stream during the shutdown of the node
func (st *BaseStream) ResetOnClose() error {
	return st.raw.Reset()
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
