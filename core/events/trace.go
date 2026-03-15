package events

import (
	"github.com/harmony-one/harmony/hmy/tracers/native"
)

type TraceEvent struct {
	Tracer *native.ParityBlockTracer
}
