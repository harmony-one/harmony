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

func TestTakeAction(t *testing.T) {
	node := &Node{}
	node.Start()
	node.RegisterService(SupportSyncing, &SupportSyncingTest{})

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(WaitForStatusUpdate):
			node.SendAction(&Action{action: Start, serviceType: SupportSyncing})
		}
	}

	node.SendAction(&Action{serviceType: Done})
}
