/* This module keeps all struct used as singleton */

package utils

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
)

var (
	// Validator ID
	validatorIDInstance      *UniqueValidatorID
	onceForUniqueValidatorID sync.Once
	// Global port and ip for logging.
	port string
	ip   string
	// Logging
	logInstance  log.Logger
	glogger      *log.GlogHandler // top-level handler
	logHandlers  []log.Handler    // sub handlers of glogger
	logVerbosity log.Lvl
	onceForLog   sync.Once

	// ZeroLog
	zeroLoggerOnce  sync.Once
	zeroLogger      *zerolog.Logger
	zeroLoggerLevel zerolog.Level = zerolog.Disabled
)

// SetLogContext used to print out loggings of node with port and ip.
// Every instance (node, txgen, etc..) needs to set this for logging.
func SetLogContext(_port, _ip string) {
	port = _port
	ip = _ip
	setZeroLogContext(_port, _ip)
}

// SetLogVerbosity specifies the verbosity of global logger
func SetLogVerbosity(verbosity log.Lvl) {
	logVerbosity = verbosity
	if glogger != nil {
		glogger.Verbosity(logVerbosity)
	}
	updateZeroLogLevel(int(verbosity))
}

// AddLogFile creates a StreamHandler that outputs JSON logs
// into rotating files with specified max file size
func AddLogFile(filepath string, maxSize int) {
	AddLogHandler(log.StreamHandler(&lumberjack.Logger{
		Filename: filepath,
		MaxSize:  maxSize,
		Compress: true,
	}, log.JSONFormat()))

	setZeroLoggerFileOutput(filepath, maxSize)
}

// AddLogHandler add a log handler
func AddLogHandler(handler log.Handler) {
	logHandlers = append(logHandlers, handler)
	if glogger != nil {
		multiHandler := log.MultiHandler(logHandlers...)
		glogger.SetHandler(multiHandler)
	}
}

// UniqueValidatorID defines the structure of unique validator ID
type UniqueValidatorID struct {
	uniqueID uint32
}

// GetUniqueValidatorIDInstance returns a singleton instance
func GetUniqueValidatorIDInstance() *UniqueValidatorID {
	onceForUniqueValidatorID.Do(func() {
		validatorIDInstance = &UniqueValidatorID{
			uniqueID: 0,
		}
	})
	return validatorIDInstance
}

// GetUniqueID returns a unique ID and increment the internal variable
func (s *UniqueValidatorID) GetUniqueID() uint32 {
	return atomic.AddUint32(&s.uniqueID, 1)
}

// GetLogInstance returns logging singleton.
func GetLogInstance() log.Logger {
	onceForLog.Do(func() {
		ostream := log.StreamHandler(io.Writer(os.Stdout), log.TerminalFormat(false))
		logHandlers = append(logHandlers, ostream)
		multiHandler := log.MultiHandler(logHandlers...)
		glogger = log.NewGlogHandler(multiHandler)
		glogger.Verbosity(logVerbosity)
		logInstance = log.New("port", port, "ip", ip)
		logInstance.SetHandler(glogger)
		log.Root().SetHandler(glogger)
	})
	return logInstance
}

// ZeroLog
func setZeroLogContext(port string, ip string) {
	childLogger := Logger().
		With().
		Str("port", port).
		Str("ip", ip).
		Logger()
	zeroLogger = &childLogger
}

// SetZeroLoggerFileOutput sets zeroLogger's output stream
// to destinated filepath with log file rotation.
func setZeroLoggerFileOutput(filepath string, maxSize int) error {
	dir := path.Dir(filepath)
	filename := path.Base(filepath)

	// Initialize ZeroLogger if it hasn't been already
	// TODO: zerolog filename prefix can be removed once all loggers
	// has been replaced
	childLogger := Logger().Output(&lumberjack.Logger{
		Filename: fmt.Sprintf("%s/zerolog-%s", dir, filename),
		MaxSize:  maxSize,
		Compress: true,
	})
	zeroLogger = &childLogger

	return nil
}

// ZerologCallerHook is a hook that adds caller file/line/function info.
// It differs from zerolog's own Caller method in three ways: It logs filename
// and line number separately in caller_file (string) and caller_line (int)
// fields, it logs only basename of the source file and not the full path,
// closing a builder host information leak, and it includes a package-qualified
// caller information in caller_func (string).
type ZerologCallerHook struct {
	// Number of extra frames to skip.  Use from logging bridges.
	Skip int
}

// Run adds caller file/line/function info to the given event.
func (h ZerologCallerHook) Run(e *zerolog.Event, level zerolog.Level, message string) {
	pcs := make([]uintptr, 1)
	// 4 == runtime.Callers()+ZerologCallerHook.Run()+Event.msg()+Event.Msg()
	if runtime.Callers(4+h.Skip, pcs) == 0 || pcs[0] == 0 {
		return
	}
	frame, _ := runtime.CallersFrames(pcs).Next()
	var fun, pkg string
	s := frame.Function
	slash := strings.LastIndexByte(s, '/')
	if slash >= 0 {
		s = s[slash+1:]
	}
	dot := strings.IndexByte(s, '.')
	switch {
	case dot >= 0:
		fun = s[dot+1:]
		pkg = frame.Function[:len(frame.Function)-len(fun)-1]
	case slash >= 0:
		fun = ""
		pkg = frame.Function
	default:
		fun = frame.Function
		pkg = ""
	}
	e.Str("caller_file", path.Base(frame.File))
	e.Int("caller_line", frame.Line)
	e.Str("caller_pkg", pkg)
	e.Str("caller_func", fun)
}

// RawLogger returns the zerolog.Logger singleton without caller hook.
func RawLogger() *zerolog.Logger {
	zeroLoggerOnce.Do(func() {
		zerolog.TimeFieldFormat = "2006-01-02T15:04:05.999999999Z"
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
			Level(zeroLoggerLevel).
			With().
			Timestamp().
			Logger()
		zeroLogger = &logger
	})
	return zeroLogger
}

// Logger returns the zerolog.Logger singleton with caller hook.
func Logger() *zerolog.Logger {
	logger := RawLogger().Hook(ZerologCallerHook{})
	return &logger
}

func updateZeroLogLevel(level int) {
	switch level {
	case 0:
		zeroLoggerLevel = zerolog.Disabled
	case 1:
		zeroLoggerLevel = zerolog.ErrorLevel
	case 2:
		zeroLoggerLevel = zerolog.WarnLevel
	case 3:
		zeroLoggerLevel = zerolog.InfoLevel
	default:
		zeroLoggerLevel = zerolog.DebugLevel
	}
	childLogger := Logger().Level(zeroLoggerLevel)
	zeroLogger = &childLogger
}
