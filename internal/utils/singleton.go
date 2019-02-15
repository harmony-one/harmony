/* This module keeps all struct used as singleton */

package utils

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
)

// Global Port and IP for logging.
var (
	Port string
	IP   string
	// Global Variable to use libp2p for networking
	// FIXME: this is a temporary hack, once we totally switch to libp2p
	// this variable shouldn't be used
	UseLibP2P bool
)

// SetPortAndIP used to print out loggings of node with Port and IP.
// Every instance (node, txgen, etc..) needs to set this for logging.
func SetPortAndIP(port, ip string) {
	Port = port
	IP = ip
}

// UniqueValidatorID defines the structure of unique validator ID
type UniqueValidatorID struct {
	uniqueID uint32
}

var validatorIDInstance *UniqueValidatorID
var logInstance log.Logger
var onceForUniqueValidatorID sync.Once
var onceForLog sync.Once

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
		logInstance = log.New("port", Port, "ip", IP)
	})
	return logInstance
}
