package node

import (
	"testing"
	"time"
)

func TestTakeAction(t *testing.T) {
	node := &Node{}
	node.Start()

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(WaitForStatusUpdate):
			node.SendAction(&Action{t: SyncingActionTest})
		}
	}

	node.SendAction(&Action{t: Done})
}
