/* This module keeps all struct used as singleton */

package utils

import (
	"sync"
	"sync/atomic"
)

type UniqueValidatorId struct {
	uniqueId uint32
}

var instance *UniqueValidatorId
var once sync.Once

func GetUniqueValidatorIdInstance() *UniqueValidatorId {
	once.Do(func() {
		instance = &UniqueValidatorId{
			uniqueId: 0,
		}
	})
	return instance
}

func (s *UniqueValidatorId) GetUniqueId() uint32 {
	return atomic.AddUint32(&s.uniqueId, 1)
}
