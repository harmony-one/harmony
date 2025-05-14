package sttypes

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	maxMsgBytes        = 20 * 1024 * 1024 // 20MB
	sizeBytes          = 4                // uint32
	streamReadTimeout  = 60 * time.Second
	streamWriteTimeout = 60 * time.Second
	withDeadlines      = false // set stream deadlines
)

// Stream is the interface for streams implemented in each service.
// The stream interface is used for stream management as well as rate limiters
type Stream interface {
	ID() StreamID
	ProtoID() ProtoID
	ProtoSpec() (ProtoSpec, error)
	WriteBytes([]byte) error
	ReadBytes() ([]byte, error)
	Close(reason string) error
	CloseOnExit() error
	Failures() int32
	AddFailedTimes(faultRecoveryThreshold time.Duration)
	ResetFailedTimes()
}

// BaseStream is the wrapper around
type BaseStream struct {
	raw    libp2p_network.Stream
	reader *bufio.Reader
	lock   sync.Mutex

	readTimeout  time.Duration
	writeTimeout time.Duration

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
	return &BaseStream{
		raw:             st,
		reader:          bufio.NewReader(st),
		readTimeout:     streamReadTimeout,
		writeTimeout:    streamWriteTimeout,
		failures:        0,
		lastFailureTime: time.Now(),
	}
}

func (st *BaseStream) setReadDeadline() error {
	return st.raw.SetReadDeadline(time.Now().Add(st.readTimeout))
}

func (st *BaseStream) setWriteDeadline() error {
	return st.raw.SetWriteDeadline(time.Now().Add(st.writeTimeout))
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
	st.lock.Lock()
	defer st.lock.Unlock()

	// Clean up resources
	if st.reader != nil {
		st.reader.Reset(nil) // Clear buffer
	}

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

func (st *BaseStream) IsHealthy() bool {
	st.failureLock.Lock()
	defer st.failureLock.Unlock()

	// Too many failures recently
	if st.failures > 3 && time.Since(st.lastFailureTime) < 5*time.Minute {
		return false
	}

	// Check if underlying connection is still good
	if st.raw.Conn().IsClosed() {
		return false
	}

	return true
}

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

	st.lock.Lock()
	defer st.lock.Unlock()

	// Adjust write timeout
	if withDeadlines {
		if err := st.setWriteDeadline(); err != nil {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Err(err).
				Msg("failed to adjust write deadline")
			return err
		}
	} else {
		// Disable write timeout
		if err := st.raw.SetWriteDeadline(time.Time{}); err != nil {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Err(err).
				Msg("failed to disable write deadline")
			return err
		}
	}

	_, err = st.raw.Write(message[:size])
	if err != nil {
		return err
	}
	bytesWriteCounter.Add(float64(size))
	return nil
}

// ReadBytes reads bytes from the stream with blocking behavior.
// It will wait indefinitely for data unless:
// - The stream is explicitly closed
// - A network error occurs
// - The message size exceeds maxMsgBytes
func (st *BaseStream) ReadBytes() (content []byte, err error) {
	defer func() {
		msgReadCounter.Inc()
		if err != nil {
			msgReadFailedCounterVec.With(prometheus.Labels{"error": err.Error()}).Inc()
		}
	}()

	// Adjust read timeout
	if withDeadlines {
		if err := st.setReadDeadline(); err != nil {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Err(err).
				Msg("failed to adjust read deadline")
			return nil, errors.Wrap(err, "failed to adjust read deadline")
		}
	} else {
		// Disable read timeout for true blocking behavior
		if err := st.raw.SetReadDeadline(time.Time{}); err != nil {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Err(err).
				Msg("failed to disable read deadline")
			return nil, errors.Wrap(err, "failed to disable read deadline")
		}
	}

	// 1. Read message length prefix (blocking)
	lengthBuf := make([]byte, sizeBytes)
	n, err := io.ReadFull(st.reader, lengthBuf)
	if err != nil {
		if err == io.EOF {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Msg("clean stream closure")
			return nil, nil
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil
		}
		if err == nil && n == 0 {
			return nil, nil
		}
		utils.Logger().Debug().
			Str("streamID", string(st.ID())).
			Err(err).
			Msg("failed reading length prefix")
		return nil, errors.Wrap(err, "length prefix read failed")
	}
	bytesReadCounter.Add(sizeBytes)

	// 2. Process length
	size := bytesToInt(lengthBuf)
	if size > maxMsgBytes {
		utils.Logger().Warn().
			Str("streamID", string(st.ID())).
			Int("size", size).
			Int("max", maxMsgBytes).
			Msg("message size exceeds limit")
		return nil, errors.Errorf("message size %d exceeds max %d", size, maxMsgBytes)
	}

	// 3. Read message content (blocking)
	content = make([]byte, size)
	n, err = io.ReadFull(st.reader, content)
	if err != nil {
		utils.Logger().Debug().
			Str("streamID", string(st.ID())).
			Err(err).
			Int("expected", size).
			Msg("failed reading message content")
		return nil, errors.Wrap(err, "content read failed")
	}
	bytesReadCounter.Add(float64(n))

	if n != size {
		utils.Logger().Debug().
			Str("streamID", string(st.ID())).
			Int("read", n).
			Int("expected", size).
			Msg("incomplete message read")
		return nil, errors.Errorf("read %d bytes but expected %d", n, size)
	}

	return content, nil
}

// CloseOnExit resets the stream during the shutdown of the node
func (st *BaseStream) CloseOnExit() error {
	err := st.raw.Close()
	if err != nil {
		return st.raw.Reset()
	}
	return nil
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
