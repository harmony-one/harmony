package attack

import (
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// Constants used for attack model.
const (
	DroppingTickDuration  = 2 * time.Second
	HitRate               = 10
	DelayResponseDuration = 10 * time.Second
	ViewIDThresholdMin    = 10
	ViewIDThresholdMax    = 100
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
	ViewIDThreshold           uint64
	readyByConsensusThreshold bool
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
		attack.ViewIDThreshold = uint64(ViewIDThresholdMin + rand.Intn(ViewIDThresholdMax-ViewIDThresholdMin))
	}
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
		utils.GetLogInstance().Debug("******************Killing myself******************", "PID: ", os.Getpid())
		os.Exit(1)
	}
}

// DelayResponse does attack by delaying response.
func (attack *Model) DelayResponse() {
	if !attack.AttackEnabled || attack.attackType != DelayResponse || !attack.readyByConsensusThreshold {
		return
	}
	if rand.Intn(HitRate) == 0 {
		utils.GetLogInstance().Debug("******************Model: DelayResponse******************", "PID: ", os.Getpid())
		time.Sleep(DelayResponseDuration)
	}
}

// IncorrectResponse returns if the attack model enable incorrect responding.
func (attack *Model) IncorrectResponse() bool {
	if !attack.AttackEnabled || attack.attackType != IncorrectResponse || !attack.readyByConsensusThreshold {
		return false
	}
	if rand.Intn(HitRate) == 0 {
		utils.GetLogInstance().Debug("******************Model: IncorrectResponse******************", "PID: ", os.Getpid())
		return true
	}
	return false
}

// UpdateConsensusReady enables an attack type given the current viewID.
func (attack *Model) UpdateConsensusReady(viewID uint64) {
	if viewID > attack.ViewIDThreshold {
		attack.readyByConsensusThreshold = true
	}
}
