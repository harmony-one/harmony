package attack

import (
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/harmony-one/harmony/log"
)

// Constants used for attack model.
const (
	DroppingTickDuration    = 2 * time.Second
	HitRate                 = 10
	DelayResponseDuration   = 10 * time.Second
	ConsensusIDThresholdMin = 10
	ConsensusIDThresholdMax = 100
)

// Type is the type of attack model.
type Type byte

// Constants of different attack models.
const (
	KilledItself Type = iota
	DelayResponse
	IncorrectResponse
)

// Model contains different models of attacking.
type Model struct {
	AttackEnabled             bool
	attackType                Type
	ConsensusIDThreshold      uint32
	readyByConsensusThreshold bool
	log                       log.Logger // Log utility
}

var attackModel *Model
var once sync.Once

// GetInstance returns attack model by using singleton pattern.
func GetInstance() *Model {
	once.Do(func() {
		attackModel = &Model{}
		attackModel.Init()
	})
	return attackModel
}

// Init initializes attack model.
func (attack *Model) Init() {
	attack.AttackEnabled = false
	attack.readyByConsensusThreshold = false
}

// SetAttackEnabled sets attack model enabled.
func (attack *Model) SetAttackEnabled(AttackEnabled bool) {
	attack.AttackEnabled = AttackEnabled
	if AttackEnabled {
		attack.attackType = Type(rand.Intn(3))
		attack.ConsensusIDThreshold = uint32(ConsensusIDThresholdMin + rand.Intn(ConsensusIDThresholdMax-ConsensusIDThresholdMin))
	}
}

// SetLogger sets the logger for doing logging.
func (attack *Model) SetLogger(log log.Logger) {
	attack.log = log
}

// Run runs enabled attacks.
func (attack *Model) Run() {
	attack.NodeKilledByItSelf()
	attack.DelayResponse()
}

// NodeKilledByItSelf runs killing itself attack
func (attack *Model) NodeKilledByItSelf() {
	if !attack.AttackEnabled || attack.attackType != KilledItself || !attack.readyByConsensusThreshold {
		return
	}

	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******************Killing myself******************", "PID: ", os.Getpid())
		os.Exit(1)
	}
}

// DelayResponse does attack by delaying response.
func (attack *Model) DelayResponse() {
	if !attack.AttackEnabled || attack.attackType != DelayResponse || !attack.readyByConsensusThreshold {
		return
	}
	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******************Model: DelayResponse******************", "PID: ", os.Getpid())
		time.Sleep(DelayResponseDuration)
	}
}

// IncorrectResponse returns if the attack model enable incorrect responding.
func (attack *Model) IncorrectResponse() bool {
	if !attack.AttackEnabled || attack.attackType != IncorrectResponse || !attack.readyByConsensusThreshold {
		return false
	}
	if rand.Intn(HitRate) == 0 {
		attack.log.Debug("******************Model: IncorrectResponse******************", "PID: ", os.Getpid())
		return true
	}
	return false
}

// UpdateConsensusReady enables an attack type given the current consensusID.
func (attack *Model) UpdateConsensusReady(consensusID uint32) {
	if consensusID > attack.ConsensusIDThreshold {
		attack.readyByConsensusThreshold = true
	}
}
