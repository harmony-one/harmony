package discovery

import (
	"context"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	libp2p_dis "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/rs/zerolog"
)

// Discovery is the interface for the underlying peer discovery protocol.
// The interface is implemented by dhtDiscovery
type Discovery interface {
	Start() error
	Close() error
	Advertise(ctx context.Context, ns string) (time.Duration, error)
	FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error)
	GetRawDiscovery() discovery.Discovery
}

// dhtDiscovery is a wrapper of libp2p dht discovery service. It implements Discovery
// interface.
type dhtDiscovery struct {
	dht  *libp2p_dht.IpfsDHT
	disc discovery.Discovery
	host libp2p_host.Host

	opt    DHTConfig
	logger zerolog.Logger
	ctx    context.Context
	cancel func()
}

// NewDHTDiscovery creates a new dhtDiscovery that implements Discovery interface.
func NewDHTDiscovery(ctx context.Context, cancel context.CancelFunc, host libp2p_host.Host, dht *dht.IpfsDHT, opt DHTConfig) (Discovery, error) {
	d := libp2p_dis.NewRoutingDiscovery(dht)
	logger := utils.Logger().With().Str("module", "discovery").Logger()
	return &dhtDiscovery{
		dht:    dht,
		disc:   d,
		host:   host,
		opt:    opt,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start bootstrap the dht discovery service.
func (d *dhtDiscovery) Start() error {
	return d.dht.Bootstrap(d.ctx)
}

// Stop stop the dhtDiscovery service
func (d *dhtDiscovery) Close() error {
	d.cancel()
	return nil
}

// Advertise advertises a service
func (d *dhtDiscovery) Advertise(ctx context.Context, ns string) (time.Duration, error) {
	return d.disc.Advertise(ctx, ns)
}

// FindPeers discovers peers providing a service
func (d *dhtDiscovery) FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error) {
	opt := discovery.Limit(peerLimit)
	return d.disc.FindPeers(ctx, ns, opt)
}

// GetRawDiscovery get the raw discovery to be used for libp2p pubsub options
func (d *dhtDiscovery) GetRawDiscovery() discovery.Discovery {
	return d.disc
}
