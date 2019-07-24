package consensus

import "time"

// timeout constant
const (
	// The duration of viewChangeTimeout; when a view change is initialized with v+1
	// timeout will be equal to viewChangeDuration; if view change failed and start v+2
	// timeout will be 2*viewChangeDuration; timeout of view change v+n is n*viewChangeDuration
	viewChangeDuration time.Duration = 60 * time.Second

	// timeout duration for announce/prepare/commit
	phaseDuration     time.Duration = 60 * time.Second
	bootstrapDuration time.Duration = 300 * time.Second
	maxLogSize        uint32        = 1000
	// threshold between received consensus message blockNum and my blockNum
	consensusBlockNumBuffer uint64 = 2
	// ReProposeBlockNumShard0: leader propose this block from it's database and validators accept them
	ReProposeBlockNumShard0 uint64 = 270108
	// ReProposeBlockNumShard1: leader propose this block from it's database and validators accept them
	ReProposeBlockNumShard1 uint64 = 275559
	// ReProposeBlockNumShard2: leader propose this block from it's database and validators accept them
	ReProposeBlockNumShard2 uint64 = 275057
	// ReProposeBlockNumShard3: leader propose this block from it's database and validators accept them
	ReProposeBlockNumShard3 uint64 = 276805
)

// TimeoutType is the type of timeout in view change protocol
type TimeoutType int

const (
	timeoutConsensus TimeoutType = iota
	timeoutViewChange
	timeoutBootstrap
)

var (
	// NIL is the m2 type message, which suppose to be nil/empty, however
	// we cannot sign on empty message, instead we sign on some default "nil" message
	// to indicate there is no prepared message received when we start view change
	NIL       = []byte{0x01}
	startTime time.Time
)
