package discovery

import (
	"github.com/pkg/errors"

	p2ptypes "github.com/harmony-one/harmony/p2p/types"
	badger "github.com/ipfs/go-ds-badger"
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
)

// DHTConfig is the configurable DHT options.
// For normal nodes, only BootNodes field need to be specified.
type DHTConfig struct {
	BootNodes     []string
	DataStoreFile *string // File path to store DHT data. Shall be only used for bootstrap nodes.
}

// getLibp2pRawOptions get the raw libp2p options as a slice.
func (opt DHTConfig) getLibp2pRawOptions() ([]libp2p_dht.Option, error) {
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
