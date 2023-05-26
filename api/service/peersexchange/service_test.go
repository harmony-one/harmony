package peersexchange_test

import (
	"context"
	"testing"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/api/service/peersexchange"
	"github.com/harmony-one/harmony/internal/knownpeers"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/require"
)

type syncPeerProvider struct {
}

// SyncingPeers returns peer we will connect to.
func (s syncPeerProvider) SyncingPeers(shardID uint32) ([]p2p.Peer, error) {
	return []p2p.Peer{
		p2p.PeerFromIpPortUnchecked("127.0.0.1:9090"),
	}, nil
}

func TestService_Exchange(t *testing.T) {
	reg := registry.New().SetKnownPeers(knownpeers.NewKnownPeersThreadSafe())
	opts := peersexchange.NewOptions(
		syncPeerProvider{},
		0,
		reg,
		func(ip string, port string, true bool) downloader.Client {
			return client{
				isReady: true,
				response: &pb.DownloaderResponse{ // response from remote node
					Payload: [][]byte{
						[]byte("0.0.0.0:9000"),
						[]byte("255.255.255.255:80"),
					},
				},
			}
		},
	)

	t.Run("exchange", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		service := peersexchange.New(opts)
		service.Exchange(ctx, 0)
		require.Len(t, reg.GetKnownPeers().GetUnchecked(2), 2)
	})
}
