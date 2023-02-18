package crosslink_sending

import (
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/shard"
)

type broadcast interface {
	BroadcastCrosslinkHeartbeatSignalFromBeaconToShards()
	BroadcastCrossLinkFromShardsToBeacon()
}

type Service struct {
	node    broadcast
	bc      core.BlockChain
	ch      chan core.ChainEvent
	closeCh chan struct{}
	beacon  bool
}

func New(node broadcast, bc core.BlockChain) *Service {
	return &Service{
		node:    node,
		bc:      bc,
		ch:      make(chan core.ChainEvent, 1),
		closeCh: make(chan struct{}),
		beacon:  bc.ShardID() == shard.BeaconChainShardID,
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
			if s.beacon {
				go s.node.BroadcastCrosslinkHeartbeatSignalFromBeaconToShards()
			} else {
				go s.node.BroadcastCrossLinkFromShardsToBeacon()
			}
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
