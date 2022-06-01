package rate

import (
	"container/list"
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type (
	limiterPerID struct {
		evictList *list.List
		items     map[string]*list.Element

		lock sync.Mutex
		t    timer
		c    configInt
	}

	entry struct {
		id       string
		limiter  *rate.Limiter
		lastCall time.Time
	}
)

// NewLimiterPerID creates a limit limiter based on ID
func NewLimiterPerID(rate rate.Limit, burst int, c *Config) *limiterPerID {
	lpi := &limiterPerID{
		evictList: list.New(),
		items:     make(map[string]*list.Element),
		t:         timerImpl{},
		c:         toConfigInt(rate, burst, c),
	}
	go lpi.maintainLoop()
	return lpi
}

// AllowN return whether a request of size n is allowed by IP
func (lpi *limiterPerID) AllowN(id string, n int) bool {
	if lpi.isWhitelisted(id) {
		return true
	}
	lmt := lpi.getLimiterByID(id)
	return lmt.AllowN(lpi.t.now(), n)
}

// WaitN waits until the n token is granted by IP
func (lpi *limiterPerID) WaitN(ctx context.Context, id string, n int) error {
	if lpi.isWhitelisted(id) {
		return nil
	}
	lmt := lpi.getLimiterByID(id)
	return lmt.WaitN(ctx, n)
}

func (lpi *limiterPerID) getLimiterByID(id string) *rate.Limiter {
	lpi.lock.Lock()
	defer lpi.lock.Unlock()

	elem, ok := lpi.items[id]
	if !ok || elem == nil {
		lmt := rate.NewLimiter(lpi.c.limit, lpi.c.burst)
		ent := &entry{id: id, limiter: lmt, lastCall: lpi.t.now()}
		elem := lpi.evictList.PushFront(ent)
		lpi.items[id] = elem
		return lmt
	}
	ent := elem.Value.(*entry)
	lpi.evictList.MoveToFront(elem)
	ent.lastCall = lpi.t.now()
	return ent.limiter
}

func (lpi *limiterPerID) maintainLoop() {
	t := lpi.t.newTicker(lpi.c.checkInt)
	defer t.Stop()

	for range t.C {
		lpi.maintain()
	}
}

func (lpi *limiterPerID) maintain() {
	lpi.lock.Lock()
	defer lpi.lock.Unlock()

	for lpi.evictList.Len() > lpi.c.capacity {
		evicted := lpi.evictLast(func(ent *entry) bool {
			return lpi.t.since(ent.lastCall) > lpi.c.minEvictDur
		})
		if !evicted {
			break
		}
	}
}

func (lpi *limiterPerID) evictLast(isEvict func(ent *entry) bool) bool {
	elem := lpi.evictList.Back()
	evict := elem.Value.(*entry) // not nil ensured
	if isEvict(evict) {
		delete(lpi.items, evict.id)
		lpi.evictList.Remove(elem)
		return true
	}
	return false
}

func (lpi *limiterPerID) isWhitelisted(id string) bool {
	_, ok := lpi.c.whitelist[id]
	return ok
}
