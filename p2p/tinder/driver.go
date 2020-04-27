package tinder

import (
	"context"

	p2p_discovery "github.com/libp2p/go-libp2p-core/discovery"
)

// Driver is a p2p_discovery.Discovery
var _ p2p_discovery.Discovery = (Driver)(nil)

type Unregisterer interface {
	Unregister(ctx context.Context, ns string) error
}

// Driver is a Discovery with a unregister method
type Driver interface {
	p2p_discovery.Discovery
	Unregisterer

	Name() string
}

var NoopUnregisterer Unregisterer = &noopUnregisterer{}

type noopUnregisterer struct{}

func (*noopUnregisterer) Unregister(context.Context, string) error { return nil }

func (*noopUnregisterer) Name() string { return "noop" }

// composeDriver is a Driver
var _ Driver = (*composeDriver)(nil)

// Compose Driver
type composeDriver struct {
	p2p_discovery.Advertiser
	p2p_discovery.Discoverer
	Unregisterer

	name string
}

func (c *composeDriver) Name() string {
	return c.name
}

// ComposeDriver ..
func ComposeDriver(name string,
	advertiser p2p_discovery.Advertiser,
	discover p2p_discovery.Discoverer,
	unregister Unregisterer,
) Driver {
	return &composeDriver{
		Advertiser:   advertiser,
		Discoverer:   discover,
		Unregisterer: unregister,

		name: name,
	}
}
