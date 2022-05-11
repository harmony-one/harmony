package crosslink_sending

import (
	"github.com/harmony-one/harmony/core"
)

type BroadcastCrossLinkFromBeaconToNonBeaconChains interface {
	BroadcastCrossLinkFromBeaconToNonBeaconChains()
}

type Service struct {
	node    BroadcastCrossLinkFromBeaconToNonBeaconChains
	bc      *core.BlockChain
	ch      chan core.ChainEvent
	closeCh chan struct{}
}

func New(node BroadcastCrossLinkFromBeaconToNonBeaconChains, bc *core.BlockChain) *Service {
	return &Service{
		node:    node,
		bc:      bc,
		ch:      make(chan core.ChainEvent, 1),
		closeCh: make(chan struct{}),
	}
}

// Start starts service.
func (s *Service) Start() error {
	s.bc.SubscribeChainEvent(s.ch)
	go s.run()
	return nil
}

func (s *Service) run() {
	for {
		select {
		case _, ok := <-s.ch:
			if !ok {
				return
			}
			go s.node.BroadcastCrossLinkFromBeaconToNonBeaconChains()
		case <-s.closeCh:
			return
		}
	}
}

// Stop stops service.
func (s *Service) Stop() error {
	close(s.closeCh)
	return nil
}
