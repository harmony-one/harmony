package ipfsutil

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/harmony-one/harmony/p2p/tinder"
	datastore "github.com/ipfs/go-datastore"
	ipfs_p2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	p2p_discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/rs/zerolog"
)

// RoutingOut ..
type RoutingOut struct {
	*dht.IpfsDHT
	tinder.Service
}

// NewTinderRouting ..
func NewTinderRouting(
	log *zerolog.Logger,
	rdvpeer *peer.AddrInfo,
	dhtclient bool,
) (ipfs_p2p.RoutingOption, <-chan *RoutingOut) {
	crout := make(chan *RoutingOut, 1)

	return func(
		ctx context.Context,
		h host.Host,
		dstore datastore.Batching,
		validator record.Validator,
	) (routing.Routing, error) {
		defer close(crout)

		dht, err := dht.New(
			ctx, h,
			dht.Datastore(dstore),
			dht.Validator(validator),
			dhtopts.Client(dhtclient),
		)

		if err != nil {
			return nil, err
		}

		drivers := []tinder.Driver{}
		if rdvpeer != nil {
			h.Peerstore().AddAddrs(rdvpeer.ID, rdvpeer.Addrs, libp2p_peer.PermanentAddrTTL)
			// @FIXME(gfanton): use rand as argument
			rdvClient := tinder.NewRendezvousDiscovery(
				log, h, rdvpeer.ID, rand.New(rand.NewSource(rand.Int63())),
			)

			drivers = append(drivers, rdvClient)
		}

		tinderRouting := tinder.NewRouting(log, "dht", dht, drivers...)

		drivers = append(drivers, tinderRouting)

		serv, err := tinder.NewService(
			log,
			drivers,
			p2p_discovery.NewFixedBackoff(time.Minute),
			p2p_discovery.WithBackoffDiscoveryReturnedChannelSize(24),
			p2p_discovery.WithBackoffDiscoverySimultaneousQueryBufferSize(24),
		)

		if err != nil {
			fmt.Println("why this busted", err.Error())
		}

		crout <- &RoutingOut{dht, serv}

		return tinderRouting, nil
	}, crout
}
