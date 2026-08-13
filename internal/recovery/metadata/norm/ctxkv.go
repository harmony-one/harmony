package norm

import (
	"context"

	"github.com/ethereum/go-ethereum/ethdb"
)

// ctxKV wraps a RawKV so every iterator observes context cancellation
// (checked every ctxCheckStride steps — cheap even on the ~92.7M-key
// mainnet blk-rwd walk). Cancellation latches as the iterator error, which
// strictdb.ForEach fail-closed checking surfaces to the caller.
type ctxKV struct {
	RawKV
	ctx context.Context
}

const ctxCheckStride = 1024

func withCtx(kv RawKV, ctx context.Context) RawKV {
	if ctx == nil {
		return kv
	}
	return &ctxKV{RawKV: kv, ctx: ctx}
}

func (c *ctxKV) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return &ctxIter{Iterator: c.RawKV.NewIterator(prefix, start), ctx: c.ctx}
}

type ctxIter struct {
	ethdb.Iterator
	ctx   context.Context
	steps int
	err   error
}

func (i *ctxIter) Next() bool {
	if i.err != nil {
		return false
	}
	i.steps++
	if i.steps%ctxCheckStride == 0 {
		if err := i.ctx.Err(); err != nil {
			i.err = err
			return false
		}
	}
	return i.Iterator.Next()
}

func (i *ctxIter) Error() error {
	if i.err != nil {
		return i.err
	}
	return i.Iterator.Error()
}
