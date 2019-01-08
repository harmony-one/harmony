/* This module keeps all struct used as singleton */

package utils

import (
	"sync"
	"sync/atomic"

	"github.com/harmony-one/harmony/log"
)

// UniqueValidatorID defines the structure of unique validator ID
type UniqueValidatorID struct {
	uniqueID uint32
}

var instance *UniqueValidatorID
var logInstance log.Logger
var once sync.Once
var onceForLog sync.Once

// GetUniqueValidatorIDInstance returns a singleton instance
func GetUniqueValidatorIDInstance() *UniqueValidatorID {
	once.Do(func() {
		instance = &UniqueValidatorID{
			uniqueID: 0,
		}
	})
	return instance
}

// GetUniqueID returns a unique ID and increment the internal variable
func (s *UniqueValidatorID) GetUniqueID() uint32 {
	return atomic.AddUint32(&s.uniqueID, 1)
}

func GetLogInstance() log.Logger {
	onceForLog.Do(func() {
		logInstance = log.New()
	})
	return logInstance
}
