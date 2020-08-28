package pubsub

import (
	"context"
	"sync"

	"github.com/harmony-one/abool"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// topicRunner runs the message handlers on a specific topic.
// Currently, a topic runner is combined with multiple PubSubHandlers.
// TODO: Redesign the topics and decouple the message usage according to Handler so that
//       one topic has only one handler.
type topicRunner struct {
	topic       Topic
	pubSub      rawPubSub
	topicHandle topicHandle
	options     []libp2p_pubsub.ValidatorOpt

	// all active handlers in the topic; handlerLock protected
	handlers    []Handler
	handlerLock sync.RWMutex

	ctxCancel func() // since upper function calls are single-threaded, there is no race here.

	metric   *psMetric
	running  abool.AtomicBool
	closed   abool.AtomicBool
	stopC    chan struct{}
	stoppedC chan struct{}
	log      zerolog.Logger
}

func newTopicRunner(host *pubSubHost, topic Topic, handlers []Handler, options []libp2p_pubsub.ValidatorOpt) (*topicRunner, error) {
	tr := &topicRunner{
		topic:    topic,
		pubSub:   host.pubSub,
		handlers: handlers,
		options:  options,
		stoppedC: make(chan struct{}),
		log:      host.log.With().Str("pubSubTopic", string(topic)).Logger(),
	}

	tr.metric = newPsMetric(topic, defaultMetricInterval, tr.log)

	var err error
	tr.topicHandle, err = tr.pubSub.Join(tr.topic)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot join topic [%v]", tr.topic)
	}
	if err := tr.pubSub.RegisterTopicValidator(tr.topic, tr.validateMsg, tr.options...); err != nil {
		return nil, errors.Wrapf(err, "cannot register topic validator [%v]", tr.topic)
	}

	return tr, nil
}

func (tr *topicRunner) start() (err error) {
	if changed := tr.running.SetToIf(false, true); !changed {
		return errTopicAlreadyRunning
	}
	if tr.closed.IsSet() {
		return errTopicClosed
	}
	defer func() {
		if err != nil {
			tr.running.SetTo(false)
		}
	}()

	sub, err := tr.topicHandle.Subscribe()
	if err != nil {
		return errors.Wrapf(err, "cannot subscribe topic [%v]", tr.topic)
	}

	ctx, cancel := context.WithCancel(context.Background())
	tr.ctxCancel = cancel

	go tr.metric.run()
	go tr.run(ctx, sub)

	return
}

func (tr *topicRunner) validateMsg(ctx context.Context, peer PeerID, raw *libp2p_pubsub.Message) libp2p_pubsub.ValidationResult {
	m := newMessage(raw)
	handlers := tr.getHandlers()
	vResults := make([]ValidateResult, 0, len(handlers))

	for _, handler := range handlers {
		vRes := handler.ValidateMsg(ctx, peer, m.raw.GetData())
		vResults = append(vResults, vRes)
	}
	cache, action, err := mergeValidateResults(handlers, vResults)
	m.setValidateCache(cache)

	tr.recordValidateResult(m, action, err)
	return libp2p_pubsub.ValidationResult(action)
}

func (tr *topicRunner) run(ctx context.Context, sub subscription) {
	defer func() {
		sub.Cancel()
		tr.stoppedC <- struct{}{}
	}()

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			// baseCtx has been canceled
			return
		}
		tr.handleMessage(ctx, newMessage(msg))
	}
}

func (tr *topicRunner) handleMessage(ctx context.Context, msg *message) {
	handlers := tr.getHandlers()

	for _, handler := range handlers {
		tr.deliverMessageForHandler(ctx, msg, handler)
	}
}

func (tr *topicRunner) deliverMessageForHandler(ctx context.Context, msg *message, handler Handler) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	validationCache := msg.getHandlerCache(handler.Specifier())
	go handler.DeliverMsg(ctx, msg.raw.GetData(), validationCache)
}

func (tr *topicRunner) stop() error {
	if changed := tr.running.SetToIf(true, false); !changed {
		return errTopicAlreadyStopped
	}

	tr.ctxCancel()
	tr.metric.stop()
	<-tr.stoppedC
	return nil
}

func (tr *topicRunner) close() error {
	if changed := tr.closed.SetToIf(false, true); !changed {
		return errTopicClosed
	}
	if err := tr.stop(); err != nil {
		if err != errTopicAlreadyStopped {
			return err
		}
	}
	if err := tr.pubSub.UnregisterTopicValidator(tr.topic); err != nil {
		return errors.Wrapf(err, "failed to unregister topic %v", tr.topic)
	}
	return nil
}

func (tr *topicRunner) isRunning() bool {
	return tr.running.IsSet()
}

func (tr *topicRunner) getHandlers() []Handler {
	tr.handlerLock.RLock()
	defer tr.handlerLock.RUnlock()

	handlers := make([]Handler, len(tr.handlers))
	copy(handlers, tr.handlers)

	return handlers
}

func (tr *topicRunner) isHandlerRunning(specifier HandlerSpecifier) bool {
	tr.handlerLock.RLock()
	defer tr.handlerLock.RUnlock()

	for _, handler := range tr.handlers {
		if handler.Specifier() == specifier {
			return true
		}
	}
	return false
}

func (tr *topicRunner) addHandler(newHandler Handler) error {
	tr.handlerLock.Lock()
	defer tr.handlerLock.Unlock()

	for _, handler := range tr.handlers {
		if handler.Specifier() == newHandler.Specifier() {
			return errors.Wrapf(errHandlerAlreadyExist, "cannot add handler [%v] at [%v]",
				handler.Specifier(), tr.topic)
		}
	}
	tr.handlers = append(tr.handlers, newHandler)
	return nil
}

func (tr *topicRunner) removeHandler(spec HandlerSpecifier) error {
	tr.handlerLock.Lock()
	defer tr.handlerLock.Unlock()

	for i, handler := range tr.handlers {
		if handler.Specifier() == spec {
			tr.handlers = append(tr.handlers[:i], tr.handlers[i+1:]...)
			return nil
		}
	}
	return errors.Wrapf(errHandlerNotExist, "cannot remove handler [%v] from [%v]",
		spec, tr.topic)
}

func (tr *topicRunner) recordValidateResult(msg *message, action ValidateAction, err error) {
	// log in metric non-block
	go tr.metric.recordValidateResult(msg, action, err)
}

func (tr *topicRunner) sendMessage(ctx context.Context, msg []byte) (err error) {
	return tr.topicHandle.Publish(ctx, msg)
}
