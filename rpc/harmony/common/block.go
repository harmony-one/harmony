package common

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
)

// BlockFactory is the interface of factory for RPC block data
type BlockFactory interface {
	NewBlock(b *types.Block, args *BlockArgs) (interface{}, error)
}

// BlockDataProvider helps with providing data for RPC blocks
type BlockDataProvider interface {
	GetLeader(b *types.Block) string
	GetSigners(b *types.Block) ([]string, error)
	GetStakingTxs(b *types.Block) (interface{}, error)
	GetStakingTxHashes(b *types.Block) []common.Hash
}
