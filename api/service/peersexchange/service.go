package peersexchange

import (
	"context"
	"time"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

const (
	defaultExchangeTimeout = 5 * time.Minute
	defaultCheckTimeout    = 30 * time.Minute
)

// SyncingPeerProvider is an interface for getting the peers in the given shard.
type SyncingPeerProvider interface {
	SyncingPeers(shardID uint32) (peers []p2p.Peer, err error)
}

type Service struct {
	options Options
}

// New creates a new peer exchange service.
func New(options Options) *Service {
	return &Service{
		options: options,
	}
}

// Exchange starts loop for exchanging peers with peers.
func (s *Service) Exchange(ctx context.Context, timeout time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(timeout):
	}
	peers, err := s.options.syncingPeerProvider.SyncingPeers(s.options.shardID)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("cannot get peers from peer provider")
		return
	}
	for _, peer := range peers {
		s.peersExchangeWithPeer(peer)
	}
	go s.Exchange(ctx, defaultExchangeTimeout)
}

func (s *Service) peersExchangeWithPeer(peer p2p.Peer) {
	client := s.options.clientSetup(peer.IP, peer.Port, true)
	if client == nil {
		return
	}
	if !client.IsReady() {
		utils.Logger().Error().Str("ip", peer.IP).Msg("[PeersExchange] client.go:ClientSetup failed to dial")
		return
	}
	utils.Logger().Debug().Str("ip", peer.IP).Msg("[PeersExchange] grpc connected successfully")
	defer client.Close("successful")
	rs, err := client.GetPeers()
	if err != nil {
		utils.Logger().Error().Err(err).Str("ip", peer.IP).Msg("[PeersExchange] failed to get peers")
		return
	}
	for _, peer := range rs.Payload {
		if l := len(peer); l == 0 || l > 50 {
			continue
		}
		s.options.reg.GetKnownPeers().AddUnchecked(p2p.PeerFromIpPortUnchecked(string(peer)))
	}
}

func (s *Service) peersCheckUnchecked(ctx context.Context, timeout time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(timeout):
	}
	known := s.options.reg.GetKnownPeers()

	unckeched := known.GetUnchecked(known.GetUncheckedCount())
	for _, peer := range unckeched {
		if ctx.Err() != nil {
			return
		}
		client := downloader.ClientSetup(peer.IP, peer.Port, true)
		if client != nil {
			known.AddChecked(peer)
		} else {
			known.RemoveUnchecked(peer)
		}
	}
	go s.peersCheckUnchecked(ctx, defaultCheckTimeout)
}

// Start starts the peer exchange service.
func (s *Service) Start() error { // TODO: make service accept context
	ctx := context.TODO()
	go s.Exchange(ctx, 1*time.Minute)
	go s.peersCheckUnchecked(ctx, 5*time.Minute)
	return nil
}

func (s *Service) Stop() error {
	// TODO: services should be stopped by context.
	return nil
}
