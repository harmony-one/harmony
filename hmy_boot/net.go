package hmy_boot

import (
	commonRPC "github.com/harmony-one/harmony/rpc/boot/common"
)

// GetPeerInfo returns the peer info to the node, including blocked peer, connected peer, number of peers
func (hmyboot *BootService) GetPeerInfo() commonRPC.BootNodePeerInfo {

	var c commonRPC.C
	c.TotalKnownPeers, c.Connected, c.NotConnected = hmyboot.BootNodeAPI.PeerConnectivity()

	knownPeers := hmyboot.BootNodeAPI.ListKnownPeers()
	connectedPeers := hmyboot.BootNodeAPI.ListConnectedPeers()

	return commonRPC.BootNodePeerInfo{
		PeerID:         hmyboot.BootNodeAPI.PeerID(),
		KnownPeers:     knownPeers,
		ConnectedPeers: connectedPeers,
		C:              c,
	}
}
