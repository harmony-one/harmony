package gating

import (
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Scores interface {
	GetPeerScore(id peer.ID) (float64, error)
}

// ScoringConnectionGater enhances a ConnectionGater by enforcing a minimum score for peer connections
type ScoringConnectionGater struct {
	ExtendedConnectionGater
	scores   Scores
	minScore float64
}

func AddScoring(gater ExtendedConnectionGater, scores Scores, minScore float64) *ScoringConnectionGater {
	return &ScoringConnectionGater{ExtendedConnectionGater: gater, scores: scores, minScore: minScore}
}

func (g *ScoringConnectionGater) checkScore(p peer.ID) (allow bool, score float64) {
	score, err := g.scores.GetPeerScore(p)
	if err != nil {
		utils.Logger().Warn().Err(err).Str("peer_id", p.String()).Msg("failed to get peer score")
		return false, score
	}
	return score >= g.minScore, score
}

func (g *ScoringConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptPeerDial(p) {
		return false
	}
	check, score := g.checkScore(p)
	if !check {
		utils.Logger().Warn().Str("peer_id", p.String()).Float64("score", score).Float64("min_score", g.minScore).Msg("peer has failed checkScore")
	}
	return check
}

func (g *ScoringConnectionGater) InterceptAddrDial(id peer.ID, ma multiaddr.Multiaddr) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptAddrDial(id, ma) {
		return false
	}
	check, score := g.checkScore(id)
	if !check {
		utils.Logger().Warn().Str("peer_id", id.String()).Float64("score", score).Float64("min_score", g.minScore).Msg("peer has failed checkScore")
	}
	return check
}

func (g *ScoringConnectionGater) InterceptSecured(dir network.Direction, id peer.ID, mas network.ConnMultiaddrs) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptSecured(dir, id, mas) {
		return false
	}
	check, score := g.checkScore(id)
	if !check {
		utils.Logger().Warn().Str("peer_id", id.String()).Float64("score", score).Float64("min_score", g.minScore).Msg("peer has failed checkScore")
	}
	return check
}

func (g *ScoringConnectionGater) InterceptAccept(mas network.ConnMultiaddrs) (allow bool) {
	if !g.ExtendedConnectionGater.InterceptAccept(mas) {
		return false
	}
	utils.Logger().Info().Str("multi_addr", mas.RemoteMultiaddr().String()).Msg("connection accepted")
	return true
}
