package attack

import (
	"testing"

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
	model.NodeKilledByItSelf()

	model.UpdateConsensusReady(model.ViewIDThreshold - 1)
	model.DelayResponse()

	model.UpdateConsensusReady(model.ViewIDThreshold + 1)
	model.DelayResponse()
}
