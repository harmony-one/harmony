package pubsub

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/harmony-one/harmony/common/herrors"
	"github.com/rs/zerolog"
)

const timeBulletFly = 10 * time.Millisecond

var (
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// TestTopicRunner_accept test the scenario of handlers accept all messages
func TestTopicRunner_accept(t *testing.T) {
	host := makeTestPubSubHost()

	delivers, deliverChs := makeDelivers(3)
	validates := makeValidateAcceptFuncs(3)
	handlers := makeFakeHandlers(testTopic, 3, validates, delivers)

	tr, err := newTopicRunner(host, testTopic, handlers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}
	// feed in message should receive in deliverChs
	for i := 0; i != 10; i++ {
		sendTm := testMsg(i)
		if err := host.pubsub.(*fakePubSub).addMessage(testTopic, sendTm); err != nil {
			t.Fatal(err)
		}
		// let the bullet fly
		time.Sleep(timeBulletFly)
		for di, deliverCh := range deliverChs {
			select {
			case cache := <-deliverCh:
				tm, ok := cache.HandlerCache.(testMsg)
				if !ok || !reflect.DeepEqual(tm, sendTm) {
					t.Errorf("%v/%v message unexpected, %+v / %+v", i, di, cache.HandlerCache, sendTm)
				}
			default:
				t.Errorf("%v/%v message not delivered", i, di)
			}
		}
	}
	if err := tr.close(); err != nil {
		t.Fatal(err)
	}
}

// TestTopicRunner_reject test scenario that one of the handlers are rejecting messages
func TestTopicRunner_reject(t *testing.T) {
	host := makeTestPubSubHost()

	delivers, deliverChs := makeDelivers(2)
	validates := []validateFunc{validateAcceptFn, validateRejectFn}
	handlers := makeFakeHandlers(testTopic, 2, validates, delivers)

	tr, err := newTopicRunner(host, testTopic, handlers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i != 10; i++ {
		sendTm := testMsg(i)
		if err := host.pubsub.(*fakePubSub).addMessage(testTopic, sendTm); err != nil {
			t.Fatal(err)
		}
		// let the bullet fly
		time.Sleep(timeBulletFly)
		for di, deliverCh := range deliverChs {
			select {
			case <-deliverCh:
				t.Errorf("%v/%v should not receive rejected message", i, di)
			default:
			}
		}
	}
	if err := tr.close(); err != nil {
		t.Fatal(err)
	}
}

// TestTopicRunner_halfAccept test scenario that half of the message are rejected
func TestTopicRunner_halfAccept(t *testing.T) {
	host := makeTestPubSubHost()

	delivers, deliverChs := makeDelivers(2)
	validates := []validateFunc{validateAcceptFn, validateAcceptHalfFunc}
	handlers := makeFakeHandlers(testTopic, 2, validates, delivers)

	tr, err := newTopicRunner(host, testTopic, handlers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i != 10; i++ {
		sendTm := testMsg(i)
		if err := host.pubsub.(*fakePubSub).addMessage(testTopic, sendTm); err != nil {
			t.Fatal(err)
		}
		// let the bullet fly
		time.Sleep(timeBulletFly)
		for di, deliverCh := range deliverChs {
			if i%2 == 0 {
				// rejected
				select {
				case <-deliverCh:
					t.Errorf("%v/%v should not receive rejected message", i, di)
				default:
				}
			} else {
				// accepted
				select {
				case cache := <-deliverCh:
					tm, ok := cache.HandlerCache.(testMsg)
					if !ok || !reflect.DeepEqual(tm, sendTm) {
						t.Errorf("%v/%v message unexpected, %+v / %+v", i, di, cache.HandlerCache, sendTm)
					}
				default:
					t.Errorf("%v/%v message not delivered", i, di)
				}
			}

		}
	}
	if err := tr.close(); err != nil {
		t.Fatal(err)
	}
}

// TestTopicRunner_dynamicHandler test the scenario of dynamically add or remove handlers
func TestTopicRunner_dynamicHandler(t *testing.T) {
	host := makeTestPubSubHost()
	// Create 2 handlers: 1 accept, 2 reject
	handler1, deliver1 := makeTestHandler(testTopic, 0, validateAcceptFn)
	handler2, deliver2 := makeTestHandler(testTopic, 1, validateRejectFn)

	tr, err := newTopicRunner(host, testTopic, []PubSubHandler{handler1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}
	fps := host.pubsub.(*fakePubSub)
	// Phase 1: only one handler that accept message
	fps.addMessage(testTopic, testMsg(0))
	time.Sleep(timeBulletFly)
	select {
	case <-deliver1:
	default:
		t.Fatal("phase one: did not receive message")
	}

	// Phase 2: one accept handler + one reject handler
	if err := tr.addHandler(handler2); err != nil {
		t.Fatal(err)
	}
	fps.addMessage(testTopic, testMsg(1))
	time.Sleep(timeBulletFly)
	select {
	case <-deliver1:
		t.Fatal("phase two: should not receive message")
	case <-deliver2:
		t.Fatal("phase two: should not receive message")
	default:
	}

	// Phase 3: remove reject handler, accepted message at handler 1
	if err := tr.removeHandler(handler2.Specifier()); err != nil {
		t.Fatal(err)
	}
	fps.addMessage(testTopic, testMsg(2))
	time.Sleep(timeBulletFly)
	select {
	case <-deliver2:
		t.Fatal("phase three: received message from handler 2")
	default:
	}
	select {
	case <-deliver1:
	default:
		t.Fatal("phase three: did not receive message from handler 1")
	}
}

func TestTopicRunner_StartStop(t *testing.T) {
	host := makeTestPubSubHost()
	handler, deliver := makeTestHandler(testTopic, 0, validateAcceptFn)
	fps := host.pubsub.(*fakePubSub)

	tr, err := newTopicRunner(host, testTopic, []PubSubHandler{handler}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}
	// After start, can receive message through handler
	if err := assertTopicRunning(testTopic, fps, deliver, true); err != nil {
		t.Fatalf("initial start: %v", err)
	}
	// Cannot start topic runner twice
	gotErr := tr.start()
	if assErr := herrors.AssertError(gotErr, errTopicAlreadyRunning); assErr != nil {
		t.Fatal(err)
	}

	// stop then start
	if err := tr.stop(); err != nil {
		t.Fatal(err)
	}
	if err := assertTopicRunning(testTopic, fps, deliver, false); err != nil {
		t.Fatalf("after stop: %v", err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}
	if err := assertTopicRunning(testTopic, fps, deliver, true); err != nil {
		t.Fatalf("after restarted: %v", err)
	}

	// close and then start, should have error
	if err := tr.close(); err != nil {
		t.Fatal(err)
	}
	if err := assertTopicRunning(testTopic, fps, deliver, false); err != nil {
		t.Fatalf("after closed: %v", err)
	}
	gotErr = tr.start()
	if assErr := herrors.AssertError(gotErr, errTopicClosed); assErr != nil {
		t.Fatal(assErr)
	}
	if err := assertTopicRunning(testTopic, fps, deliver, false); err != nil {
		t.Fatalf("after closed and try restart: %v", err)
	}
}

// TestTopicRunner_race test the race condition of the topicRunner
func TestTopicRunner_race(t *testing.T) {
	t.SkipNow()

	host := makeTestPubSubHost()

	numHandlers := 10
	delivers, _ := makeDelivers(numHandlers)
	validates := makeValidateAcceptFuncs(numHandlers)
	handlers := makeFakeHandlers(testTopic, numHandlers, validates, delivers)

	tr, err := newTopicRunner(host, testTopic, handlers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}

	// Spawn several goroutines:
	// 3 to feed messages
	// 1 for Add and remove handler

	var (
		stop = make(chan struct{})
		wg   sync.WaitGroup
	)
	wg.Add(4)

	// 3 goroutine to feed messages
	ps := host.pubsub.(*fakePubSub)
	for i := 0; i != 3; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				case <-time.After(10 * time.Millisecond):
				}
				ps.addMessage(testTopic, testMsg(1))
			}
		}()
	}

	// 1 goroutine to add or remove handler
	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
			}
			index := r.Intn(10)
			ope := r.Intn(2)
			if ope == 0 {
				tr.addHandler(handlers[index])
			} else {
				tr.removeHandler(handlers[index].Specifier())
			}
		}
	}()

	time.Sleep(10 * time.Second)
	close(stop)
	wg.Wait()
}

func makeDelivers(num int) ([]deliverFunc, []chan ValidateCache) {
	delivers := make([]deliverFunc, 0, num)
	deliverChs := make([]chan ValidateCache, 0, num)

	for i := 0; i != num; i++ {
		deliver, deliverCh := makeDeliverFunc()

		delivers = append(delivers, deliver)
		deliverChs = append(deliverChs, deliverCh)
	}
	return delivers, deliverChs
}

func makeDeliverFunc() (deliverFunc, chan ValidateCache) {
	deliverCh := make(chan ValidateCache)
	deliver := func(ctx context.Context, rawData []byte, cache ValidateCache) {
		deliverCh <- cache // Yes, the deliver function can be blocking
	}
	return deliver, deliverCh
}

func makeValidateAcceptFuncs(num int) []validateFunc {
	validates := make([]validateFunc, 0, num)
	for i := 0; i != num; i++ {
		validates = append(validates, validateAcceptFn)
	}
	return validates
}

func validateAcceptFn(rawData []byte) ValidateResult {
	msg := decodeTestMsg(rawData)
	return ValidateResult{
		ValidateCache: ValidateCache{
			HandlerCache: msg,
		},
		Action: MsgAccept,
		Err:    nil,
	}
}

func validateRejectFn(rawData []byte) ValidateResult {
	return ValidateResult{
		ValidateCache: ValidateCache{},
		Action:        MsgReject,
		Err:           errors.New("error intended"),
	}
}

// validateAcceptHalfFunc only accept when testMsg is odd
func validateAcceptHalfFunc(rawData []byte) ValidateResult {
	msg := decodeTestMsg(rawData)
	if msg%2 == 0 {
		return ValidateResult{
			ValidateCache: ValidateCache{},
			Action:        MsgReject,
			Err:           errors.New("error intended"),
		}
	}
	return ValidateResult{
		ValidateCache: ValidateCache{
			HandlerCache: msg,
		},
		Action: MsgAccept,
		Err:    nil,
	}
}

func makeTestHandler(topic string, index int, validate validateFunc) (*fakePubSubHandler, chan ValidateCache) {
	deliver, deliverCh := makeDeliverFunc()

	return &fakePubSubHandler{
		topic:    topic,
		index:    index,
		validate: validate,
		deliver:  deliver,
	}, deliverCh
}

func makeTestPubSubHost() *pubSubHost {
	ps := newFakePubSub()
	log := zerolog.New(os.Stdout)

	return &pubSubHost{
		pubsub: ps,
		log:    log,
	}
}

// assertTopicRunning send a probe message to see whether message handling is action
func assertTopicRunning(topic string, pubSub *fakePubSub, deliver chan ValidateCache, running bool) error {
	if err := pubSub.addMessage(topic, testMsg(0)); err != nil {
		return err
	}
	time.Sleep(timeBulletFly)
	if running {
		select {
		case <-deliver:
		default:
			return errors.New("should receive message")
		}
	} else {
		select {
		case <-deliver:
			return errors.New("should not receive message")
		default:
		}
	}
	return nil

}
