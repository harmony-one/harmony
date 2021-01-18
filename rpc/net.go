package rpc

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// PublicNetService offers network related RPC methods
type PublicNetService struct {
	net     p2p.Host
	chainID uint64
	version Version
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net p2p.Host, chainID uint64, version Version) rpc.API {
	// manually set different namespace to preserve legacy behavior
	var namespace string
	switch version {
	case V1, Eth:
		namespace = netV1Namespace
	case V2:
		namespace = netV2Namespace
	default:
		utils.Logger().Error().Msgf("Unknown version %v, ignoring API.", version)
		return rpc.API{}
	}

	return rpc.API{
		Namespace: namespace,
		Version:   APIVersion,
		Service:   &PublicNetService{net, chainID, version},
		Public:    true,
	}
}

// PeerCount returns the number of connected peers
// Note that the return type is an interface to account for the different versions
func (s *PublicNetService) PeerCount(ctx context.Context) (interface{}, error) {
	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(s.net.GetPeerCount()), nil
	case V2:
		return s.net.GetPeerCount(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// Version returns the network version, i.e. ChainID identifying which network we are using
func (s *PublicNetService) Version(ctx context.Context) string {
	return fmt.Sprintf("%d", s.chainID)
}
