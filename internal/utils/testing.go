package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/log"
)

// TestLogRedirector is a log15-compatible log handler that logs to the testing
// object.  It is useful when debugging, because it surfaces application log
// messages – such as ones emitted by the SUT – which are otherwise discarded
// and not displayed during testing.
//
// Typical usage:
//
//	func TestMyFunc(t *testing.T) {
//		l := utils.GetLogInstance()
//		lrd := NewTestLogRedirector(l, t)
//		defer lrd.Close()
//
//		// Everything sent to the logger l will be printed onto the test log,
//		// until lrd.Close() is called at the end of the function.
//		// Contexts are formatted using the "%#v" format specifier.
//		l.Debug("hello", "audience", "world")
//		// The above prints: hello, audience="world"
//	}
type TestLogRedirector struct {
	l log.Logger
	h log.Handler
	t *testing.T
}

// NewTestLogRedirector returns a new testing log redirector.
// The given logger's handler is saved and replaced by the receiver.
// Caller shall ensure Close() is called when the redirector is no longer
// needed.
func NewTestLogRedirector(l log.Logger, t *testing.T) *TestLogRedirector {
	r := &TestLogRedirector{l: l, h: l.GetHandler(), t: t}
	l.SetHandler(r)
	return r
}

// Log logs the given log15 record into the testing object.
func (redirector *TestLogRedirector) Log(r *log.Record) error {
	segments := []string{fmt.Sprintf("[%s] %s", r.Lvl.String(), r.Msg)}
	for i := 0; i+1 < len(r.Ctx); i += 2 {
		segments = append(segments, fmt.Sprintf("%s=%#v", r.Ctx[i], r.Ctx[i+1]))
	}
	redirector.t.Log(strings.Join(segments, ", "))
	return nil
}

// Close restores the log handler back to the original one.
func (redirector *TestLogRedirector) Close() error {
	if redirector.l != nil {
		redirector.l.SetHandler(redirector.h)
		redirector.l = nil
		redirector.h = nil
	}
	return nil
}
