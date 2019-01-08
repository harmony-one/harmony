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

func GetLogInstance() log.Logger {
	onceForLog.Do(func() {
		logInstance = log.New()
	})
	return logInstance
}
