package consensus

import "time"

// timeout constant
const (
	receiveTimeout time.Duration = 5 * time.Second
	// The duration of viewChangeTimeout; when a view change is initialized with v+1
	// timeout will be equal to viewChangeDuration; if view change failed and start v+2
	// timeout will be 2*viewChangeDuration; timeout of view change v+n is n*viewChangeDuration
	viewChangeDuration time.Duration = 30 * time.Second

	bootstrapDuration time.Duration = 60 * time.Second

	// timeout duration for announce/prepare/commit
	phaseDuration time.Duration = 20 * time.Second
	maxLogSize    uint32        = 1000
)

// NIL is the m2 type message
var NIL = []byte{0x01}
