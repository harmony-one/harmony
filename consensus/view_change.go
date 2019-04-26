package consensus

import "sync"

// PbftPhase  PBFT phases: pre-prepare, prepare and commit
type PbftPhase int

// Enum for PbftPhase
const (
	Annonce PbftPhase = iota
	Prepare
	Commit
	Finish
)

// Mode determines whether a node is in normal or viewchanging mode
type Mode int

// Enum for node Mode
const (
	Normal Mode = iota
	ViewChanging
)

// PbftMode contains mode and consensusID of viewchanging
type PbftMode struct {
	mode        Mode
	consensusID uint32
	mux         sync.Mutex
}

// Mode return the current node mode
func (pm *PbftMode) Mode() Mode {
	return pm.mode
}

// SetMode set the node mode as required
func (pm *PbftMode) SetMode(m Mode) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.mode = m
}

// ConsensusID return the current viewchanging id
func (pm *PbftMode) ConsensusID() uint32 {
	return pm.consensusID
}

// SetConsensusID sets the viewchanging id accordingly
func (pm *PbftMode) SetConsensusID(consensusID uint32) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.consensusID = consensusID
}

// startViewChange start a new view change
func (consensus *Consensus) startViewChange(consensusID uint32) {
	consensus.mode.SetMode(ViewChanging)
	consensus.mode.SetConsensusID(consensusID)
	// TODO (cm): implement the actual logic
}
