package stagedstreamsync

import (
	"fmt"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/pkg/errors"
)

var emptySigVerifyErr *sigVerifyErr

type sigVerifyErr struct {
	err error
}

func (e *sigVerifyErr) Error() string {
	return fmt.Sprintf("[VerifyHeaderSignature] %v", e.err.Error())
}

func verifyAndInsertBlocks(bc blockChain, blocks types.Blocks) (int, error) {
	for i, block := range blocks {
		if err := verifyAndInsertBlock(bc, block, blocks[i+1:]...); err != nil {
			return i, err
		}
	}
	return len(blocks), nil
}

func verifyBlock(bc blockChain, block *types.Block, nextBlocks ...*types.Block) error {
	var (
		sigBytes bls.SerializedSignature
		bitmap   []byte
		err      error
	)
	if len(nextBlocks) > 0 {
		// get commit sig from the next block
		next := nextBlocks[0]
		sigBytes = next.Header().LastCommitSignature()
		bitmap = next.Header().LastCommitBitmap()
	} else {
		// get commit sig from current block
		sigBytes, bitmap, err = chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
		if err != nil {
			return errors.Wrap(err, "parse commitSigAndBitmap")
		}
	}

	if err := bc.Engine().VerifyHeaderSignature(bc, block.Header(), sigBytes, bitmap); err != nil {
		return &sigVerifyErr{err}
	}
	if err := bc.Engine().VerifyHeader(bc, block.Header(), true); err != nil {
		return errors.Wrap(err, "[VerifyHeader]")
	}

	return nil
}

func verifyAndInsertBlock(bc blockChain, block *types.Block, nextBlocks ...*types.Block) error {
	//verify block
	if err := verifyBlock(bc, block, nextBlocks...); err != nil {
		return err
	}
	// insert block
	if _, err := bc.InsertChain(types.Blocks{block}, false); err != nil {
		if errors.Is(err, core.ErrKnownBlock) {
			return nil
		}
		return errors.Wrap(err, "[InsertChain]")
	}
	return nil
}
