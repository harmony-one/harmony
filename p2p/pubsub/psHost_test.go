package pubsub

import "testing"

var (
	testTopic1 = Topic("test topic1")
	testTopic2 = Topic("test topic2")
	testTopic3 = Topic("test topic3")
)

// makeTestPsHost makes a psHost for test.
// The host has
//   1. Running topic testTopic1 with 2 handlers; first is running
//   2. Stopped topic testTopic2 with 1 handler
func makeTestPsHost(t *testing.T) *pubSubHost {
	host := &pubSubHost{
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
	host.Start()

	handler1, _ := makeTestHandler(testTopic1, 0, validateAcceptFn)
	handler2, _ := makeTestHandler(testTopic1, 1, validateAcceptFn)
	handler3, _ := makeTestHandler(testTopic2, 2, validateAcceptFn)

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
	return host
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
		host := makeTestPsHost(t)

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
		host := makeTestPsHost(t)

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

func makeTestHandlerNoDeliver(topic Topic, index int, validate validateFunc) Handler {
	handler, _ := makeTestHandler(topic, index, validate)
	return handler
}
