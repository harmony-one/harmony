package ipfsutil

import (
	"context"
	"math/rand"

	"github.com/harmony-one/harmony/p2p/tinder"
	datastore "github.com/ipfs/go-datastore"
	ipfs_p2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/rs/zerolog"
)

// RoutingOut ..
type RoutingOut struct {
	*dht.IpfsDHT
	tinder.Routing
}

// NewTinderRouting ..
func NewTinderRouting(
	log *zerolog.Logger,
	rdvpeer peer.ID,
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
		if string(rdvpeer) != "" {
			// @FIXME(gfanton): use rand as argument
			rdvClient := tinder.NewRendezvousDiscovery(
				log, h, rdvpeer, rand.New(rand.NewSource(rand.Int63())),
			)
			drivers = append(drivers, rdvClient)
		}

		tinderRouting := tinder.NewRouting(log, "dht", dht, drivers...)
		crout <- &RoutingOut{dht, tinderRouting}

		return tinderRouting, nil
	}, crout
}
