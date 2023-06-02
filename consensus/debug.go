package consensus

// GetConsensusPhase returns the current phase of the consensus.
func (consensus *Consensus) GetConsensusPhase() string {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.getConsensusPhase()
}

// GetConsensusPhase returns the current phase of the consensus.
func (consensus *Consensus) getConsensusPhase() string {
	return consensus.phase.String()
}

// GetConsensusMode returns the current mode of the consensus
func (consensus *Consensus) GetConsensusMode() string {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.current.mode.String()
}

// GetCurBlockViewID returns the current view ID of the consensus
func (consensus *Consensus) GetCurBlockViewID() uint64 {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.getCurBlockViewID()
}

// GetCurBlockViewID returns the current view ID of the consensus
func (consensus *Consensus) getCurBlockViewID() uint64 {
	return consensus.current.GetCurBlockViewID()
}

// GetViewChangingID returns the current view changing ID of the consensus
func (consensus *Consensus) GetViewChangingID() uint64 {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.current.GetViewChangingID()
}

// GetViewChangingID returns the current view changing ID of the consensus
func (consensus *Consensus) getViewChangingID() uint64 {
	return consensus.current.GetViewChangingID()
}
