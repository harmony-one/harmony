package streammanager

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamManager_SubscribeAddStreamEvent(t *testing.T) {
	sm := newTestStreamManager()

	addStreamEvtC := make(chan EvtStreamAdded, 1)
	sub := sm.SubscribeAddStreamEvent(addStreamEvtC)
	defer sub.Unsubscribe()
	stopC := make(chan struct{}, 1)

	var numStreamAdded uint32
	go func() {
		for {
			select {
			case <-addStreamEvtC:
				atomic.AddUint32(&numStreamAdded, 1)
			case <-stopC:
				return
			}
		}
	}()

	sm.Start()
	time.Sleep(defTestWait)
	close(stopC)
	sm.Close()

	if atomic.LoadUint32(&numStreamAdded) != 16 {
		t.Errorf("numStreamAdded unexpected")
	}
}

func TestStreamManager_SubscribeRemoveStreamEvent(t *testing.T) {
	sm := newTestStreamManager()

	rmStreamEvtC := make(chan EvtStreamRemoved, 1)
	sub := sm.SubscribeRemoveStreamEvent(rmStreamEvtC)
	defer sub.Unsubscribe()
	stopC := make(chan struct{}, 1)

	var numStreamRemoved uint32
	go func() {
		for {
			select {
			case <-rmStreamEvtC:
				atomic.AddUint32(&numStreamRemoved, 1)
			case <-stopC:
				return
			}
		}
	}()

	sm.Start()
	time.Sleep(defTestWait)

	err := sm.RemoveStream(makeStreamID(1))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(defTestWait)
	close(stopC)
	sm.Close()

	if atomic.LoadUint32(&numStreamRemoved) != 1 {
		t.Errorf("numStreamAdded unexpected")
	}
}
