package prometheus

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"sync"
)

var (
	// Prometheus Pusher
	onceForPusher sync.Once
	pusher        *push.Pusher
	registry      *prometheus.Registry
)

// Pusher returns the pusher, initialized once only
func PromPusher(config PrometheusConfig) *push.Pusher {
	onceForPusher.Do(func() {
		if registry == nil {
			registry = prometheus.NewRegistry()
		}
		pusher = push.New(config.Gateway, fmt.Sprintf("%s/%d", config.Network, config.Shard)).
			Gatherer(registry).
			Grouping("instance", config.Instance)
	})
	return pusher
}

func PromRegistry() *prometheus.Registry {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	return registry
}
