package node

import (
	"sync"
	"time"

	"github.com/harmony-one/abool"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/time/rate"
)

const (
	pubSubRateLimit  = 5  // 5 messages per second
	pubSubBurstLimit = 20 // 20 messages at burst

	// When a node is bootstrapped, it will be flooded with some pub-sub message from the past.
	// Ease the rate limit at the initial bootstrap.
	rateLimiterEasyPeriod = 5 * time.Second
)

// Preconfigured internal nodes
// TODO: replace all internal node peer ID here
var trustedPeers []libp2p_peer.ID

func getDefaultTrustedPeerMap() map[libp2p_peer.ID]struct{} {
	peerMap := make(map[libp2p_peer.ID]struct{})
	for _, peer := range trustedPeers {
		peerMap[peer] = struct{}{}
	}
	return peerMap
}

// TODO: maintain a timed list to avoid memory leak (growing forever)
// TODO: make each message with a weight
type psRateLimiter struct {
	limiters     map[libp2p_peer.ID]*rate.Limiter
	trustedPeers map[libp2p_peer.ID]struct{}

	started *abool.AtomicBool
	lock    sync.Mutex
}

func newPSRateLimiter() *psRateLimiter {
	return &psRateLimiter{
		limiters:     make(map[libp2p_peer.ID]*rate.Limiter),
		trustedPeers: getDefaultTrustedPeerMap(),

		started: abool.NewBool(false),
	}
}

// Start start the rate limiter. Before start, the rate limiter does not actually limit the rate.
// This is for the bootstrap grace period.
func (rl *psRateLimiter) Start() {
	rl.started.Set()
}

func (rl *psRateLimiter) isStarted() bool {
	return rl.started.IsSet()
}

// Allow returns whether a pub-sub message is allowed to be processed
func (rl *psRateLimiter) Allow(id libp2p_peer.ID) bool {
	// skip limiting for bootstrap grace period
	if !rl.isStarted() {
		return true
	}
	rl.lock.Lock()
	defer rl.lock.Unlock()

	if rl.isTrusted(id) {
		return true
	}
	l := rl.getLimiter(id)
	return l.Allow()
}

func (rl *psRateLimiter) getLimiter(id libp2p_peer.ID) *rate.Limiter {
	if l, ok := rl.limiters[id]; ok && l != nil {
		return l
	}
	return rl.addNewLimiter(id)
}

func (rl *psRateLimiter) addNewLimiter(id libp2p_peer.ID) *rate.Limiter {
	l := rate.NewLimiter(pubSubRateLimit, pubSubBurstLimit)
	rl.limiters[id] = l
	return l
}

// AddTrustedPeer add a trust peer. Potentially used for RPC calls to add customized trust peers.
func (rl *psRateLimiter) AddTrustedPeer(id libp2p_peer.ID) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	rl.trustedPeers[id] = struct{}{}
}

func (rl *psRateLimiter) isTrusted(id libp2p_peer.ID) bool {
	_, ok := rl.trustedPeers[id]
	return ok
}

type blacklist struct {
	bl           libp2p_pubsub.Blacklist
	trustedPeers map[libp2p_peer.ID]struct{} // Currently fixed after initialization
}

func newBlacklist(raw libp2p_pubsub.Blacklist) *blacklist {
	return &blacklist{
		bl:           raw,
		trustedPeers: getDefaultTrustedPeerMap(),
	}
}

func (bl *blacklist) Add(id libp2p_peer.ID) bool {
	_, trusted := bl.trustedPeers[id]
	if trusted {
		return false
	}
	return bl.bl.Add(id)
}

func (bl *blacklist) Contains(id libp2p_peer.ID) bool {
	return bl.bl.Contains(id)
}
