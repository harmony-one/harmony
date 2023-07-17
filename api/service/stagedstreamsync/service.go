package stagedstreamsync

import (
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/p2p"
)

// StagedStreamSyncService is simply a adapter of downloaders, which support block synchronization
type StagedStreamSyncService struct {
	Downloaders *Downloaders
}

// NewService creates a new downloader service
func NewService(host p2p.Host, bcs []core.BlockChain, consensus *consensus.Consensus, config Config, dbDir string) *StagedStreamSyncService {
	return &StagedStreamSyncService{
		Downloaders: NewDownloaders(host, bcs, consensus, dbDir, config),
	}
}

// Start starts the service
func (s *StagedStreamSyncService) Start() error {
	s.Downloaders.Start()
	return nil
}

// Stop stops the service
func (s *StagedStreamSyncService) Stop() error {
	s.Downloaders.Close()
	return nil
}
