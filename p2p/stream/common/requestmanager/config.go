package requestmanager

import "time"

// TODO: determine the values in production environment
const (
	// throttle to do request every 100 milliseconds
	throttleInterval = 100 * time.Millisecond

	// StreamMonitorInterval monitors stream connections every 1000 milliseconds
	StreamMonitorInterval = 5000 * time.Millisecond

	// NoStreamTimeout defines no stream timeout duration
	NoStreamTimeout = 60 * time.Second

	// number of request to be done in each throttle loop
	throttleBatch = 16

	// PendingRequestTimeout is the pending request timeout
	PendingRequestTimeout = 180 * time.Second

	// deliverTimeout is the timeout for a response delivery. If the response cannot be delivered
	// within timeout because blocking of the channel, the response will be dropped.
	deliverTimeout = 5 * time.Second

	// maxWaitingSize is the maximum requests that are in waiting list
	maxWaitingSize = 1024
)
