package peersexchange

import (
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/internal/knownpeers"
)

type ClientSetup func(ip string, port string, true bool) downloader.Client

type Options struct {
	syncingPeerProvider SyncingPeerProvider
	shardID             uint32 // TODO: this should be dynamic, but current version of consensus does not support it.
	knownPeers          knownpeers.KnownPeers
	clientSetup         ClientSetup
}

func NewOptions(
	syncingPeerProvider SyncingPeerProvider,
	shardID uint32,
	knownPeers knownpeers.KnownPeers,
	clientSetup ClientSetup,
) Options {
	return Options{
		syncingPeerProvider: syncingPeerProvider,
		shardID:             shardID,
		knownPeers:          knownPeers,
		clientSetup:         clientSetup,
	}
}
