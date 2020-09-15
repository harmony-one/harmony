package consensus

// GetConsensusPhase ..
func (c *Consensus) GetConsensusPhase() string {
	return c.phase.String()
}

// GetConsensusMode ..
func (c *Consensus) GetConsensusMode() string {
	return c.current.mode.String()
}

// GetCurViewID ..
func (c *Consensus) GetCurViewID() uint64 {
	return c.current.GetCurViewID()
}

// GetViewID ..
func (c *Consensus) GetViewChangingID() uint64 {
	return c.current.GetViewChangingID()
}
