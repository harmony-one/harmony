// package p2p
package p2p

import (
	eth_metrics "github.com/ethereum/go-ethereum/metrics"
	"github.com/libp2p/go-libp2p/core/metrics"
)

const (
	// ingressMeterName is the prefix of the per-packet inbound metrics.
	ingressMeterName = "p2p/ingress"

	// egressMeterName is the prefix of the per-packet outbound metrics.
	egressMeterName = "p2p/egress"
)

var (
	ingressTrafficMeter = eth_metrics.NewRegisteredMeter(ingressMeterName, nil)
	egressTrafficMeter  = eth_metrics.NewRegisteredMeter(egressMeterName, nil)
)

// Counter is a wrapper around a metrics.BandwidthCounter that meters both the
// inbound and outbound network traffic.
type Counter struct {
	*metrics.BandwidthCounter
}

func newCounter() *Counter {
	return &Counter{metrics.NewBandwidthCounter()}
}

func (c *Counter) LogRecvMessage(size int64) {
	ingressTrafficMeter.Mark(size)
}

func (c *Counter) LogSentMessage(size int64) {
	egressTrafficMeter.Mark(size)
}
