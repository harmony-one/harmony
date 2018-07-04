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
	IncorrectTransaction
)

// AttackModel contains different models of attacking.
type Attack struct {
	AttackEnabled        bool
	attackType           AttackType
	ConsensusIdThreshold int
	readyByConsensus     bool
	log                  log.Logger // Log utility
}

var attack *Attack
var once sync.Once

// GetAttackModel returns attack model by using singleton pattern.
func GetAttackModel() *Attack {
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
		attack.ConsensusIdThreshold = ConsensusIdThresholdMin + rand.Intn(ConsensusIdThresholdMax-ConsensusIdThresholdMin)
	}
}

func (attack *Attack) SetLogger(log log.Logger) {
	attack.log = log
}

// NodeKilledByItSelf runs killing itself attack
func (attack *Attack) NodeKilledByItSelf() {
	if !attack.AttackEnabled || attack.attackType != DelayResponse || !attack.readyByConsensus {
		return
	}

	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******Killing myself*******", "PID: ", os.Getpid())
		os.Exit(1)
	}
}

func (attack *Attack) DelayResponse() {
	if !attack.AttackEnabled || attack.attackType != DelayResponse || !attack.readyByConsensus {
		return
	}
	if rand.Intn(HitRate) == 0 {
		time.Sleep(DelayResponseDuration)
	}
}

func (attack *Attack) UpdateConsensusReady(consensusId int) {
	if consensusId > attack.ConsensusIdThreshold {
		attack.readyByConsensus = true
	}
}
