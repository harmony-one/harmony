package sttypes

import (
	"bufio"
	"encoding/binary"
	"io"
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
	IsTrusted() bool
	WriteBytes([]byte) error
	ReadBytes() ([]byte, error)
	Close(reason string, criticalErr bool) error
	CloseOnExit() error
	Failures() int32
	AddFailedTimes(faultRecoveryThreshold time.Duration)
	ResetFailedTimes()
	GetProgressTracker() *ProgressTracker
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

	trusted bool

	failures        int32
	lastFailureTime time.Time
	failureLock     sync.Mutex

	// Progress tracking for timeout management
	progressTracker *ProgressTracker
	timeoutConfig   *StreamTimeoutConfig
}

// NewBaseStream creates BaseStream as the wrapper of libp2p Stream
func NewBaseStream(st libp2p_network.Stream, trusted bool) *BaseStream {
	config := DefaultStreamTimeoutConfig()
	return &BaseStream{
		raw:             st,
		trusted:         trusted,
		reader:          bufio.NewReader(st),
		readTimeout:     streamReadTimeout,
		writeTimeout:    streamWriteTimeout,
		failures:        0,
		lastFailureTime: time.Now(),
		progressTracker: NewProgressTracker(config.ProgressTimeout, config.ProgressThreshold),
		timeoutConfig:   config,
	}
}

// NewBaseStreamWithConfig creates BaseStream with custom timeout configuration
func NewBaseStreamWithConfig(st libp2p_network.Stream, trusted bool, config *StreamTimeoutConfig) *BaseStream {
	if config == nil {
		config = DefaultStreamTimeoutConfig()
	}

	return &BaseStream{
		raw:             st,
		trusted:         trusted,
		reader:          bufio.NewReader(st),
		readTimeout:     streamReadTimeout,
		writeTimeout:    streamWriteTimeout,
		failures:        0,
		lastFailureTime: time.Now(),
		progressTracker: NewProgressTracker(config.ProgressTimeout, config.ProgressThreshold),
		timeoutConfig:   config,
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

func (st *BaseStream) IsTrusted() bool {
	return st.trusted
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

// GetProgressTracker returns the progress tracker for this stream
func (st *BaseStream) GetProgressTracker() *ProgressTracker {
	return st.progressTracker
}

// GetTimeoutConfig returns the timeout configuration for this stream
func (st *BaseStream) GetTimeoutConfig() *StreamTimeoutConfig {
	return st.timeoutConfig
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
		return &MessageError{Err: errors.Wrapf(errors.New("message too long"), "message length %d exceeds max %d", len(b), maxMsgBytes)}
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
		return &StreamWriteError{Err: err}
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
	_, err = io.ReadFull(st.reader, lengthBuf)
	if err != nil {
		// Classify the error for better handling
		errorType, errorDesc := ClassifyStreamError(err)

		switch errorType {
		case ErrorTypeRemoteDisconnect:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "remote_disconnect").
				Msg("stream closed by remote peer")
			return nil, errors.Wrap(err, "stream closed")

		case ErrorTypeConnectionReset:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "connection_reset").
				Str("description", errorDesc).
				Msg("connection reset by peer during length prefix read")
			return nil, errors.Wrap(err, "connection reset")

		case ErrorTypeBrokenPipe:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "broken_pipe").
				Str("description", errorDesc).
				Msg("broken pipe during length prefix read")
			return nil, errors.Wrap(err, "broken pipe")

		case ErrorTypeTimeout:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "timeout").
				Msg("timeout reading length prefix")
			return nil, errors.Wrap(err, "timeout")

		case ErrorTypeResourceExhaustion:
			utils.Logger().Error().
				Str("streamID", string(st.ID())).
				Str("errorType", "resource_exhaustion").
				Str("description", errorDesc).
				Msg("resource exhaustion during length prefix read")
			return nil, errors.Wrap(err, "resource exhaustion")

		case ErrorTypeLocalNetwork:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "local_network").
				Str("description", errorDesc).
				Err(err).
				Msg("local network error reading length prefix")
			return nil, errors.Wrap(err, "network error")

		default:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "unknown").
				Str("description", errorDesc).
				Err(err).
				Msg("failed reading length prefix")
			return nil, errors.Wrap(err, "length prefix read failed")
		}
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
		return nil, errors.Wrapf(errors.New("message size exceeds limit"), "message size %d exceeds max %d", size, maxMsgBytes)
	}

	// 3. Read message content (blocking)
	content = make([]byte, size)
	bytesRead, err := io.ReadFull(st.reader, content)
	if err != nil {
		// Classify the error for better handling
		errorType, errorDesc := ClassifyStreamError(err)

		switch errorType {
		case ErrorTypeRemoteDisconnect:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "remote_disconnect").
				Int("expected", size).
				Msg("stream closed by remote peer during content read")
			return nil, errors.Wrap(err, "stream closed")

		case ErrorTypeConnectionReset:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "connection_reset").
				Str("description", errorDesc).
				Int("expected", size).
				Msg("connection reset by peer during content read")
			return nil, errors.Wrap(err, "connection reset")

		case ErrorTypeBrokenPipe:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "broken_pipe").
				Str("description", errorDesc).
				Int("expected", size).
				Msg("broken pipe during content read")
			return nil, errors.Wrap(err, "broken pipe")

		case ErrorTypeTimeout:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "timeout").
				Int("expected", size).
				Msg("timeout reading message content")
			return nil, errors.Wrap(err, "timeout")

		case ErrorTypeResourceExhaustion:
			utils.Logger().Error().
				Str("streamID", string(st.ID())).
				Str("errorType", "resource_exhaustion").
				Str("description", errorDesc).
				Int("expected", size).
				Msg("resource exhaustion during content read")
			return nil, errors.Wrap(err, "resource exhaustion")

		case ErrorTypeLocalNetwork:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "local_network").
				Str("description", errorDesc).
				Err(err).
				Int("expected", size).
				Msg("local network error reading message content")
			return nil, errors.Wrap(err, "network error")

		default:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "unknown").
				Str("description", errorDesc).
				Err(err).
				Int("expected", size).
				Msg("failed reading message content")
			return nil, errors.Wrap(err, "content read failed")
		}
	}
	bytesReadCounter.Add(float64(bytesRead))

	if bytesRead != size {
		utils.Logger().Debug().
			Str("streamID", string(st.ID())).
			Int("read", bytesRead).
			Int("expected", size).
			Msg("incomplete message read")
		return nil, errors.Wrapf(errors.New("incomplete read"), "read %d bytes but expected %d", bytesRead, size)
	}

	return content, nil
}

// ReadBytesWithProgress reads bytes from the stream with progress-based timeout.
// Progress tracking only starts after reading size prefix and stops when read completes/errors.
func (st *BaseStream) ReadBytesWithProgress(progressTracker *ProgressTracker) (content []byte, err error) {
	defer func() {
		msgReadCounter.Inc()
		if err != nil {
			msgReadFailedCounterVec.With(prometheus.Labels{"error": err.Error()}).Inc()
		}
		// Always stop tracking when function exits (success or error)
		if progressTracker != nil {
			progressTracker.StopTracking()
		}
	}()

	// Disable read timeout for progress-based reading
	if err := st.raw.SetReadDeadline(time.Time{}); err != nil {
		utils.Logger().Debug().
			Str("streamID", string(st.ID())).
			Err(err).
			Msg("failed to disable read deadline")
		return nil, errors.Wrap(err, "failed to disable read deadline")
	}

	// 1. Read message length prefix (blocking, no timeout - wait indefinitely for size)
	lengthBuf := make([]byte, sizeBytes)
	_, err = io.ReadFull(st.reader, lengthBuf)
	if err != nil {
		// Classify the error for better handling
		errorType, errorDesc := ClassifyStreamError(err)

		switch errorType {
		case ErrorTypeRemoteDisconnect:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "remote_disconnect").
				Msg("stream closed by remote peer")
			return nil, errors.Wrap(err, "stream closed")

		case ErrorTypeConnectionReset:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "connection_reset").
				Str("description", errorDesc).
				Msg("connection reset by peer during length prefix read")
			return nil, errors.Wrap(err, "connection reset")

		case ErrorTypeBrokenPipe:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "broken_pipe").
				Str("description", errorDesc).
				Msg("broken pipe during length prefix read")
			return nil, errors.Wrap(err, "broken pipe")

		case ErrorTypeTimeout:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "timeout").
				Msg("timeout reading length prefix")
			return nil, errors.Wrap(err, "timeout")

		case ErrorTypeResourceExhaustion:
			utils.Logger().Error().
				Str("streamID", string(st.ID())).
				Str("errorType", "resource_exhaustion").
				Str("description", errorDesc).
				Msg("resource exhaustion during length prefix read")
			return nil, errors.Wrap(err, "resource exhaustion")

		case ErrorTypeLocalNetwork:
			utils.Logger().Warn().
				Str("streamID", string(st.ID())).
				Str("errorType", "local_network").
				Str("description", errorDesc).
				Err(err).
				Msg("local network error reading length prefix")
			return nil, errors.Wrap(err, "network error")

		default:
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Str("errorType", "unknown").
				Str("description", errorDesc).
				Err(err).
				Msg("failed reading length prefix")
			return nil, errors.Wrap(err, "length prefix read failed")
		}
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
		return nil, errors.Wrapf(errors.New("message size exceeds limit"), "message size %d exceeds max %d", size, maxMsgBytes)
	}

	// 3. NOW start progress tracking (only after size is read successfully)
	if progressTracker != nil {
		progressTracker.StartTracking()
	}

	// 4. Read message content with progress tracking and chunked reading
	content = make([]byte, size)
	totalRead := 0

	for totalRead < size {
		// Read a chunk with a short timeout using configurable chunk size
		chunkSize := min(int(st.timeoutConfig.ChunkSize), size-totalRead)
		chunk := content[totalRead : totalRead+chunkSize]

		// Set a short deadline for this chunk read using config
		chunkTimeout := st.timeoutConfig.ChunkReadTimeout
		if err := st.raw.SetReadDeadline(time.Now().Add(chunkTimeout)); err != nil {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Err(err).
				Msg("failed to set chunk read deadline")
		}

		// Read chunk with timeout - use single Read instead of ReadFull
		n, err := st.reader.Read(chunk)
		if err != nil {
			// Classify the error for better handling
			errorType, errorDesc := ClassifyStreamError(err)

			// Special handling for timeout errors with progress tracking
			if errorType == ErrorTypeTimeout {
				// Check if we made progress recently (not in this failed read)
				if progressTracker != nil && progressTracker.IsHealthy() {
					progressTracker.ResetTimeout()
					utils.Logger().Debug().
						Str("streamID", string(st.ID())).
						Str("errorType", "timeout_recoverable").
						Int("read", totalRead).
						Int("expected", size).
						Msg("recent progress detected, continuing read")
					continue
				} else {
					utils.Logger().Warn().
						Str("streamID", string(st.ID())).
						Str("errorType", "timeout_unrecoverable").
						Int("read", totalRead).
						Int("expected", size).
						Msg("no recent progress detected, timeout")
					return nil, errors.Wrap(err, "progress timeout")
				}
			}

			// Handle other error types
			switch errorType {
			case ErrorTypeRemoteDisconnect:
				utils.Logger().Debug().
					Str("streamID", string(st.ID())).
					Str("errorType", "remote_disconnect").
					Int("read", totalRead).
					Int("expected", size).
					Msg("stream closed by remote peer during chunk read")
				return nil, errors.Wrap(err, "stream closed")

			case ErrorTypeConnectionReset:
				utils.Logger().Warn().
					Str("streamID", string(st.ID())).
					Str("errorType", "connection_reset").
					Str("description", errorDesc).
					Int("read", totalRead).
					Int("expected", size).
					Msg("connection reset by peer during chunk read")
				return nil, errors.Wrap(err, "connection reset")

			case ErrorTypeBrokenPipe:
				utils.Logger().Warn().
					Str("streamID", string(st.ID())).
					Str("errorType", "broken_pipe").
					Str("description", errorDesc).
					Int("read", totalRead).
					Int("expected", size).
					Msg("broken pipe during chunk read")
				return nil, errors.Wrap(err, "broken pipe")

			case ErrorTypeResourceExhaustion:
				utils.Logger().Error().
					Str("streamID", string(st.ID())).
					Str("errorType", "resource_exhaustion").
					Str("description", errorDesc).
					Int("read", totalRead).
					Int("expected", size).
					Msg("resource exhaustion during chunk read")
				return nil, errors.Wrap(err, "resource exhaustion")

			case ErrorTypeLocalNetwork:
				utils.Logger().Warn().
					Str("streamID", string(st.ID())).
					Str("errorType", "local_network").
					Str("description", errorDesc).
					Err(err).
					Int("read", totalRead).
					Int("expected", size).
					Msg("local network error reading message content chunk")
				return nil, errors.Wrap(err, "network error")

			default:
				utils.Logger().Debug().
					Str("streamID", string(st.ID())).
					Str("errorType", "unknown").
					Str("description", errorDesc).
					Err(err).
					Int("read", totalRead).
					Int("expected", size).
					Msg("failed reading message content chunk")
				return nil, errors.Wrap(err, "content read failed")
			}
		}

		// Check if we got some data
		if n == 0 {
			// No data read, this might indicate end of stream
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Int("read", totalRead).
				Int("expected", size).
				Msg("no data read from chunk, possible end of stream")
			return nil, errors.Wrap(io.EOF, "unexpected end of stream during chunk read")
		}

		totalRead += n

		// Update progress tracker
		if progressTracker != nil {
			progressTracker.UpdateProgress(n)
		}

		// Reset deadline for next chunk
		if err := st.raw.SetReadDeadline(time.Time{}); err != nil {
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Err(err).
				Msg("failed to reset read deadline")
		}

		// Log progress for large messages
		if size > 1024*1024 { // Log progress for messages > 1MB
			utils.Logger().Debug().
				Str("streamID", string(st.ID())).
				Int("read", totalRead).
				Int("expected", size).
				Float64("progress", float64(totalRead)/float64(size)*100).
				Msg("reading large message")
		}
	}

	bytesReadCounter.Add(float64(totalRead))

	if totalRead != size {
		utils.Logger().Debug().
			Str("streamID", string(st.ID())).
			Int("read", totalRead).
			Int("expected", size).
			Msg("incomplete message read")
		return nil, errors.Wrapf(errors.New("incomplete read"), "read %d bytes but expected %d", totalRead, size)
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetProgressTracker sets a custom progress tracker for this stream
func (st *BaseStream) SetProgressTracker(tracker *ProgressTracker) {
	st.progressTracker = tracker
}

// SetTimeoutConfig sets a custom timeout configuration for this stream
func (st *BaseStream) SetTimeoutConfig(config *StreamTimeoutConfig) {
	st.timeoutConfig = config
	// Update the progress tracker with new configuration
	if st.progressTracker != nil && config != nil {
		st.progressTracker = NewProgressTracker(config.ProgressTimeout, config.ProgressThreshold)
	}
}
