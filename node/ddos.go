package node

import (
	"sync"
	"time"

	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/p2p"
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
var trustedPeers = []string{
	"Qme6wYYAXd53FCjB5vQdcDUPuNLsSLDzgU8xUowfgaeyCk",
	"QmYbvZNqVkQK21B75agEYVgKZJXQmEVsq39EfuvMZtme6Q",
	"QmUkHXAxqUSPaZ2SFXoF1bVPS7CGJax1FqiDGbfBPFFGPJ",
	"QmcXfk9W2oEnQWACoFpRaoK9xjrmrtZJPTesJaNqrgc7NF",
	"QmUZ5S9BFA3ZzaVJ1FcTHyhCG8Fjkw1DWms8z6EHKeJYPz",
	"QmZSBKVCY3BQKPhPb6BvwdJgv48PpyxdQKHfWHzSJQERFZ",
	"QmeaCoaVjAgt3i1HUk7WnnGSckKvpxKMApriFvtNvgCQtW",
	"QmQb1ALhFoEfNFWXSDn2QuS19fsCkZxc8qyGMk4mwfbAi3",
	"QmTRcgyARZKvia7NzoQgzQgi8KLjwVCcqRWc4sXmcZ4Xp1",
	"QmQ7fWBuqAmtaG2pozE2n5rPd6bBdvhhneTfeFSbsMSTY9",
	"QmTrCwSG67mHaoJdv4rWPQbTP9H7BWEKLAsZp85XJqJt8t",
	"QmWPgidDQpE8teJtqp7N56bj6HttJgvph4C39UVrtkqRTd",
	"QmSGGt6DRcaJutDci7hP7fmSsgMm61ZbrWnimt7qezpEes",
	"QmV3v6vhGWmsNrrgbDYqi1PEj58M3QwJ2p7TZwNfM3Btq2",
	"QmapawakHjFvzK8jtki2dhn2xsW7sx53FsaVjBk7SDtrVZ",
	"QmXGRV9emo4AZRPqzT2NFJLbkVsfprrGFPMBjyzLTUp2Ne",
	"QmZCEd9N4G38LMeX3FaDHwdtFsc3gsLbdP9ZCf5oGHpVh7",
	"QmfWhSdXijFqnBWVTTuzyh4xQwUmDveBSftSprLe5j1H1S",
	"QmSYHkxMTKXsxrx9Gf53E75K5HorEunHbYG2NehrErfj79",
	"QmVyEaZafK4a2eagrA5XnT3Y6tihM7iuRyUEpnXk8oHNK6",
	"QmWNWwD1MCdy4JJqCBfe4rtsSrYkciF4fjPBrMDFh5QFJ3",
	"QmbxTnbxiiHmuanQuHo25EHFzbJMR1YJWN8mp6bNdKzr2Y",
	"QmNqvByPqNccFGNA4Znt5eKxoeBXtS5VMWwj1UH2FcTMEh",
	"QmXhYD6uVTsCLJq44nJSEBkEHwNHCKoowA3NL9RtYZqigz",
	"QmNkbrtbyJtFZtfdumkJkSzK9oiY1Xu3vVpBMyxXLxKhkp",
	"QmbSFi1zGwyxJ9VuFuzNbvvrytHqMaxyrR6ha5TPmZxWE9",
	"QmRgquDUnwW5rCC49iwore86msVsARRZRdTbdiL3iVzHij",
	"QmTGh5UeGE2EqoEYz3pDKbE1nRSCho3yu4aZBWaCH4VwQ2",
	"QmQPVKmzvt3Ctt4jfpxM1kprtDRS1AjWVPMxLUfEUwv4yG",
	"QmWoW9HzC72Ufvg73tm2iQhUM67H4C48hujMSDLFpey62s",
	"QmdkWrKour5DC8XYFMo7df4THPcw6pcmrvLrcTnGaDehTj",
	"QmS2GkF9yrFD5MMa5XTpSX57iCMSCmUrbgzKzLJUpboRq5",
	"QmefubgDnMmPKYpwpHyS41XXuMEkiKDLvfTpwY6RFEXqCX",
}

func getDefaultTrustedPeerMap() map[string]struct{} {
	peerMap := make(map[string]struct{})
	for _, peer := range trustedPeers {
		peerMap[peer] = struct{}{}
	}
	return peerMap
}

// TODO: maintain a timed list to avoid memory leak (growing forever)
// TODO: make each message with a weight
type psRateLimiter struct {
	limiters     map[libp2p_peer.ID]*rate.Limiter
	trustedPeers map[string]struct{}

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
func (rl *psRateLimiter) AddTrustedPeer(id string) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	rl.trustedPeers[id] = struct{}{}
}

func (rl *psRateLimiter) isTrusted(id libp2p_peer.ID) bool {
	pretty := id.String()
	_, ok := rl.trustedPeers[pretty]
	return ok
}

type blacklist struct {
	bl           libp2p_pubsub.Blacklist
	trustedPeers map[string]struct{} // Currently fixed after initialization
}

// TODO: use hostV2.Blacklist() after this issue (https://github.com/libp2p/go-libp2p-pubsub/issues/426)
//       is fixed.
func newBlacklist() *blacklist {
	raw, _ := libp2p_pubsub.NewTimeCachedBlacklist(p2p.BlacklistExpiry)
	return &blacklist{
		bl:           raw,
		trustedPeers: getDefaultTrustedPeerMap(),
	}
}

func (bl *blacklist) Add(id libp2p_peer.ID) bool {
	_, trusted := bl.trustedPeers[id.String()]
	if trusted {
		return false
	}
	return bl.bl.Add(id)
}

func (bl *blacklist) Contains(id libp2p_peer.ID) bool {
	return bl.bl.Contains(id)
}
