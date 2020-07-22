package apiv2

import (
	"fmt"

	"github.com/harmony-one/harmony/p2p"
)

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	net     p2p.Host
	chainID uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net p2p.Host, chainID uint64) *PublicNetAPI {
	return &PublicNetAPI{net, chainID}
}

// PeerCount returns the number of connected peers
func (s *PublicNetAPI) PeerCount() int {
	return s.net.GetPeerCount()
}

// Version returns the network version, i.e. ChainID identifying which network we are using
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.chainID)
}
