package service

import (
	"fmt"
	"testing"
	"time"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
)

type SupportSyncingTest struct{}

func (s *SupportSyncingTest) StartService() {
	fmt.Println("SupportSyncingTest starting")
}

func (s *SupportSyncingTest) StopService() {
	fmt.Println("SupportSyncingTest stopping")
}

func (s *SupportSyncingTest) NotifyService(data map[string]interface{}) {
	fmt.Println("SupportSyncingTest being notified")
}
func (s *SupportSyncingTest) SetMessageChan(msgChan chan *msg_pb.Message) {
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
