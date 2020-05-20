package apiv1

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/p2p"
)

// PublicNetAPI offers network related RPC methods


// An NetworkVersion response  model.
//
// This is used to offer network related RPC methods
//
// swagger:response NetworkVersion
type PublicNetAPI struct {
	// The net peer to peer host
        //
        // in: path
        // required: false
	net            p2p.Host
	// The network Version
        //
        // in: path
        // required: false
	networkVersion uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net p2p.Host, networkVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{net, networkVersion}
}

// PeerCount returns the number of connected peers
func (s *PublicNetAPI) PeerCount() hexutil.Uint {
    // swagger:operation GET / PeerCount
    //
    // Returns the number of connected peers
    //
    // ---
    // produces:
    // - application/json
    // parameters:
    // - name: net
    //   in: query
    //   description: the network peer to peer host
    //   required: false
    //   type: p2p.Host
    // - name: NetworkVersion
    //   in: query
    //   description: the network version
    //   required: false
    //   type: integer
    //   format: uint64
    // responses:
    //   '200':
    //     description: number of connected peers
    //     schema:
    //       type: string
    //       format: hexutil.Uint
	return hexutil.Uint(s.net.GetPeerCount())
}
// Version swagger:route GET /net_version net_version
//
// Returns the network version
//
// Responses:
//        200: NetworkVersion
// Version returns the network version, i.e. network ID identifying which network we are using
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion) // TODO(ricl): we should add support for network id (https://github.com/ethereum/wiki/wiki/JSON-RPC#net_version)
}
