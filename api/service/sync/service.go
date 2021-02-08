package sync

import (
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/hmy/downloader"
	"github.com/harmony-one/harmony/p2p"
)

// Service is simply a adapter of downloaders, which support block synchronization
type Service struct {
	downloaders *downloader.Downloaders
}

// NewService creates the a new downloader service
func NewService(host p2p.Host, bcs []*core.BlockChain, config downloader.Config) *Service {
	return &Service{
		downloaders: downloader.NewDownloaders(host, bcs, config),
	}
}

// Start start the service
func (s *Service) Start() error {
	s.downloaders.Start()
	return nil
}

// Stop stop the service
func (s *Service) Stop() error {
	s.downloaders.Close()
	return nil
}

// APIs return all APIs of the service
func (s *Service) APIs() []rpc.API {
	return nil
}
