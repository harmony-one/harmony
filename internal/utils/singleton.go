/* This module keeps all struct used as singleton */

package utils

import (
	"sync"
	"sync/atomic"
)

// UniqueValidatorID defines the structure of unique validator ID
type UniqueValidatorID struct {
	uniqueID uint32
}

var instance *UniqueValidatorID
var once sync.Once

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
