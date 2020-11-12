package utils

import (
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
func PromPusher(pubkey string) *push.Pusher {
	onceForPusher.Do(func() {
		if registry == nil {
			registry = prometheus.NewRegistry()
		}
		pusher = push.New("https://gateway.harmony.one", "hmy").
			Gatherer(registry).
			Grouping("instance", pubkey)
	})
	return pusher
}

func PromRegistry() *prometheus.Registry {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	return registry
}
