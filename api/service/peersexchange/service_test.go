package peersexchange_test

import (
	"context"
	"testing"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/api/service/peersexchange"
	"github.com/harmony-one/harmony/internal/knownpeers"
	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/require"
)

type syncPeerProvider struct {
}

func (s syncPeerProvider) SyncingPeers(shardID uint32) ([]p2p.Peer, error) {
	return []p2p.Peer{
		p2p.PeerFromIpPortUnchecked("127.0.0.1:9090"),
	}, nil
}

func TestService_Exchange(t *testing.T) {
	p := knownpeers.NewKnownPeers()
	opts := peersexchange.NewOptions(
		syncPeerProvider{},
		0,
		p,
		func(ip string, port string, true bool) downloader.Client {
			return client{}
		},
	)

	t.Run("exchange", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		service := peersexchange.New(opts)
		service.Exchange(ctx, 0)
		require.Len(t, p.GetUnchecked(1), 1)
	})
}
