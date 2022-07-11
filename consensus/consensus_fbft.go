package consensus

import "sync"

type LockedFBFTPhase struct {
	mu    sync.Mutex
	phase FBFTPhase
}

func NewLockedFBFTPhase(initialPhrase FBFTPhase) *LockedFBFTPhase {
	return &LockedFBFTPhase{
		phase: initialPhrase,
	}
}

func (a *LockedFBFTPhase) Set(phrase FBFTPhase) {
	a.mu.Lock()
	a.phase = phrase
	a.mu.Unlock()
}

func (a *LockedFBFTPhase) Get() FBFTPhase {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.phase
}

func (a *LockedFBFTPhase) String() string {
	return a.Get().String()
}
