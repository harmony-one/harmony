// Package ctxerror provides a context-aware error facility.
//
// Inspired by log15-style (semi-)structured logging,
// it also provides a log15 bridge.
package ctxerror

//go:generate mockgen -source ctxerror.go -destination mock/ctxerror.go

import (
	"fmt"
)

// CtxError is a context-aware error container.
type CtxError interface {
	// Error returns a fully formatted message, with context info.
	Error() string

	// Message returns the bare error message, without context info.
	Message() string

	// Contexts returns message contexts.
	// Caller shall not modify the returned map.
	Contexts() map[string]interface{}

	// WithCause chains an error after the receiver.
	// It returns the merged/chained instance,
	// where the message is "<receiver.Message>: <c.Message>",
	// and with contexts merged (ones in c takes precedence).
	WithCause(c error) CtxError
}

type ctxError struct {
	msg string
	ctx map[string]interface{}
}

// New creates and returns a new context-aware error.
func New(msg string, ctx ...interface{}) CtxError {
	e := &ctxError{msg: msg, ctx: make(map[string]interface{})}
	e.updateCtx(ctx...)
	return e
}

func (e *ctxError) updateCtx(ctx ...interface{}) {
	var name string
	if len(ctx)%2 == 1 {
		ctx = append(ctx, nil)
	}
	for idx, value := range ctx {
		if idx%2 == 0 {
			name = value.(string)
		} else {
			e.ctx[name] = value
		}
	}
}

// Error returns a fully formatted message, with context info.
func (e *ctxError) Error() string {
	s := e.msg
	for k, v := range e.ctx {
		s += fmt.Sprintf(", %s=%#v", k, v)
	}
	return s
}

// Message returns the bare error message, without context info.
func (e *ctxError) Message() string {
	return e.msg
}

// Contexts returns message contexts.
// Caller shall not modify the returned map.
func (e *ctxError) Contexts() map[string]interface{} {
	return e.ctx
}

// WithCause chains an error after the receiver.
// It returns the merged/chained instance,
// where the message is “<receiver.Message>: <c.Message>”,
// and with contexts merged (ones in c takes precedence).
func (e *ctxError) WithCause(c error) CtxError {
	r := &ctxError{msg: e.msg + ": ", ctx: make(map[string]interface{})}
	for k, v := range e.ctx {
		r.ctx[k] = v
	}
	switch c := c.(type) {
	case *ctxError:
		r.msg += c.msg
		for k, v := range c.ctx {
			r.ctx[k] = v
		}
	default:
		r.msg += c.Error()
	}
	return r
}

// Log15Func is a log15-compatible logging function.
type Log15Func func(msg string, ctx ...interface{})

// Log15Logger logs something with a log15-style logging function.
type Log15Logger interface {
	Log15(f Log15Func)
}

// Log15 logs the receiver with a log15-style logging function.
func (e *ctxError) Log15(f Log15Func) {
	var ctx []interface{}
	for k, v := range e.ctx {
		ctx = append(ctx, k, v)
	}
	f(e.msg, ctx...)
}

// Log15 logs an error with a log15-style logging function.
// It handles both regular errors and Log15Logger-compliant errors.
func Log15(f Log15Func, e error) {
	if e15, ok := e.(Log15Logger); ok {
		e15.Log15(f)
	} else {
		f(e.Error())
	}
}
