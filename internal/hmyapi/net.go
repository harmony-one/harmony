package hmyapi

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/p2p"
)

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	net       p2p.Host
	networkID uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net p2p.Host, networkID uint64) *PublicNetAPI {
	return &PublicNetAPI{net, networkID}
}

// PeerCount returns the number of connected peers
func (s *PublicNetAPI) PeerCount() hexutil.Uint {
	return hexutil.Uint(s.net.GetPeerCount())
}

// NetworkID ...
func (s *PublicNetAPI) NetworkID() string {
	return fmt.Sprintf("%d", 1) // TODO(ricl): we should add support for network id (https://github.com/ethereum/wiki/wiki/JSON-RPC#net_version)
}
