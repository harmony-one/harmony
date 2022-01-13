package rpc

import "github.com/harmony-one/harmony/internal/rate"

const (
	testRate  rate.Limit = 1000000
	testBurst            = 100000
)

func getTestRateLimiter() *rateLimiter {
	config := &rate.Config{
		Whitelist: []string{"127.0.0.1", "localhost"},
	}
	return &rateLimiter{
		il: rate.NewLimiterPerID(testRate, testBurst, config), // use default IDRateLimiter settings
	}
}
