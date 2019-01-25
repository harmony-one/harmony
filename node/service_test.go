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
			node.SendAction(&Action{action: Start, serviceType: SyncingSupport})
		}
	}

	node.SendAction(&Action{serviceType: Done})
}
