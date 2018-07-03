package attack

import (
	"harmony-benchmark/log"
	"math/rand"
	"os"
	"sync"
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
	AttackEnabled bool
	log           log.Logger // Log utility
}

var attack *Attack
var once sync.Once

// GetAttackModel returns attack model by using singleton pattern.
func GetAttackModel() *Attack {
	once.Do(func() {
		attack = &Attack{}
		attack.AttackEnabled = AttackEnabled
	})
	return attack
}

func (attack *Attack) SetLogger(log log.Logger) {
	attack.log = log
}

// Run runs all attack models in goroutine mode.
func (attack *Attack) Run() {
	if !attack.AttackEnabled {
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

func (attack *Attack) DelayResponse() {
	if !attack.AttackEnabled {
		return
	}
	if rand.Intn(HitRate) == 0 {
		time.Sleep(DelayResponseDuration)
	}
}
