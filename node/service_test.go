package node

import (
	"fmt"
	"testing"
	"time"
)

type SupportSyncingTest struct{}

func (s *SupportSyncingTest) Start() {
	fmt.Println("SupportSyncingTest starting")
}

func (s *SupportSyncingTest) Stop() {
	fmt.Println("SupportSyncingTest stopping")
}

// Test TakeAction.
func TestTakeAction(t *testing.T) {
	node := &Node{}
	node.SetupServiceManager()
	node.RegisterService(SupportSyncing, &SupportSyncingTest{})

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(WaitForStatusUpdate):
			node.SendAction(&Action{action: Start, serviceType: SupportSyncing})
		}
	}

	node.SendAction(&Action{serviceType: Done})
}
