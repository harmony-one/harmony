package consensus

import "time"

// timeout constant
const (
	// default timeout configuration is shorten to 27 seconds as the consensus is 5s
	// so each phase of the consensus will timeout every 27 seconds to tirgger a
	// new view change process
	viewChangeTimeout = 27
	// viewChangeSlot means every 45 seconds, the view change ID will be advanced.
	// so that the nodes init view change process within the 45 seconds range will
	// be have the same view change ID
	viewChangeSlot = 45
	// The duration of viewChangeTimeout for each view change
	viewChangeDuration time.Duration = viewChangeTimeout * time.Second

	// timeout duration for announce/prepare/commit
	// shorten the duration from 60 to 45 seconds as the consensus is 5s
	phaseDuration     time.Duration = viewChangeTimeout * time.Second
	bootstrapDuration time.Duration = 120 * time.Second
	maxLogSize        uint32        = 1000
	// threshold between received consensus message blockNum and my blockNum
	consensusBlockNumBuffer uint64 = 2
)

// TimeoutType is the type of timeout in view change protocol
type TimeoutType int

const (
	timeoutConsensus TimeoutType = iota
	timeoutViewChange
	timeoutBootstrap
)

func (t TimeoutType) String() string {
	switch t {
	case timeoutConsensus:
		return "timeoutConsensus"
	case timeoutViewChange:
		return "timeoutViewChange"
	case timeoutBootstrap:
		return "timeoutBootstrap"
	}
	return "unknown"
}

var (
	// NIL is the m2 type message, which suppose to be nil/empty, however
	// we cannot sign on empty message, instead we sign on some default "nil" message
	// to indicate there is no prepared message received when we start view change
	NIL       = []byte{0x01}
	startTime time.Time
)
