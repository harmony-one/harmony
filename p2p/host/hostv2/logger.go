package hostv2

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/whyrusleeping/go-logging"

	"github.com/harmony-one/harmony/internal/utils"
)

type goLoggingBackend struct {
	mtx          sync.Mutex
	levels       map[ /*module*/ string]logging.Level
	defaultLevel logging.Level
}

func newGoLoggingBackend() *goLoggingBackend {
	return &goLoggingBackend{
		levels:       make(map[string]logging.Level),
		defaultLevel: logging.DEBUG,
	}
}

func (l *goLoggingBackend) GetLevel(module string) logging.Level {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if level, ok := l.levels[module]; ok {
		return level
	} else {
		return logging.DEBUG
	}
}

func (l *goLoggingBackend) SetLevel(level logging.Level, module string) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.levels[module] = level
}

func (l *goLoggingBackend) IsEnabledFor(level logging.Level, module string) bool {
	return level <= l.GetLevel(module)
}

func (l goLoggingBackend) Log(
	level logging.Level, callDepth int, record *logging.Record,
) error {
	var e *zerolog.Event
	zl := utils.RawLogger().Hook(utils.ZerologCallerHook{Skip: callDepth + 1})
	switch level {
	case logging.DEBUG:
		e = zl.Debug()
	case logging.INFO:
		e = zl.Info()
	case logging.NOTICE:
		e = zl.Info()
	case logging.WARNING:
		e = zl.Warn()
	case logging.ERROR:
		e = zl.Error()
	case logging.CRITICAL:
		e = zl.Fatal()
	default:
		e = zl.Log()
	}
	e.
		Str("go_logging_module", record.Module).
		Time("go_logging_time", record.Time).
		Uint64("go_logging_record_id", record.Id)
	e.Msg(record.Message())
	return nil
}

func (l *goLoggingBackend) ResetLevel(module string) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	delete(l.levels, module)
}

func (l *goLoggingBackend) DefaultLevel() logging.Level {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.defaultLevel
}

func (l *goLoggingBackend) SetDefaultLevel(level logging.Level) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.defaultLevel = level
}

// GoLoggingBackend is a github.com/whyrusleeping/go-logging logger backend
// which forwards messages to the zerolog logger.
var GoLoggingBackend = newGoLoggingBackend()
