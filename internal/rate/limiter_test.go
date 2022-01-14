package rate

import (
	"container/list"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

var (
	id1 = "id1"
	id2 = "id2"
)

func TestLimiterPerID_AllowN(t *testing.T) {
	tests := []struct {
		steps []testStep
	}{
		{
			steps: []testStep{
				{id1, 1, true},
				{id1, 1, true},
				{id1, 1, false},
			},
		},
		{
			steps: []testStep{
				{id1, 1, true},
				{id2, 1, true},
				{id1, 1, true},
				{id1, 1, false},
				{id2, 1, true},
			},
		},
		{
			steps: []testStep{
				{id1, 2, true},
				{id1, 1, false},
			},
		},
		{
			steps: []testStep{
				{id1, 3, false},
				{id1, 3, false},
			},
		},
	}
	for i, test := range tests {
		lpi := newTestLimiter()
		steps := test.steps
		for j, step := range steps {
			res := lpi.AllowN(step.id, step.n)
			if res != step.exp {
				t.Errorf("Test %v/%v unexpected %v/%v", i, j, res, step.exp)
			}
			lpi.t.(*testTimer).tick()
		}
	}
}

func TestLimiterPerID_maintain(t *testing.T) {
	lpi := newTestLimiter()
	lpi.c.capacity = 0
	lpi.AllowN(id1, 2)
	lpi.t.(*testTimer).tick()
	lpi.AllowN(id2, 2)
	lpi.t.(*testTimer).tick()

	lpi.maintain()
	if lpi.evictList.Len() != 1 || len(lpi.items) != 1 {
		t.Errorf("unexpected number. Expect 1")
	}
	for id := range lpi.items {
		if id != id2 {
			t.Errorf("unexpected id. Expect id2")
		}
	}
	lpi.t.(*testTimer).tick()
	lpi.maintain()
	if lpi.evictList.Len() != 0 || len(lpi.items) != 0 {
		t.Errorf("unexpected number. Expect 0")
	}
}

func TestLimiterPerID_maintain_largeCap(t *testing.T) {
	lpi := newTestLimiter()
	lpi.c.capacity = 10
	lpi.AllowN(id1, 2)
	lpi.t.(*testTimer).tick()
	lpi.AllowN(id2, 2)
	lpi.t.(*testTimer).tick()
	lpi.maintain()

	if lpi.evictList.Len() != 2 || len(lpi.items) != 2 {
		t.Errorf("unexpected number")
	}
}

func TestLimiterPerID_maintain_largeMinDur(t *testing.T) {
	lpi := newTestLimiter()
	lpi.c.minEvictDur = 5 * time.Second
	lpi.AllowN(id1, 2)
	lpi.t.(*testTimer).tick()
	lpi.AllowN(id2, 2)
	lpi.t.(*testTimer).tick()
	lpi.maintain()

	if lpi.evictList.Len() != 2 || len(lpi.items) != 2 {
		t.Errorf("unexpected number")
	}
}

type testStep struct {
	id  string
	n   int
	exp bool
}

func newTestLimiter() *limiterPerID {
	return &limiterPerID{
		evictList: list.New(),
		items:     make(map[string]*list.Element),
		t:         &testTimer{time.Now()},
		c:         testConfig,
	}
}

var testConfig = configInt{
	limit:       rate.Every(100 * time.Second),
	burst:       2,
	capacity:    1,
	checkInt:    time.Second,
	minEvictDur: 1 * time.Second,
	whitelist:   make(map[string]struct{}),
}

// testTimer will increment 1 sec per call
type testTimer struct {
	c time.Time
}

func (t *testTimer) now() time.Time {
	return t.c
}

func (t *testTimer) tick() {
	t.c = t.c.Add(time.Second)
}

func (t *testTimer) since(t2 time.Time) time.Duration {
	return t.c.Sub(t2)
}

func (t *testTimer) newTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(time.Nanosecond)
}
