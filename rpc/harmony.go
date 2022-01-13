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

// Syncing returns false in case the node is currently not syncing with the network. It can be up to date or has not
// yet received the latest block headers from its pears. In case it is synchronizing:
// - startingBlock: block number this node started to synchronise from
// - currentBlock:  block number this node is currently importing
// - highestBlock:  block number of the highest block header this node has received from peers
// - pulledStates:  number of state entries processed until now
// - knownStates:   number of known state entries that still need to be pulled
func (s *PublicHarmonyService) Syncing(
	ctx context.Context,
) (interface{}, error) {
	// TODO(dm): find our Downloader module for syncing blocks
	return false, nil
}

// GasPrice returns a suggestion for a gas price.
// Note that the return type is an interface to account for the different versions
func (s *PublicHarmonyService) GasPrice(ctx context.Context) (interface{}, error) {
	price, err := s.hmy.SuggestPrice(ctx)
	if err != nil || price.Cmp(big.NewInt(3e10)) < 0 {
		price = big.NewInt(3e10)
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
