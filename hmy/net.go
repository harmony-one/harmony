package hmy

import (
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	commonRPC "github.com/harmony-one/harmony/rpc/common"
	"github.com/harmony-one/harmony/staking/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// GetCurrentUtilityMetrics ..
func (hmy *Harmony) GetCurrentUtilityMetrics() (*network.UtilityMetric, error) {
	return network.NewUtilityMetricSnapshot(hmy.BlockChain)
}

// GetPeerInfo returns the peer info to the node, including blocked peer, connected peer, number of peers
func (hmy *Harmony) GetPeerInfo() commonRPC.NodePeerInfo {

	topics := hmy.NodeAPI.ListTopic()
	p := make([]commonRPC.P, len(topics))

	for i, t := range topics {
		topicPeer := hmy.NodeAPI.ListPeer(t)
		p[i].Topic = t
		p[i].Peers = make([]peer.ID, len(topicPeer))
		copy(p[i].Peers, topicPeer)
	}

	return commonRPC.NodePeerInfo{
		PeerID:       nodeconfig.GetPeerID(),
		BlockedPeers: hmy.NodeAPI.ListBlockedPeer(),
		P:            p,
	}
}
