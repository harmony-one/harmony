package rate

import (
	"time"

	"golang.org/x/time/rate"
)

const (
	defCheckInt    = 1 * time.Minute // Check and evict every 1 minute
	defMinEvictDur = 3 * time.Minute // limiter evicted must be inactive for 3 minutes
	defCapacity    = 100             // Evict old IPs if size of IP list grows larger than 100
)

type (
	// Config is the config for Limiter
	Config struct {
		Capacity    *int
		CheckInt    *time.Duration
		MinEvictDur *time.Duration
		Whitelist   []string
	}

	// internal config
	configInt struct {
		limit       rate.Limit
		burst       int
		capacity    int
		checkInt    time.Duration
		minEvictDur time.Duration
		whitelist   map[string]struct{}
	}
)

func toConfigInt(limit rate.Limit, burst int, c *Config) configInt {
	ci := configInt{
		limit:       limit,
		burst:       burst,
		capacity:    defCapacity,
		checkInt:    defCheckInt,
		minEvictDur: defMinEvictDur,
		whitelist:   make(map[string]struct{}),
	}
	if c == nil {
		return ci
	}
	if c.Capacity != nil && *c.Capacity > 0 {
		ci.capacity = *c.Capacity
	}
	if c.CheckInt != nil {
		ci.checkInt = *c.CheckInt
	}
	if c.MinEvictDur != nil {
		ci.minEvictDur = *c.MinEvictDur
	}
	return ci
}
