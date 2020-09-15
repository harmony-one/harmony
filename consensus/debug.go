package consensus

// GetConsensusPhase ..
func (c *Consensus) GetConsensusPhase() string {
	return c.phase.String()
}

// GetConsensusMode ..
func (c *Consensus) GetConsensusMode() string {
	return c.current.mode.String()
}

// GetConsensusCurViewID ..
func (c *Consensus) GetConsensusCurViewID() uint64 {
	return c.current.GetViewID()
}

// GetConsensusViewID ..
func (c *Consensus) GetConsensusViewID() uint64 {
	return c.viewID
}
