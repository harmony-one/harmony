package locker

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type RWLocker interface {
	Lock()
	Unlock()
	RUnlock()
	RLock()
}

type RWLockerImpl struct {
	duration time.Duration
	called   chan string
	mu       sync.RWMutex
}

func NewRwLocker(duration time.Duration) *RWLockerImpl {
	return &RWLockerImpl{
		duration: duration,
		called:   make(chan string, 1),
	}
}

func (r *RWLockerImpl) Lock() {
	stack := getStack()
	select {
	case r.called <- stack:
	case <-time.After(r.duration):
		panic(stack)
	}
	r.mu.Lock()
}

func (r *RWLockerImpl) Unlock() {
	r.mu.Unlock()
	select {
	case <-r.called:
	case <-time.After(r.duration):
		panic(getStack())
	}
}

func (r *RWLockerImpl) RUnlock() {
	r.mu.RUnlock()
}

func (r *RWLockerImpl) RLock() {
	stack := getStack()
	ch := make(chan struct{})
	go func() {
		select {
		case <-ch:
			return
		case <-time.After(r.duration):
			called := <-r.called
			panic(fmt.Sprintf("try: %s Locked %s", stack, called))
		}
	}()
	r.mu.RLock()
	close(ch)
}

func getStack() string {
	out := ""
	for i := 2; i < 20; i++ {
		pc, file, no, ok := runtime.Caller(i)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			out += fmt.Sprintf("%s %s:%d\n", details.Name(), file, no)
		}
	}
	return out
}
