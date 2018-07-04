package attack

import (
	"harmony-benchmark/log"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	DroppingTickDuration    = 2 * time.Second
	AttackEnabled           = false
	HitRate                 = 10
	DelayResponseDuration   = 10 * time.Second
	ConsensusIdThresholdMin = 10
	ConsensusIdThresholdMax = 100
)

type AttackType byte

const (
	KilledItself AttackType = iota
	DelayResponse
	IncorrectResponse
)

// AttackModel contains different models of attacking.
type Attack struct {
	AttackEnabled        bool
	attackType           AttackType
	ConsensusIdThreshold uint32
	readyByConsensus     bool
	log                  log.Logger // Log utility
}

var attack *Attack
var once sync.Once

// GetInstance returns attack model by using singleton pattern.
func GetInstance() *Attack {
	once.Do(func() {
		attack = &Attack{}
		attack.Init()
	})
	return attack
}

func (attack *Attack) Init() {
	attack.AttackEnabled = AttackEnabled
	attack.readyByConsensus = false
}

func (attack *Attack) SetAttackEnabled(AttackEnabled bool) {
	attack.AttackEnabled = AttackEnabled
	if AttackEnabled {
		attack.attackType = AttackType(rand.Intn(3))
		attack.ConsensusIdThreshold = uint32(ConsensusIdThresholdMin + rand.Intn(ConsensusIdThresholdMax-ConsensusIdThresholdMin))
	}
}

func (attack *Attack) SetLogger(log log.Logger) {
	attack.log = log
}

func (attack *Attack) Run() {
	attack.NodeKilledByItSelf()
	attack.DelayResponse()
}

// NodeKilledByItSelf runs killing itself attack
func (attack *Attack) NodeKilledByItSelf() {
	if !attack.AttackEnabled || attack.attackType != KilledItself || !attack.readyByConsensus {
		return
	}

	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******************Killing myself******************", "PID: ", os.Getpid())
		os.Exit(1)
	}
}

func (attack *Attack) DelayResponse() {
	if !attack.AttackEnabled || attack.attackType != DelayResponse || !attack.readyByConsensus {
		return
	}
	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******************Attack: DelayResponse******************", "PID: ", os.Getpid())
		time.Sleep(DelayResponseDuration)
	}
}

func (attack *Attack) IncorrectResponse() bool {
	if !attack.AttackEnabled || attack.attackType != IncorrectResponse || !attack.readyByConsensus {
		return false
	}
	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******************Attack: IncorrectResponse******************", "PID: ", os.Getpid())
		return true
	}
	return false
}

func (attack *Attack) UpdateConsensusReady(consensusId uint32) {
	if consensusId > attack.ConsensusIdThreshold {
		attack.readyByConsensus = true
	}
}
