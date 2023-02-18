package rpc

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
)

// PublicHarmonyService provides an API to access Harmony related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicHarmonyService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicHarmonyAPI creates a new API for the RPC interface
func NewPublicHarmonyAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicHarmonyService{hmy, version},
		Public:    true,
	}
}

// ProtocolVersion returns the current Harmony protocol version this node supports
// Note that the return type is an interface to account for the different versions
func (s *PublicHarmonyService) ProtocolVersion(
	ctx context.Context,
) (interface{}, error) {
	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(s.hmy.ProtocolVersion()), nil
	case V2:
		return s.hmy.ProtocolVersion(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// Syncing returns false in case the node is in sync with the network
// If it is syncing, it returns:
// starting block, current block, and network height
func (s *PublicHarmonyService) Syncing(
	ctx context.Context,
) (interface{}, error) {
	// difference = target - current
	inSync, target, difference := s.hmy.NodeAPI.SyncStatus(s.hmy.ShardID)
	if inSync {
		return false, nil
	}
	return struct {
		Start   uint64 `json:"startingBlock"`
		Current uint64 `json:"currentBlock"`
		Target  uint64 `json:"highestBlock"`
	}{
		// Start:   0, // TODO
		Current: target - difference,
		Target:  target,
	}, nil
}

// GasPrice returns a suggestion for a gas price.
// Note that the return type is an interface to account for the different versions
func (s *PublicHarmonyService) GasPrice(ctx context.Context) (interface{}, error) {
	price, err := s.hmy.SuggestPrice(ctx)
	if err != nil || price.Cmp(big.NewInt(100e9)) < 0 {
		price = big.NewInt(100e9)
	}
	// Format response according to version
	switch s.version {
	case V1, Eth:
		return (*hexutil.Big)(price), nil
	case V2:
		return price.Uint64(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetNodeMetadata produces a NodeMetadata record, data is from the answering RPC node
func (s *PublicHarmonyService) GetNodeMetadata(
	ctx context.Context,
) (StructuredResponse, error) {
	// Response output is the same for all versions
	return NewStructuredResponse(s.hmy.GetNodeMetadata())
}

// GetPeerInfo produces a NodePeerInfo record
func (s *PublicHarmonyService) GetPeerInfo(
	ctx context.Context,
) (StructuredResponse, error) {
	// Response output is the same for all versions
	return NewStructuredResponse(s.hmy.GetPeerInfo())
}

// GetNumPendingCrossLinks returns length of hmy.BlockChain.ReadPendingCrossLinks()
func (s *PublicHarmonyService) GetNumPendingCrossLinks() (int, error) {
	links, err := s.hmy.BlockChain.ReadPendingCrossLinks()
	if err != nil {
		return 0, err
	}

	return len(links), nil
}
