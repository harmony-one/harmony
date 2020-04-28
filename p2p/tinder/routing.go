package tinder

import (
	"context"

	p2p_routing "github.com/libp2p/go-libp2p-core/routing"
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
	r p2p_routing.Routing,
	drivers ...Driver,
) Routing {
	// rdisc := discovery.NewRoutingDiscovery(r)
	// drivers = append(
	// 	drivers,
	// 	ComposeDriver("harmony-one/p2p-router", r, r, nil),
	// )

	md := NewMultiDriver(logger, drivers...)

	return &routing{
		ContentRouting: r,
		ValueStore:     r,
		PeerRouting:    r,
		Driver:         md,
		bootstrap:      r.Bootstrap,
	}
}
