package consensus

// GetConsensusPhase returns the current phase of the consensus
func (c *Consensus) GetConsensusPhase() string {
	return c.phase.String()
}

// GetConsensusMode returns the current mode of the consensus
func (c *Consensus) GetConsensusMode() string {
	return c.current.mode.String()
}

// GetCurBlockViewID returns the current view ID of the consensus
func (c *Consensus) GetCurBlockViewID() uint64 {
	return c.current.GetCurBlockViewID()
}

// GetLatestCommittedBlockViewID returns the latest committed block's viewID
func (c *Consensus) GetLatestCommittedBlockViewID() uint64 {
	latestBlock := c.Blockchain.CurrentBlock()
	if latestBlock == nil {
		return c.current.GetCurBlockViewID() - 1
	}
	return latestBlock.Header().ViewID().Uint64()
}

// GetViewChangingID returns the current view changing ID of the consensus
func (c *Consensus) GetViewChangingID() uint64 {
	return c.current.GetViewChangingID()
}

// GetBlockNum return the current blockNum of the consensus struct
func (c *Consensus) GetBlockNum() uint64 {
	return c.blockNum
}
