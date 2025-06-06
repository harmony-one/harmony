package gating

import (
	"errors"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/harmony-one/harmony/common/clock"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/store"
)

// type UnbanMetrics interface {
// 	RecordPeerUnban()
// 	RecordIPUnban()
// }

type ExpiryStore interface {
	store.IPBanStore
	store.PeerBanStore
}

// ExpiryConnectionGater enhances a ExtendedConnectionGater by implementing ban-expiration
type ExpiryConnectionGater struct {
	ExtendedConnectionGater
	store ExpiryStore
	clock clock.Clock
}

func AddBanExpiry(gater ExtendedConnectionGater, store ExpiryStore, clock clock.Clock) *ExpiryConnectionGater {
	return &ExpiryConnectionGater{
		ExtendedConnectionGater: gater,
		store:                   store,
		clock:                   clock,
	}
}

func (g *ExpiryConnectionGater) UnblockPeer(p peer.ID) error {
	if err := g.ExtendedConnectionGater.UnblockPeer(p); err != nil {
		utils.Logger().Warn().
			Str("method", "UnblockPeer").
			Str("peer_id", p.String()).
			Err(err).
			Msg("failed to unblock peer from underlying gater")
		return err
	}
	if err := g.store.SetPeerBanExpiration(p, time.Time{}); err != nil {
		utils.Logger().Warn().
			Str("method", "UnblockPeer").
			Str("peer_id", p.String()).
			Err(err).
			Msg("failed to unblock peer from expiry gater")
		return err
	}
	return nil
}

func (g *ExpiryConnectionGater) peerBanExpiryCheck(p peer.ID) (allow bool) {
	expiry, err := g.store.GetPeerBanExpiration(p)
	if errors.Is(err, store.ErrUnknownBan) {
		return true
	}
	if err != nil {
		utils.Logger().Warn().
			Str("method", "peerBanExpiryCheck").
			Str("peer_id", p.String()).
			Err(err).
			Msg("failed to load peer-ban expiry time")
		return false
	}
	if g.clock.Now().Before(expiry) {
		return false
	}
	utils.Logger().Info().
		Str("peer_id", p.String()).
		Time("expiry", expiry).
		Msg("peer-ban expired, unbanning peer")
	if err := g.store.SetPeerBanExpiration(p, time.Time{}); err != nil {
		utils.Logger().Warn().
			Str("method", "peerBanExpiryCheck").
			Str("peer_id", p.String()).
			Err(err).
			Msg("failed to unban peer")
		return false
	}
	return true
}

func (g *ExpiryConnectionGater) addrBanExpiryCheck(ma multiaddr.Multiaddr) (allow bool) {
	ip, err := manet.ToIP(ma)
	if err != nil {
		utils.Logger().Error().
			Str("method", "addrBanExpiryCheck").
			Str("addr", ma.String()).
			Msg("tried to check multi-addr with bad IP")
		return false
	}
	expiry, err := g.store.GetIPBanExpiration(ip)
	if errors.Is(err, store.ErrUnknownBan) {
		return true
	}
	if err != nil {
		utils.Logger().Warn().
			Str("method", "addrBanExpiryCheck").
			Str("ip", ip.String()).
			Err(err).
			Msg("failed to load IP-ban expiry time")
		return false
	}
	if g.clock.Now().Before(expiry) {
		return false
	}
	utils.Logger().Info().
		Str("method", "addrBanExpiryCheck").
		Str("ip", ip.String()).
		Time("expiry", expiry).
		Msg("IP-ban expired, unbanning IP")
	if err := g.store.SetIPBanExpiration(ip, time.Time{}); err != nil {
		utils.Logger().Warn().
			Str("method", "addrBanExpiryCheck").
			Str("ip", ip.String()).
			Err(err).
			Msg("failed to unban IP")
		return false
	}
	return true
}

func (g *ExpiryConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptPeerDial(p) {
		return false
	}
	peerBan := g.peerBanExpiryCheck(p)
	if !peerBan {
		utils.Logger().Warn().
			Str("method", "InterceptPeerDial").
			Str("peer_id", p.String()).
			Msg("peer is temporarily banned")
	}
	return peerBan
}

func (g *ExpiryConnectionGater) InterceptAddrDial(id peer.ID, ma multiaddr.Multiaddr) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptAddrDial(id, ma) {
		return false
	}
	peerBan := g.peerBanExpiryCheck(id)
	if !peerBan {
		utils.Logger().Warn().
			Str("method", "InterceptAddrDial").
			Str("peer_id", id.String()).
			Str("multi_addr", ma.String()).
			Msg("peer id is temporarily banned")
		return false
	}
	addrBan := g.addrBanExpiryCheck(ma)
	if !addrBan {
		utils.Logger().Warn().
			Str("method", "InterceptAddrDial").
			Str("peer_id", id.String()).
			Str("multi_addr", ma.String()).
			Msg("peer address is temporarily banned")
		return false
	}
	return true
}

func (g *ExpiryConnectionGater) InterceptAccept(mas network.ConnMultiaddrs) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptAccept(mas) {
		return false
	}
	addrBan := g.addrBanExpiryCheck(mas.RemoteMultiaddr())
	if !addrBan {
		utils.Logger().Warn().
			Str("method", "InterceptAccept").
			Str("multi_addr", mas.RemoteMultiaddr().String()).
			Msg("peer address is temporarily banned")
	}
	return addrBan
}

func (g *ExpiryConnectionGater) InterceptSecured(direction network.Direction, id peer.ID, mas network.ConnMultiaddrs) (allow bool) {
	if direction == network.DirOutbound {
		return true
	}
	if !g.ExtendedConnectionGater.InterceptSecured(direction, id, mas) {
		return false
	}
	peerBan := g.peerBanExpiryCheck(id)
	if !peerBan {
		utils.Logger().Warn().
			Str("method", "InterceptSecured").
			Str("peer_id", id.String()).
			Str("multi_addr", mas.RemoteMultiaddr().String()).
			Msg("peer id is temporarily banned")
	}
	return peerBan
}
