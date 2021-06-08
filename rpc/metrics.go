package rpc

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		rpcRateLimitCounterVec,
	)
}

var (
	rpcRateLimitCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "rpc",
			Name:      "over_ratelimit",
			Help:      "number of times triggered rpc rate limit",
		},
		[]string{"rate_limit"})
)
