package sttypes

import (
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

// StreamErrorType represents different types of stream errors
type StreamErrorType int

const (
	ErrorTypeNoError StreamErrorType = iota
	ErrorTypeRemoteDisconnect
	ErrorTypeLocalNetwork
	ErrorTypeTimeout
	ErrorTypeProgressTimeout
	ErrorTypeReadDeadline
	ErrorTypeWriteDeadline
	ErrorTypeResourceExhaustion
	ErrorTypeProtocol
	ErrorTypeConnectionReset
	ErrorTypeBrokenPipe
	ErrorTypeUnknown
)

// String returns the string representation of StreamErrorType
func (e StreamErrorType) String() string {
	switch e {
	case ErrorTypeNoError:
		return "no_error"
	case ErrorTypeRemoteDisconnect:
		return "remote_disconnect"
	case ErrorTypeLocalNetwork:
		return "local_network"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeProgressTimeout:
		return "progress_timeout"
	case ErrorTypeReadDeadline:
		return "read_deadline"
	case ErrorTypeWriteDeadline:
		return "write_deadline"
	case ErrorTypeResourceExhaustion:
		return "resource_exhaustion"
	case ErrorTypeProtocol:
		return "protocol"
	case ErrorTypeConnectionReset:
		return "connection_reset"
	case ErrorTypeBrokenPipe:
		return "broken_pipe"
	case ErrorTypeUnknown:
		return "unknown"
	default:
		return "invalid"
	}
}

// ClassifyStreamError classifies stream errors to help with better error handling
// It uses type assertions first for reliability, then falls back to string matching
func ClassifyStreamError(err error) (StreamErrorType, string) {
	if err == nil {
		return ErrorTypeNoError, "no error"
	}

	// 1. Check for EOF (remote peer disconnect) - use errors.Is to handle wrapped errors
	if errors.Is(err, io.EOF) {
		return ErrorTypeRemoteDisconnect, "remote peer disconnected"
	}

	// 2. Check for syscall errors first (most specific)
	var sysErrno syscall.Errno
	if errors.As(err, &sysErrno) {
		switch sysErrno {
		case syscall.ECONNRESET:
			return ErrorTypeConnectionReset, "connection reset by peer"
		case syscall.EPIPE:
			return ErrorTypeBrokenPipe, "broken pipe"
		case syscall.ENOBUFS:
			return ErrorTypeResourceExhaustion, "no buffer space available"
		case syscall.ENOMEM:
			return ErrorTypeResourceExhaustion, "out of memory"
		}
	}

	// 3. Check for net.OpError (wrapped system errors)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Err != nil {
			// Check the underlying error for syscall errors
			var innerSysErrno syscall.Errno
			if errors.As(opErr.Err, &innerSysErrno) {
				switch innerSysErrno {
				case syscall.ECONNRESET:
					return ErrorTypeConnectionReset, "connection reset by peer"
				case syscall.EPIPE:
					return ErrorTypeBrokenPipe, "broken pipe"
				case syscall.ENOBUFS:
					return ErrorTypeResourceExhaustion, "no buffer space available"
				case syscall.ENOMEM:
					return ErrorTypeResourceExhaustion, "out of memory"
				}
			}
			// Check for resource exhaustion in error message
			errStr := strings.ToLower(opErr.Err.Error())
			if strings.Contains(errStr, "no buffer space") {
				return ErrorTypeResourceExhaustion, "no buffer space available"
			}
		}
	}

	// 4. Check for net.Error (network errors)
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return ErrorTypeTimeout, "network timeout"
		}
		// For non-timeout network errors, check underlying cause
		if opErr, ok := netErr.(*net.OpError); ok && opErr.Err != nil {
			var innerSysErrno syscall.Errno
			if errors.As(opErr.Err, &innerSysErrno) {
				switch innerSysErrno {
				case syscall.ECONNRESET:
					return ErrorTypeConnectionReset, "connection reset by peer"
				case syscall.EPIPE:
					return ErrorTypeBrokenPipe, "broken pipe"
				}
			}
		}
		return ErrorTypeLocalNetwork, "network error"
	}

	// 5. Fall back to string matching for application-specific errors
	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)

	// Check for progress timeout specifically (application-level error)
	if strings.Contains(errStrLower, "progress timeout") {
		return ErrorTypeProgressTimeout, "progress timeout due to lack of content reading progress"
	}

	// Check for deadline errors specifically (application-level error)
	if strings.Contains(errStrLower, "read deadline") || strings.Contains(errStrLower, "setreaddeadline") {
		return ErrorTypeReadDeadline, "read deadline exceeded or failed to set read deadline"
	}
	if strings.Contains(errStrLower, "write deadline") || strings.Contains(errStrLower, "setwritedeadline") {
		return ErrorTypeWriteDeadline, "write deadline exceeded or failed to set write deadline"
	}

	// Check for connection reset in error message (fallback for wrapped errors)
	if strings.Contains(errStrLower, "connection reset") {
		return ErrorTypeConnectionReset, "connection reset by peer"
	}

	// Check for broken pipe in error message (fallback for wrapped errors)
	if strings.Contains(errStrLower, "broken pipe") {
		return ErrorTypeBrokenPipe, "broken pipe"
	}

	// Check for protocol-related errors
	if strings.Contains(errStrLower, "invalid") || strings.Contains(errStrLower, "malformed") {
		return ErrorTypeProtocol, "protocol error"
	}

	// Check for resource exhaustion (fallback)
	if strings.Contains(errStrLower, "too many") || strings.Contains(errStrLower, "resource") {
		return ErrorTypeResourceExhaustion, "resource exhaustion"
	}

	return ErrorTypeUnknown, "unknown error"
}

// IsRecoverableError determines if an error type is potentially recoverable
func IsRecoverableError(errorType StreamErrorType) bool {
	switch errorType {
	case ErrorTypeNoError, ErrorTypeTimeout, ErrorTypeProgressTimeout, ErrorTypeReadDeadline, ErrorTypeWriteDeadline, ErrorTypeLocalNetwork:
		return true
	case ErrorTypeResourceExhaustion, ErrorTypeRemoteDisconnect, ErrorTypeConnectionReset, ErrorTypeBrokenPipe,
		ErrorTypeProtocol, ErrorTypeUnknown:
		return false
	default:
		return false
	}
}

// IsRemoteDeadError determines if an error indicates the remote peer is dead/unreachable
func IsRemoteDeadError(errorType StreamErrorType) bool {
	switch errorType {
	case ErrorTypeRemoteDisconnect, ErrorTypeConnectionReset, ErrorTypeBrokenPipe:
		return true
	case ErrorTypeNoError, ErrorTypeTimeout, ErrorTypeLocalNetwork, ErrorTypeResourceExhaustion,
		ErrorTypeProtocol, ErrorTypeUnknown:
		return false
	default:
		return false
	}
}

// IsCriticalError determines if an error is critical and should trigger immediate stream removal
func IsCriticalError(errorType StreamErrorType) bool {
	switch errorType {
	case ErrorTypeResourceExhaustion, ErrorTypeRemoteDisconnect, ErrorTypeConnectionReset, ErrorTypeBrokenPipe,
		ErrorTypeProtocol, ErrorTypeUnknown:
		return true
	case ErrorTypeNoError, ErrorTypeTimeout, ErrorTypeProgressTimeout, ErrorTypeReadDeadline, ErrorTypeWriteDeadline, ErrorTypeLocalNetwork:
		return false
	default:
		return true // Default to critical for unknown error types
	}
}

// GetErrorSeverity returns the severity level of an error type
func GetErrorSeverity(errorType StreamErrorType) string {
	switch errorType {
	case ErrorTypeNoError:
		return "info" // No error
	case ErrorTypeRemoteDisconnect:
		return "warn" // Normal peer disconnect
	case ErrorTypeConnectionReset, ErrorTypeBrokenPipe:
		return "warn" // Peer issues
	case ErrorTypeTimeout:
		return "warn" // Network issues
	case ErrorTypeProgressTimeout:
		return "warn" // Progress timeout issues
	case ErrorTypeReadDeadline, ErrorTypeWriteDeadline:
		return "info" // Deadline errors are informational (recoverable)
	case ErrorTypeLocalNetwork:
		return "warn" // Local network problems
	case ErrorTypeResourceExhaustion:
		return "error" // System resource issues
	case ErrorTypeProtocol:
		return "error" // Protocol corruption
	case ErrorTypeUnknown:
		return "debug" // Unknown errors
	default:
		return "debug"
	}
}

// GetErrorCategory returns a human-readable category for the error
func GetErrorCategory(errorType StreamErrorType) string {
	switch errorType {
	case ErrorTypeNoError:
		return "no_error"
	case ErrorTypeRemoteDisconnect, ErrorTypeConnectionReset, ErrorTypeBrokenPipe:
		return "remote_peer"
	case ErrorTypeTimeout, ErrorTypeProgressTimeout, ErrorTypeReadDeadline, ErrorTypeWriteDeadline, ErrorTypeLocalNetwork:
		return "network"
	case ErrorTypeResourceExhaustion:
		return "system_resources"
	case ErrorTypeProtocol:
		return "protocol"
	case ErrorTypeUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// ShouldRetryConnection determines if a connection should be retried based on error type
func ShouldRetryConnection(errorType StreamErrorType) bool {
	switch errorType {
	case ErrorTypeTimeout, ErrorTypeProgressTimeout, ErrorTypeReadDeadline, ErrorTypeWriteDeadline, ErrorTypeLocalNetwork, ErrorTypeResourceExhaustion:
		return true
	case ErrorTypeNoError, ErrorTypeRemoteDisconnect, ErrorTypeConnectionReset, ErrorTypeBrokenPipe,
		ErrorTypeProtocol, ErrorTypeUnknown:
		return false
	default:
		return false
	}
}

// GetRetryDelay returns the recommended retry delay for an error type
func GetRetryDelay(errorType StreamErrorType) int {
	switch errorType {
	case ErrorTypeTimeout:
		return 5 // 5 seconds for timeouts
	case ErrorTypeProgressTimeout:
		return 5 // 5 seconds for progress timeouts
	case ErrorTypeReadDeadline, ErrorTypeWriteDeadline:
		return 1 // 1 second for deadline errors (quick retry)
	case ErrorTypeLocalNetwork:
		return 10 // 10 seconds for local network issues
	case ErrorTypeResourceExhaustion:
		return 30 // 30 seconds for resource issues
	default:
		return 0 // No retry for other error types
	}
}

// StreamWriteError represents an error that occurred during actual stream writing
// and should cause the stream to be closed
type StreamWriteError struct {
	Err error
}

func (e *StreamWriteError) Error() string {
	return fmt.Sprintf("stream write error: %v", e.Err)
}

func (e *StreamWriteError) Unwrap() error {
	return e.Err
}

// MessageError represents an error related to message validation or parsing
// that should NOT cause the stream to be closed
type MessageError struct {
	Err error
}

func (e *MessageError) Error() string {
	return fmt.Sprintf("message error: %v", e.Err)
}

func (e *MessageError) Unwrap() error {
	return e.Err
}

// IsStreamWriteError checks if the error is a StreamWriteError
func IsStreamWriteError(err error) bool {
	var streamErr *StreamWriteError
	return errors.As(err, &streamErr)
}

// IsMessageError checks if the error is a MessageError
func IsMessageError(err error) bool {
	var msgErr *MessageError
	return errors.As(err, &msgErr)
}

// ShouldCloseStream determines if an error should cause the stream to be closed
func ShouldCloseStream(err error) bool {
	// Don't close stream for message errors
	if IsMessageError(err) {
		return false
	}

	// Close stream for stream write errors
	if IsStreamWriteError(err) {
		return true
	}

	// For other errors, use the existing classification system
	errorType, _ := ClassifyStreamError(err)
	return IsCriticalError(errorType)
}

// GetWriteErrorSeverity returns the appropriate log severity for write errors
func GetWriteErrorSeverity(err error) string {
	if IsStreamWriteError(err) {
		return "info" // Stream write errors are informational (stream will be closed)
	}
	if IsMessageError(err) {
		return "warn" // Message errors are warnings (stream continues)
	}

	// For other errors, use existing classification
	errorType, _ := ClassifyStreamError(err)
	return GetErrorSeverity(errorType)
}
