package stagedsync

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
)

type commonBlockIter struct {
	parentToChild map[common.Hash]*types.Block
	curParentHash common.Hash
}

func newCommonBlockIter(blocks map[int]*types.Block, startHash common.Hash) *commonBlockIter {
	m := make(map[common.Hash]*types.Block)
	for _, block := range blocks {
		m[block.ParentHash()] = block
	}
	return &commonBlockIter{
		parentToChild: m,
		curParentHash: startHash,
	}
}

func (iter *commonBlockIter) Next() *types.Block {
	curBlock, ok := iter.parentToChild[iter.curParentHash]
	if !ok || curBlock == nil {
		return nil
	}
	iter.curParentHash = curBlock.Hash()
	return curBlock
}

func (iter *commonBlockIter) HasNext() bool {
	_, ok := iter.parentToChild[iter.curParentHash]
	return ok
}
