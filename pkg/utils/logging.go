package utils

import (
	"path"
	"runtime"

	"github.com/ethereum/go-ethereum/log"
)

// WithCallerSkip logs the caller info in three context items "funcFile",
// "funcLine", and "funcName".  skip is the number of frames to skip.
// 0 adds the info of the caller of this WithCallerSkip.
func WithCallerSkip(logger log.Logger, skip int) log.Logger {
	pcs := make([]uintptr, 1)
	numPCs := runtime.Callers(2+skip, pcs)
	var ctx []interface{}
	if numPCs >= 1 {
		frames := runtime.CallersFrames(pcs[:numPCs])
		frame, _ := frames.Next()
		if frame.Function != "" {
			ctx = append(ctx, "funcName", frame.Function)
		}
		if frame.File != "" {
			ctx = append(ctx, "funcFile", path.Base(frame.File))
		}
		if frame.Line != 0 {
			ctx = append(ctx, "funcLine", frame.Line)
		}
	}
	return logger.New(ctx...)
}

// WithCaller logs the caller info in three context items "funcFile",
// "funcLine", and "funcName".
func WithCaller(logger log.Logger) log.Logger {
	return WithCallerSkip(logger, 1)
}

// GetLogger is a shorthand for WithCaller(GetLogInstance()).
func GetLogger() log.Logger {
	return WithCallerSkip(GetLogInstance(), 1)
}
