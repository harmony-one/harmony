package discovery

import (
	"github.com/pkg/errors"

	p2ptypes "github.com/harmony-one/harmony/p2p/types"
	badger "github.com/ipfs/go-ds-badger"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
)

// DHTConfig is the configurable DHT options.
// For normal nodes, only BootNodes field need to be specified.
type DHTConfig struct {
	BootNodes       []string
	DataStoreFile   *string // File path to store DHT data. Shall be only used for bootstrap nodes.
	DiscConcurrency int
	DHT             *dht.IpfsDHT
}

// GetLibp2pRawOptions get the raw libp2p options as a slice.
func (opt DHTConfig) GetLibp2pRawOptions() ([]libp2p_dht.Option, error) {
	var opts []libp2p_dht.Option

	bootOption, err := getBootstrapOption(opt.BootNodes)
	if err != nil {
		return nil, err
	}
	opts = append(opts, bootOption)

	if opt.DataStoreFile != nil && len(*opt.DataStoreFile) != 0 {
		dsOption, err := getDataStoreOption(*opt.DataStoreFile)
		if err != nil {
			return nil, err
		}
		opts = append(opts, dsOption)
	}

	// if Concurrency <= 0, it uses default concurrency supplied from libp2p dht
	// the concurrency num meaning you can see Section 2.3 in the KAD paper https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf
	if opt.DiscConcurrency > 0 {
		opts = append(opts, libp2p_dht.Concurrency(opt.DiscConcurrency))
	}

	// TODO: to disable auto refresh to make sure there is no conflicts with protocol discovery functions
	// it's not applicable for legacy sync
	// opts = append(opts, libp2p_dht.DisableAutoRefresh())

	return opts, nil
}

func getBootstrapOption(bootNodes []string) (libp2p_dht.Option, error) {
	resolved, err := p2ptypes.ResolveAndParseMultiAddrs(bootNodes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse boot nodes")
	}
	return libp2p_dht.BootstrapPeers(resolved...), nil
}

func getDataStoreOption(dataStoreFile string) (libp2p_dht.Option, error) {
	ds, err := badger.NewDatastore(dataStoreFile, nil)
	if err != nil {
		return nil, errors.Wrapf(err,
			"cannot open Badger data store at %s", dataStoreFile)
	}
	return libp2p_dht.Datastore(ds), nil
}
