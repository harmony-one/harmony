package bootnode

import (
	"sync"

	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// nodeStringCounterVec is used to add version string or other static string
	// info into the metrics api
	nodeStringCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "bootnode",
			Name:      "metadata",
			Help:      "a list of boot node metadata",
		},
		[]string{"key", "value"},
	)

	onceMetrics sync.Once
)

func initMetrics() {
	onceMetrics.Do(func() {
		prom.PromRegistry().MustRegister(
			nodeStringCounterVec,
		)
	})
}
