package consensus_test

import (
	"testing"

	"github.com/harmony-one/harmony/consensus"
)

func TestState_SetBlockNum(t *testing.T) {
	state := consensus.NewState(consensus.Normal, 0)
	if state.GetBlockNum() == 1 {
		t.Errorf("GetBlockNum expected not to be 1")
	}
	state.SetBlockNum(1)
	if state.GetBlockNum() != 1 {
		t.Errorf("SetBlockNum failed")
	}
}
