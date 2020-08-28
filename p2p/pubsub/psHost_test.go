package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	testTopic1 = Topic("test topic1")
	testTopic2 = Topic("test topic2")
	testTopic3 = Topic("test topic3")
)

// makeTestPsHost makes a psHost for test.
// The host has
//   1. Running topic testTopic1 with 2 handlers; first is running
//   2. Stopped topic testTopic2 with 1 handler
func makeTestPsHost(t *testing.T) (*pubSubHost, []chan ValidateCache) {
	host := makeEmptyTestHost()
	host.Start()

	handler1, deliver1 := makeTestHandler(testTopic1, 0, validateAcceptFn)
	handler2, deliver2 := makeTestHandler(testTopic1, 1, validateAcceptFn)
	handler3, deliver3 := makeTestHandler(testTopic2, 2, validateAcceptFn)

	if err := host.AddHandler(handler1); err != nil {
		t.Fatal(err)
	}
	if err := host.AddHandler(handler2); err != nil {
		t.Fatal(err)
	}
	if err := host.AddHandler(handler3); err != nil {
		t.Fatal(err)
	}
	if err := host.startHandler(makeSpecifier(0)); err != nil {
		t.Fatal(err)
	}
	return host, []chan ValidateCache{deliver1, deliver2, deliver3}
}

func makeEmptyTestHost() *pubSubHost {
	return &pubSubHost{
		pubSub:         newFakePubSub(),
		optionProvider: &emptyValidateOptionProvider{},
		topicRunners:   make(map[Topic]*topicRunner),
		handlers:       make(map[HandlerSpecifier]Handler),
		addHandlerC:    make(chan addHandlerTask),
		startHandlerC:  make(chan startHandlerTask),
		stopHandlerC:   make(chan stopHandlerTask),
		removeHandlerC: make(chan removeHandlerTask),
		removeTopicC:   make(chan removeTopicTask),
		closeTaskC:     make(chan closeTask),
	}
}

func TestPubSubHost_AddHandler(t *testing.T) {
	tests := []struct {
		handler         Handler
		expTopicRunning bool
		expErr          error
	}{
		{
			handler:         makeTestHandlerNoDeliver(testTopic1, 3, validateAcceptFn),
			expTopicRunning: true,
			expErr:          nil,
		},
		{
			handler:         makeTestHandlerNoDeliver(testTopic2, 3, validateAcceptFn),
			expTopicRunning: false,
			expErr:          nil,
		},
		{
			handler:         makeTestHandlerNoDeliver(testTopic3, 3, validateAcceptFn),
			expTopicRunning: false,
			expErr:          nil,
		},
		{
			handler: makeTestHandlerNoDeliver(testTopic3, 1, validateAcceptFn),
			expErr:  errHandlerAlreadyExist,
		},
	}
	for i, test := range tests {
		host, _ := makeTestPsHost(t)

		err := host.AddHandler(test.handler)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil {
			continue
		}

		tr, err := host.getTopicRunner(test.handler.Topic())
		if err != nil {
			t.Fatalf("Test %v: topic not registered after add handler", i)
		}
		if tr.isRunning() != test.expTopicRunning {
			t.Errorf("Test %v: topic is running unexpected %v / %v", i, tr.isRunning(), test.expTopicRunning)
		}
		for _, handler := range tr.getHandlers() {
			if handler.Specifier() == test.handler.Specifier() {
				t.Errorf("Test %v: after addHandler, handler already running in topic", i)
			}
		}
	}
}

func TestPubSubHost_StartHandler(t *testing.T) {
	tests := []struct {
		spec   HandlerSpecifier
		expErr error
	}{
		{
			spec:   makeSpecifier(1),
			expErr: nil,
		},
		{
			spec:   makeSpecifier(2),
			expErr: nil,
		},
		{
			spec:   makeSpecifier(0),
			expErr: errHandlerAlreadyExist,
		},
		{
			spec:   makeSpecifier(3),
			expErr: errHandlerNotExist,
		},
	}
	for i, test := range tests {
		host, _ := makeTestPsHost(t)

		err := host.StartHandler(test.spec)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil {
			continue
		}

		topic := host.handlers[test.spec].Topic()
		tr, err := host.getTopicRunner(topic)
		if err != nil {
			t.Errorf("Test %v: topic [%v] not exist", i, topic)
		}
		if !tr.isRunning() {
			t.Errorf("Test %v: topic [%v] not running", i, topic)
		}
	}
}

func TestPubSubHost_StopHandler(t *testing.T) {
	tests := []struct {
		psModifier func(host *pubSubHost)
		spec       HandlerSpecifier

		expTopicRunning bool
		expErr          error
	}{
		{
			// After stop handler, topic is also stopped since no more running handlers
			psModifier:      nil,
			spec:            makeSpecifier(0),
			expTopicRunning: false,
			expErr:          nil,
		},
		{
			// After stop handler, topic is not stopped
			psModifier: func(host *pubSubHost) {
				if err := host.startHandler(makeSpecifier(1)); err != nil {
					t.Fatal(err)
				}
			},
			spec:            makeSpecifier(0),
			expTopicRunning: true,
			expErr:          nil,
		},
		{
			spec:   makeSpecifier(1),
			expErr: errHandlerNotExist,
		},
		{
			spec:   makeSpecifier(3),
			expErr: errHandlerNotExist,
		},
	}
	for i, test := range tests {
		host, _ := makeTestPsHost(t)
		if test.psModifier != nil {
			test.psModifier(host)
		}

		err := host.StopHandler(test.spec)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil {
			continue
		}

		handler, exist := host.handlers[test.spec]
		if !exist {
			t.Errorf("Test %v: after stop, handler removed from psHost", i)
		}
		tr, err := host.getTopicRunner(handler.Topic())
		if err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
		if tr.isRunning() != test.expTopicRunning {
			t.Errorf("Test %v: topic running unexpected: %v/ %v", i, tr.isRunning(), test.expTopicRunning)
		}
		for _, handler := range tr.getHandlers() {
			if handler.Specifier() == test.spec {
				t.Errorf("Test %v: after stop, handler still running", i)
			}
		}
	}
}

func TestPubSubHost_RemoveHandler(t *testing.T) {
	tests := []struct {
		psModifier func(host *pubSubHost)
		spec       HandlerSpecifier

		expErr       error
		checkTopic   Topic
		topicRemoved bool
		topicStopped bool
	}{
		{
			// After remove handler, topic is stopped
			spec:         makeSpecifier(0),
			expErr:       nil,
			checkTopic:   testTopic1,
			topicRemoved: false,
			topicStopped: true,
		},
		{
			// After removing a already stopped handler, topic still running
			spec:         makeSpecifier(1),
			expErr:       nil,
			checkTopic:   testTopic1,
			topicRemoved: false,
			topicStopped: false,
		},
		{
			// After removing an active handler, topic still running since still another running handler
			psModifier: func(host *pubSubHost) {
				if err := host.startHandler(makeSpecifier(1)); err != nil {
					t.Fatal(err)
				}
			},
			spec:         makeSpecifier(0),
			expErr:       nil,
			checkTopic:   testTopic1,
			topicRemoved: false,
			topicStopped: false,
		},
		{
			// Removing the only handler of the topic, the topic is closed
			spec:         makeSpecifier(2),
			expErr:       nil,
			checkTopic:   testTopic2,
			topicRemoved: true,
		},
		{
			spec:   makeSpecifier(3),
			expErr: errHandlerNotExist,
		},
	}
	for i, test := range tests {
		host, _ := makeTestPsHost(t)
		if test.psModifier != nil {
			test.psModifier(host)
		}
		tr, _ := host.topicRunners[test.checkTopic]

		err := host.RemoveHandler(test.spec)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil {
			continue
		}

		if _, exist := host.handlers[test.spec]; exist {
			t.Fatalf("Test %v: after remove handler still exist", i)
		}
		_, exist := host.topicRunners[test.checkTopic]
		if exist == test.topicRemoved {
			t.Fatalf("Test %v: topic removed %v / %v", i, !exist, test.topicRemoved)
		}
		if tr.closed.IsSet() != test.topicRemoved {
			t.Fatalf("Test %v: topic closed %v / %v", i, tr.closed.IsSet(), test.topicRemoved)
		}
		if test.topicRemoved {
			continue
		}
		if tr.isRunning() == test.topicStopped {
			t.Fatalf("Test %v: topic stopped unexpected: %v / %v", i, !tr.isRunning(), test.topicStopped)
		}
		for _, handler := range tr.getHandlers() {
			if handler.Specifier() == test.spec {
				t.Fatalf("Test %v: handler still in topic after remove", i)
			}
		}
	}
}

func TestPubSubHost_RemoveTopic(t *testing.T) {
	tests := []struct {
		topic           Topic
		removedHandlers []HandlerSpecifier
		expErr          error
	}{
		{
			topic:           testTopic1,
			removedHandlers: []HandlerSpecifier{makeSpecifier(0), makeSpecifier(1)},
			expErr:          nil,
		},
		{
			topic:           testTopic2,
			removedHandlers: []HandlerSpecifier{makeSpecifier(2)},
			expErr:          nil,
		},
		{
			topic:  testTopic3,
			expErr: errTopicNotRegistered,
		},
	}
	for i, test := range tests {
		host, _ := makeTestPsHost(t)
		tr, _ := host.topicRunners[test.topic]

		err := host.removeTopic(test.topic)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil {
			continue
		}

		if _, exist := host.topicRunners[test.topic]; exist {
			t.Errorf("Test %v: topic not removed", i)
		}
		if !tr.closed.IsSet() {
			t.Errorf("Test %v: topic not closed", i)
		}
	}
}

// TestPubSubHost_Race test the race condition of the pubSubHost
func TestPubSubHost_Race(t *testing.T) {
	t.Skip("skipping race tests that takes some time")

	testDuration := 10 * time.Second

	host, delivers := makeTestPsHost(t)
	defer host.Close()

	var wg sync.WaitGroup

	// spawn goroutines to consume delivers
	stop := make(chan struct{})
	numDelivered := make([]uint64, len(delivers))
	wg.Add(len(delivers))
	for i, deliver := range delivers {
		go func(deliver chan ValidateCache, index int) {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				case <-deliver:
					atomic.AddUint64(&numDelivered[index], 1)
				}

			}
		}(deliver, i)
	}
	defer func() {
		for i := range numDelivered {
			t.Logf("received %v message from topic [%s]", numDelivered[i], makeSpecifier(i))
		}
	}()

	// Spawn goroutines to feed messages
	topics := []Topic{testTopic1, testTopic2}
	msgReceived := make([]uint64, len(topics))
	wg.Add(len(topics))
	for i, topic := range topics {
		go func(topic Topic, index int) {
			defer wg.Done()
			tick := time.Tick(10 * time.Millisecond)

			for {
				select {
				case <-stop:
					return
				case <-tick:
					host.pubSub.(*fakePubSub).addMessage(topic, testMsg(1))
					atomic.AddUint64(&msgReceived[index], 1)
				}
			}
		}(topic, i)
	}
	defer func() {
		for i, topic := range topics {
			t.Logf("feed %v external messages at topic [%s]", msgReceived[i], topic)
		}
	}()

	// Spawn goroutines to publish message
	msgPublished := make([]uint64, len(topics))
	wg.Add(len(msgPublished))
	for i, topic := range topics {
		go func(topic Topic, index int) {
			defer wg.Done()

			tick := time.Tick(10 * time.Millisecond)

			for {
				select {
				case <-stop:
					return
				case <-tick:
					host.SendMessageToTopic(context.Background(), topic, testMsg(1).encode())
					atomic.AddUint64(&msgPublished[index], 1)
				}
			}
		}(topic, i)
	}
	defer func() {
		for i, topic := range topics {
			t.Logf("published %v messages at topic [%s]", msgPublished[i], topic)
		}
	}()

	// The scheduler goroutine to add, start, and stop handler.
	initialStarts := []bool{true, false, false}
	specifiers := []HandlerSpecifier{makeSpecifier(0), makeSpecifier(1), makeSpecifier(2)}
	wg.Add(len(initialStarts))
	for i := range specifiers {
		go func(index int) {
			defer wg.Done()

			started := initialStarts[index]
			tick := time.Tick(200 * time.Millisecond)

			for {
				select {
				case <-stop:
					return
				case <-tick:
					if started {
						host.StopHandler(makeSpecifier(index))
					} else {
						host.StartHandler(makeSpecifier(index))
					}
					started = !started
				}
			}
		}(i)
	}

	time.Sleep(testDuration)
	close(stop)
	wg.Wait()
}

func makeTestHandlerNoDeliver(topic Topic, index int, validate validateFunc) Handler {
	handler, _ := makeTestHandler(topic, index, validate)
	return handler
}
