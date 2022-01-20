package rpc

import (
	"context"
	"net"

	"github.com/harmony-one/harmony/internal/rate"
)

const (
	// TODO: decide these parameters
	defaultRate      rate.Limit = 0.5
	defaultBurst                = 50
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

func (rl *rateLimiter) waitN(ctx context.Context) (string, error) {
	var (
		ip  string
		err error
	)
	v := ctx.Value("X-Forwarded-For")
	if v != nil {
		ip = v.(string)
	} else {
		hostPort := ctx.Value("remote").(string)
		ip, _, err = net.SplitHostPort(hostPort)
	}
	if err != nil {
		return ip, err
	}

	return ip, rl.il.WaitN(ctx, ip, weightPerRequest)
}
