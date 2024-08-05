package hmy_boot

import (
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	commonRPC "github.com/harmony-one/harmony/rpc/harmony/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

// GetPeerInfo returns the peer info to the node, including blocked peer, connected peer, number of peers
func (hmyboot *BootService) GetPeerInfo() commonRPC.NodePeerInfo {

	topics := hmyboot.BootNodeAPI.ListTopic()
	p := make([]commonRPC.P, len(topics))

	for i, t := range topics {
		topicPeer := hmyboot.BootNodeAPI.ListPeer(t)
		p[i].Topic = t
		p[i].Peers = make([]peer.ID, len(topicPeer))
		copy(p[i].Peers, topicPeer)
	}

	return commonRPC.NodePeerInfo{
		PeerID:       nodeconfig.GetPeerID(),
		BlockedPeers: hmyboot.BootNodeAPI.ListBlockedPeer(),
		P:            p,
	}
}
