package eth

import (
	"github.com/harmony-one/harmony/core/types"
	internal_common "github.com/harmony-one/harmony/internal/common"
	rpc_common "github.com/harmony-one/harmony/rpc/common"
	"github.com/pkg/errors"
)

// BlockFactory is the factory for v1 rpc block
type BlockFactory struct {
	provider rpc_common.BlockDataProvider
}

// NewBlockFactory return the block factory with the given provider
func NewBlockFactory(provider rpc_common.BlockDataProvider) *BlockFactory {
	return &BlockFactory{provider}
}

// NewBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func (fac *BlockFactory) NewBlock(b *types.Block, args *rpc_common.BlockArgs) (interface{}, error) {
	if args.FullTx {
		return fac.newBlockWithFullTx(b, args)
	}
	return fac.newBlockWithTxHash(b, args)
}

func (fac *BlockFactory) newBlockWithTxHash(b *types.Block, args *rpc_common.BlockArgs) (*BlockWithTxHash, error) {
	blk := blockWithTxHashFromBlock(b)

	leader := fac.provider.GetLeader(b)
	ethLeader, err := internal_common.ParseAddr(leader)
	if err != nil {
		return nil, errors.Wrap(err, "parse leader address")
	}
	blk.Block.Miner = ethLeader

	if args.WithSigners {
		signers, err := fac.provider.GetSigners(b)
		if err != nil {
			return nil, errors.Wrap(err, "GetSigners")
		}
		blk.Signers = signers
	}
	return blk, nil
}

func (fac *BlockFactory) newBlockWithFullTx(b *types.Block, args *rpc_common.BlockArgs) (*BlockWithFullTx, error) {
	blk, err := blockWithFullTxFromBlock(b)
	if err != nil {
		return nil, errors.Wrap(err, "blockWithFullTxFromBlock")
	}

	leader := fac.provider.GetLeader(b)
	ethLeader, err := internal_common.ParseAddr(leader)
	if err != nil {
		return nil, errors.Wrap(err, "parse leader address")
	}
	blk.Block.Miner = ethLeader

	if args.WithSigners {
		signers, err := fac.provider.GetSigners(b)
		if err != nil {
			return nil, errors.Wrap(err, "GetSigners")
		}
		blk.Signers = signers
	}
	return blk, nil
}
