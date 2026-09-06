package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
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
	GetRawDiscovery() []discovery.Discovery
	// todo(sun): revert in phase 2
	// GetRawDiscovery() discovery.Discovery
}

// dhtDiscovery is a wrapper of libp2p dht discovery service. It implements Discovery
// interface.
type dhtDiscovery struct {
	dht  []*libp2p_dht.IpfsDHT
	disc []discovery.Discovery
	host libp2p_host.Host

	opt    DHTConfig
	logger zerolog.Logger
	ctx    context.Context
	cancel func()
}

// NewDHTDiscovery creates a new dhtDiscovery that implements Discovery interface.
func NewDHTDiscovery(ctx context.Context, cancel context.CancelFunc, host libp2p_host.Host, opt DHTConfig, dhts ...*libp2p_dht.IpfsDHT) (Discovery, error) {
	var d []discovery.Discovery
	for _, dht := range dhts {
		d = append(d, libp2p_dis.NewRoutingDiscovery(dht))
	}

	logger := utils.Logger().With().Str("module", "discovery").Logger()
	return &dhtDiscovery{
		dht:    dhts,
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
	for _, dht := range d.dht {
		if err := dht.Bootstrap(d.ctx); err != nil {
			return err
		}
	}
	return nil

	// todo(sun): revert in phase 2
	// return d.dht.Bootstrap(d.ctx)
}

// Stop stop the dhtDiscovery service
func (d *dhtDiscovery) Close() error {
	for _, dht := range d.dht {
		if err := dht.Close(); err != nil {
			return err
		}
	}
	d.cancel()
	return nil

	// todo(sun): revert in phase 2
	// d.dht.Close()
	// d.cancel()
	// return nil
}

// Advertise advertises a service
func (d *dhtDiscovery) Advertise(ctx context.Context, ns string) (time.Duration, error) {
	var lastDur time.Duration
	var lastErr error
	for _, disc := range d.disc {
		lastDur, lastErr = disc.Advertise(ctx, ns)
		if lastErr != nil {
			break
		}
	}
	return lastDur, lastErr

	// todo(sun): revert in phase 2
	// return d.disc.Advertise(ctx, ns)
}

// FindPeers discovers peers providing a service
func (d *dhtDiscovery) FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error) {
	mergedChan := make(chan libp2p_peer.AddrInfo)
	var wg sync.WaitGroup
	limitOpt := discovery.Limit(peerLimit)

	// loop through each discovery instance (harmony and legacy, in bootnode's case)
	for _, disc := range d.disc {
		wg.Add(1)

		// launch a goroutine for each DHT query
		go func(disc discovery.Discovery) {
			defer wg.Done()
			peerChan, err := disc.FindPeers(ctx, ns, limitOpt)
			if err != nil {
				d.logger.Error().Err(err).Msg("Discovery failed in one of the DHTs")
				return
			}

			// read peers from the current DHT chan and forward to the merged chan
			for peer := range peerChan {
				select {
				case mergedChan <- peer:
				case <-ctx.Done():
					return
				}
			}
		}(disc)
	}

	// close the merged chan onceboth DHT queries are completed
	go func() {
		wg.Wait()
		close(mergedChan)
	}()

	// immediately return merged chan
	return mergedChan, nil

	// todo(sun): revert in phase 2
	// opt := discovery.Limit(peerLimit)
	// return d.disc.FindPeers(ctx, ns, opt)
}

// GetRawDiscovery get the raw discovery to be used for libp2p pubsub options
// todo(sun): libp2p pubsub option only accepts a single discover.Discovery as option
// todo(sun): revert in phase 2
// func (d *dhtDiscovery) GetRawDiscovery() discovery.Discovery {
func (d *dhtDiscovery) GetRawDiscovery() []discovery.Discovery {
	return d.disc
}
