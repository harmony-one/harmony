package tinder

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	p2p_discovery "github.com/libp2p/go-libp2p-core/discovery"
	p2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
)

// MultiDriver is a simple driver manager, that forward request across multiple driver
type MultiDriver struct {
	logger  *zerolog.Logger
	drivers []Driver
	mapc    map[string]context.CancelFunc
	muc     sync.Mutex
}

// NewMultiDriver ..
func NewMultiDriver(logger *zerolog.Logger, drivers ...Driver) Driver {
	return &MultiDriver{
		logger:  logger,
		drivers: drivers,
		mapc:    map[string]context.CancelFunc{},
	}
}

// Advertise simply dispatch Advertise request across all the drivers
func (md *MultiDriver) Advertise(
	ctx context.Context, ns string, opts ...p2p_discovery.Option,
) (time.Duration, error) {
	// Get options
	var options p2p_discovery.Options
	err := options.Apply(opts...)
	if err != nil {
		return 0, err
	}

	md.muc.Lock()
	if cf, ok := md.mapc[ns]; ok {
		cf()
	}

	ctx, cf := context.WithCancel(ctx)
	md.mapc[ns] = cf
	md.muc.Unlock()

	for _, driver := range md.drivers {
		md.advertise(ctx, driver, ns, opts...)
	}

	return options.Ttl, nil
}

func (md *MultiDriver) advertise(
	ctx context.Context, d Driver,
	ns string, opts ...p2p_discovery.Option,
) {
	go func() {
		for {
			ttl, err := d.Advertise(ctx, ns, opts...)
			if err != nil {
				md.logger.Err(err).Msg("failed to advertise")
				if ctx.Err() != nil {
					return
				}
				timeMin := time.NewTimer(time.Minute)

				select {
				case <-timeMin.C:
					timeMin.Stop()
					continue
				case <-ctx.Done():
					return
				}
			}

			md.logger.Info().
				Str("driver", d.Name()).
				Str("key", ns).Msg("advertise")

			timer := time.NewTimer(7 * ttl / 8)

			select {
			case <-timer.C:
				timer.Stop()
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// FindPeers for MultiDriver doesn't care about duplicate peers, his only
// job here is to dispatch FindPeers request across all the drivers.
func (md *MultiDriver) FindPeers(
	ctx context.Context,
	ns string, opts ...p2p_discovery.Option,
) (<-chan p2p_peer.AddrInfo, error) {

	md.logger.Debug().Str("key", ns).Msg("looking for peers")
	ctx, cancel := context.WithCancel(ctx)

	const selDone = 0
	selCases := make([]reflect.SelectCase, 1)
	selCases[selDone] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}

	driverRefs := make([]string, 1)
	for _, driver := range md.drivers {
		ch, err := driver.FindPeers(ctx, ns, opts...)
		if err != nil {
			md.logger.Warn().Err(err).
				Str("driver", driver.Name()).
				Str("key", ns).
				Msg("failed to run find peers")

			continue
		}

		driverRefs = append(driverRefs, driver.Name())
		selCases = append(selCases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		})
	}

	ndrivers := len(selCases) - 1 // we dont want to wait for the context
	if ndrivers == 0 {
		md.logger.Error().Msg("no drivers available to find peers")
	}

	cpeers := make(chan p2p_peer.AddrInfo, ndrivers)
	go func() {
		defer cancel()
		defer close(cpeers)

		for ndrivers > 0 {
			idx, value, ok := reflect.Select(selCases)

			// context has been cancel stop and close chan
			if idx == selDone {
				return
			}

			if !ok {
				// The chosen channel has been closed, so zero out the channel to disable the case
				selCases[idx].Chan = reflect.ValueOf(nil)
				ndrivers--
				continue
			}

			// we can safely get our peer
			peer := value.Interface().(p2p_peer.AddrInfo)
			// md.logger.Debug().
			// 	Str("driver", driverRefs[idx]).
			// 	Str("key", ns).
			// 	Str("peer", peer.ID.String()).
			// 	Msg("found a peer")

			// forward the peer
			select {
			case cpeers <- peer:
			case <-ctx.Done():
				return
			}
		}
	}()

	return cpeers, nil
}

// Unregister ..
func (md *MultiDriver) Unregister(ctx context.Context, ns string) error {
	fmt.Println("this sometihg calling unregister?")
	// first cancel advertiser
	md.muc.Lock()
	if cf, ok := md.mapc[ns]; ok {
		cf()
		delete(md.mapc, ns)
	}
	md.muc.Unlock()

	// unregister drivers
	for _, driver := range md.drivers {
		_ = driver.Unregister(ctx, ns) // @TODO(gfanton): log this
	}

	return nil
}

// Name ..
func (*MultiDriver) Name() string { return "MultiDriver" }
