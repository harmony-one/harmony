package rpc

import (
	"context"
	"net"

	"github.com/harmony-one/harmony/internal/rate"
)

const (
	defaultRate      rate.Limit = 100  // 100 requests per second
	defaultBurst                = 1000 // Burst to 1000 request
	weightPerRequest            = 1
)

// rateLimiter is a wrapper of rate.IDLimiter which serves as the limiter for handling
// RPC requests. The IP field can be obtained from ctx in the module.
type rateLimiter struct {
	il rate.IDLimiter
}

func newRateLimiter() *rateLimiter {
	config := &rate.Config{
		Whitelist: []string{"127.0.0.1", "localhost"},
	}
	return &rateLimiter{
		il: rate.NewLimiterPerID(defaultRate, defaultBurst, config), // use default IDRateLimiter settings
	}
}

func (rl *rateLimiter) waitN(ctx context.Context) error {
	hostPort := ctx.Value("remote").(string)
	ip, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return err
	}
	return rl.il.WaitN(ctx, ip, weightPerRequest)
}
