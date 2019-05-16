/* This module keeps all struct used as singleton */

package utils

import (
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
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
)

// SetLogContext used to print out loggings of node with port and ip.
// Every instance (node, txgen, etc..) needs to set this for logging.
func SetLogContext(_port, _ip string) {
	port = _port
	ip = _ip
}

// SetLogVerbosity specifies the verbosity of global logger
func SetLogVerbosity(verbosity log.Lvl) {
	logVerbosity = verbosity
	if glogger != nil {
		glogger.Verbosity(logVerbosity)
	}
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
