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
			Namespace: "rpc",
			Subsystem: "",
			Name:      "rpc_over_ratelimit_trigger",
			Help:      "number of times triggered rpc rate limit",
		},
		[]string{"trigger_info"})
)
