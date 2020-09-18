package consensus

// GetConsensusPhase returns the current phase of the consensus
func (consensus *Consensus) GetConsensusPhase() string {
	return consensus.phase.String()
}

// GetConsensusMode returns the current mode of the consensus
func (consensus *Consensus) GetConsensusMode() string {
	return consensus.current.mode.String()
}

<<<<<<< HEAD
// GetCurBlockViewID returns the current view ID of the consensus
func (c *Consensus) GetCurBlockViewID() uint64 {
	return c.current.GetCurBlockViewID()
=======
// GetCurViewID returns the current view ID of the consensus
func (consensus *Consensus) GetCurViewID() uint64 {
	return consensus.current.GetCurViewID()
>>>>>>> [consensus] add leader key check. Move quorum check logic after tryCatchup and can spin state sync
}

// GetViewChangingID returns the current view changing ID of the consensus
func (consensus *Consensus) GetViewChangingID() uint64 {
	return consensus.current.GetViewChangingID()
}

// GetBlockNum return the current blockNum of the consensus struct
func (c *Consensus) GetBlockNum() uint64 {
	return c.blockNum
}
