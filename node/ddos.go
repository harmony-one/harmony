package node

import (
	"time"

	"github.com/harmony-one/harmony/internal/rate"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const (
	// TODO: make these parameters configurable
	pubSubRateLimit  = 4
	pubSubBurstLimit = 40

	// When a node is bootstrapped, it will be flooded with some pub-sub message from the past.
	// Ease the rate limit at the initial bootstrap.
	rateLimiterEasyPeriod = 30 * time.Second

	blacklistExpiry = 5 * time.Minute
)

var (
	// TODO: Add trusted peer id here (internal explorer, leader candidates, e.t.c)
	// TODO: make these parameters configurable
	whitelist []string
)

func newBlackList() libp2p_pubsub.Blacklist {
	blacklist, _ := libp2p_pubsub.NewTimeCachedBlacklist(blacklistExpiry)
	return blacklist
}

func newRateLimiter() rate.IDLimiter {
	return rate.NewLimiterPerID(pubSubRateLimit, pubSubBurstLimit, &rate.Config{
		Whitelist: whitelist,
	})
}
