package peersexchange

import (
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/internal/registry"
)

type ClientSetup func(ip string, port string, true bool) downloader.Client

type Options struct {
	syncingPeerProvider SyncingPeerProvider
	shardID             uint32 // TODO: this should be dynamic, but current version of consensus does not support it.
	reg                 *registry.Registry
	clientSetup         ClientSetup
}

func NewOptions(
	syncingPeerProvider SyncingPeerProvider,
	shardID uint32,
	reg *registry.Registry,
	clientSetup ClientSetup,
) Options {
	return Options{
		syncingPeerProvider: syncingPeerProvider,
		shardID:             shardID,
		reg:                 reg,
		clientSetup:         clientSetup,
	}
}
