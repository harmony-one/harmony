package consensus

// GetConsensusPhase returns the current phase of the consensus
func (c *Consensus) GetConsensusPhase() string {
	return c.phase.String()
}

// GetConsensusMode returns the current mode of the consensus
func (c *Consensus) GetConsensusMode() string {
	return c.current.mode.String()
}

// GetCurViewID returns the current view ID of the consensus
func (c *Consensus) GetCurViewID() uint64 {
	return c.current.GetCurViewID()
}

// GetViewChangingID returns the current view changing ID of the consensus
func (c *Consensus) GetViewChangingID() uint64 {
	return c.current.GetViewChangingID()
}
