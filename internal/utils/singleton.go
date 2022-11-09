/* This module keeps all struct used as singleton */

package utils

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
)

var (
	// Validator ID
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
	zeroLogger      *zerolog.Logger
	zeroLoggerLevel = zerolog.Disabled

	onceForSampleLogger sync.Once
	sampledLogger       *zerolog.Logger
)

// SetLogContext used to print out loggings of node with port and ip.
// Every instance (node, etc..) needs to set this for logging.
func SetLogContext(_port, _ip string) {
	port = _port
	ip = _ip
	setZeroLogContext(_ip, _port)
}

// SetLogVerbosity specifies the verbosity of global logger
func SetLogVerbosity(verbosity log.Lvl) {
	logVerbosity = verbosity
	if glogger != nil {
		glogger.Verbosity(logVerbosity)
	}
	updateZeroLogLevel(int(logVerbosity))
}

// AddLogFile creates a StreamHandler that outputs JSON logs
// into rotating files with specified max file size and storing at
// max rotateCount files
func AddLogFile(filepath string, maxSize int, rotateCount int, rotateMaxAge int) {
	setZeroLoggerFileOutput(filepath, maxSize, rotateCount, rotateMaxAge)
}

// AddLogHandler add a log handler
func AddLogHandler(handler log.Handler) {
	logHandlers = append(logHandlers, handler)
	if glogger != nil {
		multiHandler := log.MultiHandler(logHandlers...)
		glogger.SetHandler(multiHandler)
	}
}

// GetLogInstance returns logging singleton.
func GetLogInstance() log.Logger {
	onceForLog.Do(func() {
		writer := diode.NewWriter(os.Stdout, 1000, 10*time.Millisecond, func(missed int) {
			fmt.Printf("Logger Dropped %d messages", missed)
		})
		ostream := log.StreamHandler(writer, log.TerminalFormat(false))
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
func setZeroLoggerFileOutput(filepath string, maxSize int, rotateCount int, rotateMaxAge int) error {
	dir := path.Dir(filepath)
	filename := path.Base(filepath)

	// Initialize ZeroLogger if it hasn't been already
	// TODO: zerolog filename prefix can be removed once all loggers
	// has been replaced
	childLogger := Logger().Output(&lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/zerolog-%s", dir, filename),
		MaxSize:    maxSize,
		MaxBackups: rotateCount,
		MaxAge:     rotateMaxAge,
		Compress:   true,
	})
	zeroLogger = &childLogger

	return nil
}

const (
	dataScienceTopic = "ds"
)

func ds() *zerolog.Logger {
	logger := Logger().With().
		Str("log-topic", dataScienceTopic).
		Timestamp().
		Logger()
	return &logger
}

// AnalysisStart ..
func AnalysisStart(name string, more ...interface{}) {
	ds().Debug().Msgf("ds-%s-start %s", name, fmt.Sprint(more...))
}

// AnalysisEnd ..
func AnalysisEnd(name string, more ...interface{}) {
	ds().Debug().Msgf("ds-%s-end %s", name, fmt.Sprint(more...))
}

func init() {
	zeroLogger = Logger()
}

// Logger returns a zerolog.Logger singleton
func Logger() *zerolog.Logger {
	if zeroLogger == nil {
		zerolog.TimeFieldFormat = time.RFC3339Nano
		writer := diode.NewWriter(os.Stderr, 1000, 10*time.Millisecond, func(missed int) {
			fmt.Printf("Logger Dropped %d messages", missed)
		})
		logger := zerolog.New(zerolog.ConsoleWriter{Out: writer}).
			Level(zeroLoggerLevel).
			With().
			Caller().
			Timestamp().
			Logger()
		zeroLogger = &logger
	}
	return zeroLogger
}

// SampledLogger returns a sampled zerolog singleton to be used in criticial path like p2p message handling
func SampledLogger() *zerolog.Logger {
	// Will let 3 log messages per period of 1 second.
	// Over 3 log messages, 1 every 100 messages are logged.
	onceForSampleLogger.Do(func() {
		sLog := zeroLogger.Sample(
			&zerolog.BurstSampler{
				Burst:       3,
				Period:      1 * time.Second,
				NextSampler: &zerolog.BasicSampler{N: 100},
			},
		)
		sampledLogger = &sLog
	})

	return sampledLogger
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

// GetPort is useful for debugging, returns `--port` flag provided to executable.
func GetPort() int {
	ok := false
	for _, x := range os.Args {
		if x == "--port" {
			ok = true
			continue
		}
		if ok {
			rs, _ := strconv.ParseInt(x, 10, 64)
			return int(rs)
		}
	}
	return 0
}
