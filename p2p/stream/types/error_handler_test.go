package sttypes

import (
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/pkg/errors"
)

func TestClassifyStreamError_NilError(t *testing.T) {
	errorType, desc := ClassifyStreamError(nil)
	if errorType != ErrorTypeNoError {
		t.Errorf("Expected ErrorTypeNoError, got %v", errorType)
	}
	if desc != "no error" {
		t.Errorf("Expected 'no error', got %s", desc)
	}
}

func TestClassifyStreamError_EOF(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"direct EOF", io.EOF},
		{"wrapped EOF", errors.Wrap(io.EOF, "read failed")},
		{"double wrapped EOF", errors.Wrap(errors.Wrap(io.EOF, "inner"), "outer")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorType, desc := ClassifyStreamError(tt.err)
			if errorType != ErrorTypeRemoteDisconnect {
				t.Errorf("Expected ErrorTypeRemoteDisconnect, got %v", errorType)
			}
			if desc != "remote peer disconnected" {
				t.Errorf("Expected 'remote peer disconnected', got %s", desc)
			}
		})
	}
}

func TestClassifyStreamError_SyscallErrors(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() error
		expected    StreamErrorType
		description string
	}{
		{"ECONNRESET", func() error { return errors.Wrap(syscall.ECONNRESET, "wrapped") }, ErrorTypeConnectionReset, "connection reset by peer"},
		{"EPIPE", func() error { return errors.Wrap(syscall.EPIPE, "wrapped") }, ErrorTypeBrokenPipe, "broken pipe"},
		{"ENOBUFS", func() error { return errors.Wrap(syscall.ENOBUFS, "wrapped") }, ErrorTypeResourceExhaustion, "no buffer space available"},
		{"ENOMEM", func() error { return errors.Wrap(syscall.ENOMEM, "wrapped") }, ErrorTypeResourceExhaustion, "out of memory"},
		// Unknown syscall errors: when they don't match any case, they fall through
		// Both direct and wrapped unknown syscall errors may be classified as local network
		// if they satisfy the net.Error check (which syscall.Errno does not, but the fallthrough
		// behavior may classify them differently)
		{"unknown syscall direct", func() error { return syscall.EINVAL }, ErrorTypeLocalNetwork, "network error"},
		{"unknown syscall wrapped", func() error { return errors.Wrap(syscall.EINVAL, "wrapped") }, ErrorTypeLocalNetwork, "network error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			errorType, desc := ClassifyStreamError(err)
			if errorType != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, errorType)
			}
			if tt.expected != ErrorTypeUnknown && desc != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, desc)
			}
		})
	}
}

func TestClassifyStreamError_NetOpError(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() error
		expected    StreamErrorType
		description string
	}{
		{
			"OpError with ECONNRESET",
			func() error {
				return &net.OpError{
					Op:  "read",
					Net: "tcp",
					Err: syscall.ECONNRESET,
				}
			},
			ErrorTypeConnectionReset,
			"connection reset by peer",
		},
		{
			"OpError with EPIPE",
			func() error {
				return &net.OpError{
					Op:  "write",
					Net: "tcp",
					Err: syscall.EPIPE,
				}
			},
			ErrorTypeBrokenPipe,
			"broken pipe",
		},
		{
			"OpError with ENOBUFS",
			func() error {
				return &net.OpError{
					Op:  "write",
					Net: "tcp",
					Err: syscall.ENOBUFS,
				}
			},
			ErrorTypeResourceExhaustion,
			"no buffer space available",
		},
		{
			"OpError with string error containing 'no buffer space'",
			func() error {
				return &net.OpError{
					Op:  "write",
					Net: "tcp",
					Err: errors.New("no buffer space available"),
				}
			},
			ErrorTypeResourceExhaustion,
			"no buffer space available",
		},
		{
			"wrapped OpError",
			func() error {
				return errors.Wrap(&net.OpError{
					Op:  "read",
					Net: "tcp",
					Err: syscall.ECONNRESET,
				}, "network operation failed")
			},
			ErrorTypeConnectionReset,
			"connection reset by peer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			errorType, desc := ClassifyStreamError(err)
			if errorType != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, errorType)
			}
			if desc != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, desc)
			}
		})
	}
}

func TestClassifyStreamError_NetError(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() error
		expected    StreamErrorType
		description string
	}{
		{
			"timeout error",
			func() error {
				return &timeoutError{msg: "i/o timeout"}
			},
			ErrorTypeTimeout,
			"network timeout",
		},
		{
			"non-timeout network error",
			func() error {
				return &netError{msg: "network unreachable"}
			},
			ErrorTypeLocalNetwork,
			"network error",
		},
		{
			"net.Error with OpError containing ECONNRESET",
			func() error {
				// Create a net.Error that is actually a *net.OpError
				return &net.OpError{
					Op:  "read",
					Net: "tcp",
					Err: syscall.ECONNRESET,
				}
			},
			ErrorTypeConnectionReset,
			"connection reset by peer",
		},
		{
			"wrapped net.Error",
			func() error {
				return errors.Wrap(&timeoutError{msg: "timeout"}, "wrapped")
			},
			ErrorTypeTimeout,
			"network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			errorType, desc := ClassifyStreamError(err)
			if errorType != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, errorType)
			}
			if desc != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, desc)
			}
		})
	}
}

func TestClassifyStreamError_StringMatching(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expected    StreamErrorType
		description string
	}{
		{"progress timeout", errors.New("progress timeout occurred"), ErrorTypeProgressTimeout, "progress timeout due to lack of content reading progress"},
		{"read deadline", errors.New("read deadline exceeded"), ErrorTypeReadDeadline, "read deadline exceeded or failed to set read deadline"},
		{"setReadDeadline", errors.New("failed to setReadDeadline"), ErrorTypeReadDeadline, "read deadline exceeded or failed to set read deadline"},
		{"write deadline", errors.New("write deadline exceeded"), ErrorTypeWriteDeadline, "write deadline exceeded or failed to set write deadline"},
		{"setWriteDeadline", errors.New("failed to setWriteDeadline"), ErrorTypeWriteDeadline, "write deadline exceeded or failed to set write deadline"},
		{"connection reset string", errors.New("connection reset by peer"), ErrorTypeConnectionReset, "connection reset by peer"},
		{"broken pipe string", errors.New("broken pipe error"), ErrorTypeBrokenPipe, "broken pipe"},
		{"invalid protocol", errors.New("invalid message format"), ErrorTypeProtocol, "protocol error"},
		{"malformed data", errors.New("malformed packet"), ErrorTypeProtocol, "protocol error"},
		{"too many connections", errors.New("too many open connections"), ErrorTypeResourceExhaustion, "resource exhaustion"},
		{"resource limit", errors.New("resource limit exceeded"), ErrorTypeResourceExhaustion, "resource exhaustion"},
		{"unknown error", errors.New("some random error"), ErrorTypeUnknown, "unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorType, desc := ClassifyStreamError(tt.err)
			if errorType != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, errorType)
			}
			if desc != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, desc)
			}
		})
	}
}

func TestClassifyStreamError_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expected    StreamErrorType
		description string
	}{
		{"uppercase PROGRESS TIMEOUT", errors.New("PROGRESS TIMEOUT"), ErrorTypeProgressTimeout, "progress timeout due to lack of content reading progress"},
		{"mixed case Connection Reset", errors.New("Connection Reset by peer"), ErrorTypeConnectionReset, "connection reset by peer"},
		{"uppercase BROKEN PIPE", errors.New("BROKEN PIPE"), ErrorTypeBrokenPipe, "broken pipe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorType, desc := ClassifyStreamError(tt.err)
			if errorType != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, errorType)
			}
			if desc != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, desc)
			}
		})
	}
}

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name     string
		errType  StreamErrorType
		expected bool
	}{
		{"NoError", ErrorTypeNoError, true},
		{"Timeout", ErrorTypeTimeout, true},
		{"ProgressTimeout", ErrorTypeProgressTimeout, true},
		{"ReadDeadline", ErrorTypeReadDeadline, true},
		{"WriteDeadline", ErrorTypeWriteDeadline, true},
		{"LocalNetwork", ErrorTypeLocalNetwork, true},
		{"ResourceExhaustion", ErrorTypeResourceExhaustion, false},
		{"RemoteDisconnect", ErrorTypeRemoteDisconnect, false},
		{"ConnectionReset", ErrorTypeConnectionReset, false},
		{"BrokenPipe", ErrorTypeBrokenPipe, false},
		{"Protocol", ErrorTypeProtocol, false},
		{"Unknown", ErrorTypeUnknown, false},
		{"Invalid", StreamErrorType(999), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverableError(tt.errType)
			if result != tt.expected {
				t.Errorf("IsRecoverableError(%v) = %v, expected %v", tt.errType, result, tt.expected)
			}
		})
	}
}

func TestIsCriticalError(t *testing.T) {
	tests := []struct {
		name     string
		errType  StreamErrorType
		expected bool
	}{
		{"ResourceExhaustion", ErrorTypeResourceExhaustion, true},
		{"RemoteDisconnect", ErrorTypeRemoteDisconnect, true},
		{"ConnectionReset", ErrorTypeConnectionReset, true},
		{"BrokenPipe", ErrorTypeBrokenPipe, true},
		{"Protocol", ErrorTypeProtocol, true},
		{"Unknown", ErrorTypeUnknown, true},
		{"NoError", ErrorTypeNoError, false},
		{"Timeout", ErrorTypeTimeout, false},
		{"ProgressTimeout", ErrorTypeProgressTimeout, false},
		{"ReadDeadline", ErrorTypeReadDeadline, false},
		{"WriteDeadline", ErrorTypeWriteDeadline, false},
		{"LocalNetwork", ErrorTypeLocalNetwork, false},
		{"Invalid", StreamErrorType(999), true}, // Default to critical
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCriticalError(tt.errType)
			if result != tt.expected {
				t.Errorf("IsCriticalError(%v) = %v, expected %v", tt.errType, result, tt.expected)
			}
		})
	}
}

func TestShouldCloseStream(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"MessageError", &MessageError{Err: errors.New("message too long")}, false},
		{"wrapped MessageError", errors.Wrap(&MessageError{Err: errors.New("invalid")}, "outer"), false},
		{"StreamWriteError", &StreamWriteError{Err: errors.New("write failed")}, true},
		{"wrapped StreamWriteError", errors.Wrap(&StreamWriteError{Err: errors.New("write failed")}, "outer"), true},
		{"EOF", io.EOF, true},
		{"wrapped EOF", errors.Wrap(io.EOF, "read failed"), true},
		{"ECONNRESET", syscall.ECONNRESET, true},
		{"EPIPE", syscall.EPIPE, true},
		{"ENOBUFS", syscall.ENOBUFS, true},
		{"ENOMEM", syscall.ENOMEM, true},
		{"timeout error", &timeoutError{msg: "timeout"}, false},
		{"progress timeout", errors.New("progress timeout"), false},
		{"read deadline", errors.New("read deadline exceeded"), false},
		{"unknown error", errors.New("unknown error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldCloseStream(tt.err)
			if result != tt.expected {
				t.Errorf("ShouldCloseStream(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsStreamWriteError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"StreamWriteError", &StreamWriteError{Err: errors.New("write failed")}, true},
		{"wrapped StreamWriteError", errors.Wrap(&StreamWriteError{Err: errors.New("write failed")}, "outer"), true},
		{"MessageError", &MessageError{Err: errors.New("invalid")}, false},
		{"regular error", errors.New("some error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStreamWriteError(tt.err)
			if result != tt.expected {
				t.Errorf("IsStreamWriteError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsMessageError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"MessageError", &MessageError{Err: errors.New("message too long")}, true},
		{"wrapped MessageError", errors.Wrap(&MessageError{Err: errors.New("invalid")}, "outer"), true},
		{"StreamWriteError", &StreamWriteError{Err: errors.New("write failed")}, false},
		{"regular error", errors.New("some error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMessageError(tt.err)
			if result != tt.expected {
				t.Errorf("IsMessageError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestGetErrorSeverity(t *testing.T) {
	tests := []struct {
		name     string
		errType  StreamErrorType
		expected string
	}{
		{"NoError", ErrorTypeNoError, "info"},
		{"RemoteDisconnect", ErrorTypeRemoteDisconnect, "warn"},
		{"ConnectionReset", ErrorTypeConnectionReset, "warn"},
		{"BrokenPipe", ErrorTypeBrokenPipe, "warn"},
		{"Timeout", ErrorTypeTimeout, "warn"},
		{"ProgressTimeout", ErrorTypeProgressTimeout, "warn"},
		{"ReadDeadline", ErrorTypeReadDeadline, "info"},
		{"WriteDeadline", ErrorTypeWriteDeadline, "info"},
		{"LocalNetwork", ErrorTypeLocalNetwork, "warn"},
		{"ResourceExhaustion", ErrorTypeResourceExhaustion, "error"},
		{"Protocol", ErrorTypeProtocol, "error"},
		{"Unknown", ErrorTypeUnknown, "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorSeverity(tt.errType)
			if result != tt.expected {
				t.Errorf("GetErrorSeverity(%v) = %v, expected %v", tt.errType, result, tt.expected)
			}
		})
	}
}

func TestGetWriteErrorSeverity(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"StreamWriteError", &StreamWriteError{Err: errors.New("write failed")}, "info"},
		{"MessageError", &MessageError{Err: errors.New("message too long")}, "warn"},
		{"EOF", io.EOF, "warn"},
		{"timeout", &timeoutError{msg: "timeout"}, "warn"},
		{"ECONNRESET", syscall.ECONNRESET, "warn"},
		{"unknown", errors.New("unknown"), "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetWriteErrorSeverity(tt.err)
			if result != tt.expected {
				t.Errorf("GetWriteErrorSeverity(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// Helper types for testing

type timeoutError struct {
	msg   string
	opErr *net.OpError
}

func (e *timeoutError) Error() string {
	return e.msg
}

func (e *timeoutError) Timeout() bool {
	return true
}

func (e *timeoutError) Temporary() bool {
	return false
}

type netError struct {
	msg   string
	opErr *net.OpError
}

func (e *netError) Error() string {
	return e.msg
}

func (e *netError) Timeout() bool {
	return false
}

func (e *netError) Temporary() bool {
	return false
}
