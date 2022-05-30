package consensus

import "sync"

type LockedFBFTPhase struct {
	mu     sync.Mutex
	phrase FBFTPhase
}

func NewLockedFBFTPhase(initialPhrase FBFTPhase) *LockedFBFTPhase {
	return &LockedFBFTPhase{
		phrase: initialPhrase,
	}
}

func (a *LockedFBFTPhase) Set(phrase FBFTPhase) {
	a.mu.Lock()
	a.phrase = phrase
	a.mu.Unlock()
}

func (a *LockedFBFTPhase) Get() FBFTPhase {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.phrase
}

func (a *LockedFBFTPhase) String() string {
	return a.Get().String()
}
