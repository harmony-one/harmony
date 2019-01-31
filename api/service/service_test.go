package service

import (
	"fmt"
	"testing"
	"time"
)

type SupportSyncingTest struct{}

func (s *SupportSyncingTest) StartService() {
	fmt.Println("SupportSyncingTest starting")
}

func (s *SupportSyncingTest) StopService() {
	fmt.Println("SupportSyncingTest stopping")
}

// Test TakeAction.
func TestTakeAction(t *testing.T) {
	store := &Store{}
	store.SetupServiceManager()
	store.RegisterService(SupportSyncing, &SupportSyncingTest{})

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(WaitForStatusUpdate):
			store.SendAction(&Action{action: Start, serviceType: SupportSyncing})
		}
	}

	store.SendAction(&Action{serviceType: Done})
}
