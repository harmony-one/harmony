package apiv1

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/hmy"
	commonRPC "github.com/harmony-one/harmony/internal/hmyapi/common"
)

// PublicHarmonyAPI provides an API to access Harmony related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicHarmonyAPI struct {
	hmy *hmy.Harmony
}

// NewPublicHarmonyAPI ...
func NewPublicHarmonyAPI(hmy *hmy.Harmony) *PublicHarmonyAPI {
	return &PublicHarmonyAPI{hmy}
}

// ProtocolVersion returns the current Harmony protocol version this node supports
func (s *PublicHarmonyAPI) ProtocolVersion() hexutil.Uint {
	return hexutil.Uint(s.hmy.ProtocolVersion())
}

// Syncing returns false in case the node is currently not syncing with the network. It can be up to date or has not
// yet received the latest block headers from its pears. In case it is synchronizing:
// - startingBlock: block number this node started to synchronise from
// - currentBlock:  block number this node is currently importing
// - highestBlock:  block number of the highest block header this node has received from peers
// - pulledStates:  number of state entries processed until now
// - knownStates:   number of known state entries that still need to be pulled
func (s *PublicHarmonyAPI) Syncing() (interface{}, error) {
	// TODO(dm): find our Downloader module for syncing blocks
	return false, nil
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicHarmonyAPI) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	// TODO(ricl): add SuggestPrice API
	return (*hexutil.Big)(big.NewInt(1)), nil
}

// GetNodeMetadata produces a NodeMetadata record, data is from the answering RPC node
func (s *PublicHarmonyAPI) GetNodeMetadata() commonRPC.NodeMetadata {
	return s.hmy.GetNodeMetadata()
}

// GetPeerInfo produces a NodePeerInfo record
func (s *PublicHarmonyAPI) GetPeerInfo() commonRPC.NodePeerInfo {
	return s.hmy.GetPeerInfo()
}
