package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/eth/rpc"
	hmyboot "github.com/harmony-one/harmony/hmy_boot"
)

// PublicBootService provides an API to access Harmony related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBootService struct {
	hmyboot *hmyboot.BootService
	version Version
}

// NewPublicBootAPI creates a new API for the RPC interface
func NewPublicBootAPI(hmyboot *hmyboot.BootService, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicBootService{hmyboot, version},
		Public:    true,
	}
}

// ProtocolVersion returns the current Harmony protocol version this node supports
// Note that the return type is an interface to account for the different versions
func (s *PublicBootService) ProtocolVersion(
	ctx context.Context,
) (interface{}, error) {
	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(s.hmyboot.ProtocolVersion()), nil
	case V2:
		return s.hmyboot.ProtocolVersion(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetNodeMetadata produces a NodeMetadata record, data is from the answering RPC node
func (s *PublicBootService) GetNodeMetadata(
	ctx context.Context,
) (StructuredResponse, error) {
	// Response output is the same for all versions
	return NewStructuredResponse(s.hmyboot.GetNodeMetadata())
}

// GetPeerInfo produces a NodePeerInfo record
func (s *PublicBootService) GetPeerInfo(
	ctx context.Context,
) (StructuredResponse, error) {
	// Response output is the same for all versions
	return NewStructuredResponse(s.hmyboot.GetPeerInfo())
}
