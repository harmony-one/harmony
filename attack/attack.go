package attack

import (
	"harmony-benchmark/log"
	"math/rand"
	"os"
	"time"
)

const (
	DroppingTickDuration  = 2 * time.Second
	AttackEnabled         = false
	HitRate               = 10
	DelayResponseDuration = 10 * time.Second
)

// AttackModel contains different models of attacking.
type Attack struct {
	log log.Logger // Log utility
}

func New(log log.Logger) *Attack {
	attackModel := Attack{}
	// Logger
	attackModel.log = log
	return &attackModel
}

// Run runs all attack models in goroutine mode.
func (attack *Attack) Run() {
	if !AttackEnabled {
		return
	}
	// Adding attack model here.
	go func() {
		attack.NodeKilledByItSelf()
	}()
}

// NodeKilledByItSelf runs killing itself attack
func (attack *Attack) NodeKilledByItSelf() {
	tick := time.Tick(DroppingTickDuration)
	for {
		<-tick
		if rand.Intn(HitRate) == 0 {
			attack.log.Debug("***********************Killing myself***********************", "PID: ", os.Getpid())
			os.Exit(1)
		}
	}
}

func DelayResponse() {
	if !AttackEnabled {
		return
	}
	if rand.Intn(HitRate) == 0 {
		time.Sleep(DelayResponseDuration)
	}
}
