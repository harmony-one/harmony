package streammanager

import (
	"container/list"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/whyrusleeping/timecache"
)

const (
	coolDownPeriod = 1 * time.Minute
)

type coolDownCache struct {
	timeCache *timecache.TimeCache
}

func newCoolDownCache() *coolDownCache {
	tl := timecache.NewTimeCache(coolDownPeriod)
	return &coolDownCache{
		timeCache: tl,
	}
}

// Has check and add the peer ID to the cache
func (cache *coolDownCache) Has(id peer.ID) bool {
	has := cache.timeCache.Has(string(id))
	if !has {
		cache.timeCache.Add(string(id))
	}
	return has
}

// Reset the cool down cache
func (cache *coolDownCache) Reset() {
	cache.timeCache.Q = list.New()
	cache.timeCache.M = make(map[string]time.Time)
}
