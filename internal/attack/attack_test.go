package attack

import (
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
)

// Simple test for IncorrectResponse
func TestIncorrectResponse(t *testing.T) {
	GetInstance().SetAttackEnabled(false)
	assert.False(t, GetInstance().IncorrectResponse(), "error")
	GetInstance().SetAttackEnabled(true)
}

// Simple test for UpdateConsensusReady
func TestUpdateConsensusReady(t *testing.T) {
	model := GetInstance()
	model.SetLogger(log.New())
	model.NodeKilledByItSelf()

	model.UpdateConsensusReady(model.ConsensusIDThreshold - 1)
	model.DelayResponse()

	model.UpdateConsensusReady(model.ConsensusIDThreshold + 1)
	model.DelayResponse()
}
