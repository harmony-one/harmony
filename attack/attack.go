package attack

import (
	"harmony-benchmark/consensus"
	"harmony-benchmark/log"
	"time"
)

const (
	DroppingTimerDuration = 2 * time.Second
)

// AttackModel contains different models of attacking.
type Attack struct {
	log log.Logger // Log utility
}

func New(consensus *consensus.Consensus) *Attack {
	attackModel := Attack{}
	// Logger
	attackModel.log = consensus.Log
	return &attackModel
}

func (attack *Attack) Run() {
	// Adding attack model here.
	go func() {
		// TODO(minhdoan): Enable it later after done attacking.
		// attack.NodeKilledByItSelf()
	}()
}

// NodeKilledByItSelf
// Attack #1 in the doc.
func (attack *Attack) NodeKilledByItSelf() {
	tick := time.Tick(DroppingTimerDuration)
	for {
		<-tick
		attack.log.Debug("**********************************killed itself**********************************")
	}
}
