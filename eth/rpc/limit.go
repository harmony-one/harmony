package rpc

import (
	"context"

	"github.com/harmony-one/harmony/internal/rate"
)

const (
	defaultRate      = 100  // 100 requests per second
	defaultBurst     = 1000 // Burst to 1000 request
	weightPerRequest = 1
)

type limiter interface {
	WaitN(ctx context.Context, n int) error
}

// httpServerLimiter is a wrapper of rate.IDLimiter which serves as the limiter for handling
// single RPC requests. The IP field is obtainable from ctx
// in the module
type httpServerLimiter rate.IDLimiter

func (*httpServerLimiter) WaitN(ctx context.Context, n int) error {
	remote := 
}
