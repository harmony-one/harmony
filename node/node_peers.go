package node

import (
	"context"
	"time"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

func (node *Node) PeersExchange(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Minute):

		}
	}
}

func (node *Node) peersExchange() {
	peers, err := node.SyncingPeerProvider.SyncingPeers(node.Consensus.ShardID)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("cannot get peers from peer provider")
		return
	}
	for _, peer := range peers {
		node.peersExchangeWithPeer(peer)
	}
}

func (node *Node) peersExchangeWithPeer(peer p2p.Peer) {
	client := downloader.ClientSetup(peer.IP, peer.Port, true)
	if client == nil {
		return
	}
	if !client.IsReady() {
		utils.Logger().Error().Str("ip", peer.IP).Msg("[PeersExchange] client.go:ClientSetup fail to dial")
		return
	}
	utils.Logger().Debug().Str("ip", peer.IP).Msg("[PeersExchange] grpc connect successfully")
	defer client.Close("successful")
	// client.GetBlockHashes()
}
