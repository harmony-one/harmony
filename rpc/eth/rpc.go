package eth

import (
	"context"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
)

// PublicEthService provides an API to access to the Eth endpoints for the Harmony blockchain.
type PublicEthService struct {
	hmy *hmy.Harmony
}

// NewPublicEthService creates a new API for the RPC interface
func NewPublicEthService(hmy *hmy.Harmony, namespace string) rpc.API {
	if namespace == "" {
		namespace = "eth"
	}

	return rpc.API{
		Namespace: namespace,
		Version:   "1.0",
		Service:   &PublicEthService{hmy},
		Public:    true,
	}
}

// GetBalance returns the amount of Atto for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicEthService) GetBalance(
	ctx context.Context, address string, blockNrOrHash rpc.BlockNumberOrHash,
) (*hexutil.Big, error) {
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	balance, err := s.hmy.GetBalance(ctx, addr, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	return (*hexutil.Big)(balance), nil
}
