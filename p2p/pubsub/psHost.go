package pubsub

import (
	"context"
	"errors"
	"sync"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
)

// pubSubHost is the host used for pub-sub message handling.
// All requests in pubSubHost are handled single threaded.
type pubSubHost struct {
	pubSub         rawPubSub
	optionProvider ValidateOptionProvider

	// registered topic and handlers
	topicRunners map[Topic]*topicRunner       // all topicRunners (running & stopped)
	handlers     map[HandlerSpecifier]Handler // all pubSubHandlers (running & stopped)

	// task channels
	addHandlerC    chan addHandlerTask
	startHandlerC  chan startHandlerTask
	stopHandlerC   chan stopHandlerTask
	removeHandlerC chan removeHandlerTask
	removeTopicC   chan removeTopicTask
	closeTaskC     chan closeTask

	// utils
	topicLock sync.RWMutex // lock to protect topicRunners
	log       zerolog.Logger
}

func newPubSubHost(pubSub *libp2p_pubsub.PubSub, optionProvider ValidateOptionProvider, log zerolog.Logger) *pubSubHost {
	return &pubSubHost{
		pubSub:         newPubSubAdapter(pubSub),
		optionProvider: optionProvider,

		topicRunners: make(map[Topic]*topicRunner),
		handlers:     make(map[HandlerSpecifier]Handler),

		addHandlerC:    make(chan addHandlerTask),
		startHandlerC:  make(chan startHandlerTask),
		stopHandlerC:   make(chan stopHandlerTask),
		removeHandlerC: make(chan removeHandlerTask),
		removeTopicC:   make(chan removeTopicTask),
		closeTaskC:     make(chan closeTask),

		log: log.With().Str("module", "pub-sub").Logger(),
	}
}

// psHost tasks. Each task represents a upper function call which is executed in a single
// thread.
type (
	addHandlerTask struct {
		handler Handler
		errC    chan error
	}
	startHandlerTask struct {
		spec HandlerSpecifier
		errC chan error
	}
	stopHandlerTask struct {
		spec HandlerSpecifier
		errC chan error
	}
	removeHandlerTask struct {
		spec HandlerSpecifier
		errC chan error
	}
	removeTopicTask struct {
		topic Topic
		errC  chan error
	}
	closeTask struct {
		closed chan struct{}
	}
)

// Start start the pubSubHost to handler requests (a.k.a. upper function calls).
// All upper function calls are running in a single thread to ensure the race
// condition is not a concern.
func (psh *pubSubHost) Start() {
	go psh.run()
}

func (psh *pubSubHost) run() {
	for {
		select {
		case task := <-psh.addHandlerC:
			task.errC <- psh.addHandler(task.handler)

		case task := <-psh.startHandlerC:
			task.errC <- psh.startHandler(task.spec)

		case task := <-psh.stopHandlerC:
			task.errC <- psh.stopHandler(task.spec)

		case task := <-psh.removeHandlerC:
			task.errC <- psh.removeHandler(task.spec)

		case task := <-psh.removeTopicC:
			task.errC <- psh.removeTopic(task.topic)

		case task := <-psh.closeTaskC:
			close(task.closed)
			return
		}
	}
}

// Close close the pubSubHost.
func (psh *pubSubHost) Close() {
	task := closeTask{closed: make(chan struct{})}
	psh.closeTaskC <- task

	<-task.closed
}

// AddHandler add a Handler to pubSubHost. The Handler is only registered
// but not running. To run the handler, call StartPubSubHandler.
func (psh *pubSubHost) AddHandler(handler Handler) error {
	task := addHandlerTask{
		handler: handler,
		errC:    make(chan error),
	}
	psh.addHandlerC <- task
	return <-task.errC
}

// StartHandler start the Handler specified by spec.
// If the underlying topic is not running, start the topic subscription and run.
func (psh *pubSubHost) StartHandler(spec HandlerSpecifier) error {
	task := startHandlerTask{
		spec: spec,
		errC: make(chan error),
	}
	psh.startHandlerC <- task
	return <-task.errC
}

// StopHandler stop the pub sub handler with the given specifier.
// If the underlying topic has no running handlers, pause it.
func (psh *pubSubHost) StopHandler(spec HandlerSpecifier) error {
	task := stopHandlerTask{
		spec: spec,
		errC: make(chan error),
	}
	psh.stopHandlerC <- task
	return <-task.errC
}

// RemoveHandler removes a pub sub handler with the given specifier.
// If the underlying topic has no more handlers (running & stopped), close
// and remove the topic.
func (psh *pubSubHost) RemoveHandler(spec HandlerSpecifier) error {
	task := removeHandlerTask{
		spec: spec,
		errC: make(chan error),
	}
	psh.removeHandlerC <- task
	return <-task.errC
}

// RemoveTopic close and remove the topic specified with topic.
// All handlers associated with the topic are also removed.
func (psh *pubSubHost) RemoveTopic(topic Topic) error {
	task := removeTopicTask{
		topic: topic,
		errC:  make(chan error),
	}
	psh.removeTopicC <- task
	return <-task.errC
}

// SendMessageToTopic send the message to the given topic.
// TODO: add pb encode rule here, replace msg as raw bytes with encoded pb type
func (psh *pubSubHost) SendMessageToTopic(ctx context.Context, topic Topic, msg []byte) error {
	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}
	return tr.sendMessage(ctx, msg)
}

// addHandler add a Handler to pubSubHost.
// If the topic of the handler is not registered, register it.
func (psh *pubSubHost) addHandler(handler Handler) error {
	var (
		topic     = handler.Topic()
		specifier = handler.Specifier()
	)
	if _, exist := psh.handlers[specifier]; exist {
		return errHandlerAlreadyExist
	}

	if err := psh.addTopicRunnerIfNotExist(topic); err != nil {
		return err
	}

	psh.handlers[specifier] = handler
	return nil
}

// startHandler start the handler with the given specifier.
// If the topic of the handler is not running, run it.
func (psh *pubSubHost) startHandler(spec HandlerSpecifier) error {
	handler, exist := psh.handlers[spec]
	if !exist {
		return errHandlerNotExist
	}
	tr, exist := psh.topicRunners[handler.Topic()]
	if !exist {
		return errTopicNotRegistered
	}

	if err := tr.addHandler(handler); err != nil {
		return err
	}

	if !tr.isRunning() {
		if err := tr.start(); err != nil {
			return err
		}
	}
	return nil
}

// stopHandler stop the handler with the given specifier.
// If the topic of the handler has no more handlers to play with, stop the handler.
func (psh *pubSubHost) stopHandler(spec HandlerSpecifier) error {
	handler, exist := psh.handlers[spec]
	if !exist {
		return errHandlerNotExist
	}
	tr, err := psh.getTopicRunner(handler.Topic())
	if err != nil {
		return err
	}

	if err := tr.removeHandler(spec); err != nil {
		return err
	}

	if len(tr.getHandlers()) == 0 {
		if err := tr.stop(); err != nil {
			return err
		}
	}
	return nil
}

// removeHandler remove the handler with the given specifier.
// If the topic of the handler has no more handlers (running and not running),
// stop listen the topic.
func (psh *pubSubHost) removeHandler(spec HandlerSpecifier) error {
	handler, exist := psh.handlers[spec]
	if !exist {
		return errHandlerNotExist
	}
	topic := handler.Topic()
	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}

	if err := tr.removeHandler(spec); err != nil {
		if !errors.Is(err, errHandlerNotExist) {
			return err
		}
	}

	delete(psh.handlers, spec)
	if psh.haveNoHandlerOnTopic(topic) {
		return psh.closeAndRemoveTopicRunner(topic)
	}
	return nil
}

// removeTopic close the topic and remove all handlers in the topic
func (psh *pubSubHost) removeTopic(topic Topic) error {
	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}
	if err := tr.close(); err != nil {
		return err
	}
	for spec, handler := range psh.handlers {
		if handler.Topic() == topic {
			delete(psh.handlers, spec)
		}
	}
	return nil
}

func (psh *pubSubHost) haveNoHandlerOnTopic(topic Topic) bool {
	for _, handler := range psh.handlers {
		if handler.Topic() == topic {
			return false
		}
	}
	return true
}

func (psh *pubSubHost) addTopicRunnerIfNotExist(topic Topic) error {
	psh.topicLock.Lock()
	defer psh.topicLock.Unlock()

	if _, exist := psh.topicRunners[topic]; exist {
		return nil
	}

	options := psh.optionProvider.getValidateOptions(topic)
	tr, err := newTopicRunner(psh, topic, nil, options)
	if err != nil {
		return err
	}
	psh.topicRunners[topic] = tr
	return nil
}

func (psh *pubSubHost) closeAndRemoveTopicRunner(topic Topic) error {
	psh.topicLock.Lock()
	defer psh.topicLock.Unlock()

	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}
	if err := tr.close(); err != nil {
		return err
	}
	delete(psh.topicRunners, topic)
	return nil
}

func (psh *pubSubHost) getTopicRunner(topic Topic) (*topicRunner, error) {
	psh.topicLock.RLock()
	defer psh.topicLock.RUnlock()

	tr, exist := psh.topicRunners[topic]
	if !exist {
		return nil, errTopicNotRegistered
	}
	return tr, nil
}
