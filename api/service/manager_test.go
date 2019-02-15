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
	m := &Manager{}
	m.SetupServiceManager()
	m.RegisterService(SupportSyncing, &SupportSyncingTest{})

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			m.SendAction(&Action{Action: Start, ServiceType: SupportSyncing})
		}
	}

	m.SendAction(&Action{ServiceType: Done})
}
