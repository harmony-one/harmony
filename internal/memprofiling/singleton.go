package memprofiling

import "sync"

var singleton *MemProfiling
var once sync.Once

// GetMemProfiling returns a pointer of MemProfiling.
func GetMemProfiling() *MemProfiling {
	once.Do(func() {
		singleton = &MemProfiling{}
	})
	return singleton
}
