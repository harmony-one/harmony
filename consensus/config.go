package consensus

import "time"

const (
	// blockDuration is the period a node try to publish a new block if it's leader
	blockDuration  time.Duration = 5 * time.Second
	receiveTimeout time.Duration = 5 * time.Second
	maxLogSize     uint32        = 1000
)
