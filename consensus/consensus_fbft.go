package consensus

type LockedFBFTPhase struct {
	phase FBFTPhase
}

func NewLockedFBFTPhase(initialPhrase FBFTPhase) *LockedFBFTPhase {
	return &LockedFBFTPhase{
		phase: initialPhrase,
	}
}

func (a *LockedFBFTPhase) Set(phrase FBFTPhase) {
	a.phase = phrase
}

func (a *LockedFBFTPhase) Get() FBFTPhase {
	return a.phase
}

func (a *LockedFBFTPhase) String() string {
	return a.Get().String()
}
