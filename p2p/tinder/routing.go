package tinder

import (
	"context"

	p2p_routing "github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"github.com/rs/zerolog"
)

// Routing ..
type Routing interface {
	p2p_routing.Routing
	Driver
}

type routing struct {
	p2p_routing.ContentRouting
	p2p_routing.PeerRouting
	p2p_routing.ValueStore
	Driver
	bootstrap func(context.Context) error
}

func (r *routing) Bootstrap(ctx context.Context) error {
	return r.bootstrap(ctx)
}

// NewRouting ..
func NewRouting(
	logger *zerolog.Logger,
	name string,
	r p2p_routing.Routing,
	drivers ...Driver,
) Routing {
	rdisc := discovery.NewRoutingDiscovery(r)
	drivers = append(drivers, ComposeDriver(name, rdisc, rdisc, nil))
	md := NewMultiDriver(logger, drivers...)
	cr := discovery.NewDiscoveryRouting(md)
	return &routing{
		ContentRouting: cr,
		ValueStore:     r,
		PeerRouting:    r,
		Driver:         md,
		bootstrap:      r.Bootstrap,
	}
}
