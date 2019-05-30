package consensus

import "time"

// timeout constant
const (
	receiveTimeout time.Duration = 5 * time.Second
	// The duration of viewChangeTimeout; when a view change is initialized with v+1
	// timeout will be equal to viewChangeDuration; if view change failed and start v+2
	// timeout will be 2*viewChangeDuration; timeout of view change v+n is n*viewChangeDuration
	viewChangeDuration time.Duration = 30 * time.Second

	// timeout duration for announce/prepare/commit
	phaseDuration time.Duration = 20 * time.Second
	maxLogSize    uint32        = 1000
)

const (
	timeoutNormal     = "normal"
	timeoutViewChange = "viewchange"
)

// NIL is the m2 type message, which suppose to be nil/empty, however
// we cannot sign on empty message, instead we sign on some default "nil" message
// to indicate there is no prepared message received when we start view change
var NIL = []byte{0x01}
