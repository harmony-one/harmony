package utils

import "sync"

type CrosslinkStorage struct {
	mu   sync.Mutex
	data [4]uint64
}

func NewCrosslinkStorage() *CrosslinkStorage {
	return &CrosslinkStorage{
		mu:   sync.Mutex{},
		data: [4]uint64{},
	}
}

func (a *CrosslinkStorage) Set(shardID uint32, value uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.data[shardID] = value
}

func (a *CrosslinkStorage) Get(shardID uint32) uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data[shardID]
}
