package p2p

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
// TODO: Redesign the topics and decouple the message usage according to PubSubHandler so that
//       one topic has only one handler.
type topicRunner struct {
	topic       string
	pubSub      *libp2p_pubsub.PubSub
	topicHandle *libp2p_pubsub.Topic
	options     []libp2p_pubsub.ValidatorOpt

	// all active handlers in the topic; lock protected
	handlers []PubSubHandler
	lock     sync.RWMutex

	validateResultHook func(msg *message, action ValidateAction, err error)

	baseCtx       context.Context
	baseCtxCancel func()
	running       abool.AtomicBool
	closed        abool.AtomicBool
	log           zerolog.Logger
}

func newTopicRunner(host *pubSubHost, topic string, handlers []PubSubHandler, options []libp2p_pubsub.ValidatorOpt) (*topicRunner, error) {
	tr := &topicRunner{
		topic:    topic,
		pubSub:   host.pubsub,
		handlers: handlers,
		options:  options,
		log:      host.log.With().Str("pubSubTopic", topic).Logger(),
	}

	var err error
	tr.topicHandle, err = tr.pubSub.Join(tr.topic)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot join topic [%v]", tr.topic)
	}

	tr.validateResultHook = tr.recordInMetrics
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

	tr.baseCtx, tr.baseCtxCancel = context.WithCancel(context.Background())

	sub, err := tr.prepare()
	if err != nil {
		return
	}

	go tr.run(sub)

	return
}

func (tr *topicRunner) prepare() (*libp2p_pubsub.Subscription, error) {
	sub, err := tr.topicHandle.Subscribe()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot subscribe topic [%v]", tr.topic)
	}
	if err := tr.pubSub.RegisterTopicValidator(tr.topic, tr.validateMsg, tr.options...); err != nil {
		return nil, errors.Wrapf(err, "cannot register topic validator [%v]", tr.topic)
	}
	return sub, nil
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

	tr.validateResultHook(m, action, err)
	return libp2p_pubsub.ValidationResult(action)
}

func (tr *topicRunner) run(sub *libp2p_pubsub.Subscription) {
	defer sub.Cancel()

	for {
		msg, err := sub.Next(tr.baseCtx)
		if err != nil {
			// stop function has been called
			return
		}
		tr.handleMessage(newMessage(msg))
	}
}

func (tr *topicRunner) handleMessage(msg *message) {
	handlers := tr.getHandlers()

	for _, handler := range handlers {
		go tr.deliverMessageForHandler(msg, handler)
	}
}

func (tr *topicRunner) deliverMessageForHandler(msg *message, handler PubSubHandler) {
	validationCache := msg.getHandlerCache(handler.Specifier())
	handler.DeliverMsg(tr.baseCtx, msg.raw.GetData(), validationCache)
}

func (tr *topicRunner) stop() error {
	if changed := tr.running.SetToIf(true, false); !changed {
		return errTopicAlreadyStopped
	}
	tr.baseCtxCancel()
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

func (tr *topicRunner) getHandlers() []PubSubHandler {
	tr.lock.RLock()
	defer tr.lock.RUnlock()

	handlers := make([]PubSubHandler, len(tr.handlers))
	copy(handlers, tr.handlers)

	return handlers
}

func (tr *topicRunner) addHandler(newHandler PubSubHandler) error {
	tr.lock.Lock()
	defer tr.lock.Unlock()

	for _, handler := range tr.handlers {
		if handler.Specifier() == newHandler.Specifier() {
			return errors.Wrapf(errHandlerAlreadyExist, "cannot add handler [%v] at [%v]",
				handler.Specifier(), tr.topic)
		}
	}
	tr.handlers = append(tr.handlers, newHandler)
	return nil
}

func (tr *topicRunner) removeHandler(spec string) error {
	tr.lock.Lock()
	defer tr.lock.Unlock()

	for i, handler := range tr.handlers {
		if handler.Specifier() == spec {
			tr.handlers = append(tr.handlers[:i], tr.handlers[i+1:]...)
			return nil
		}
	}
	return errors.Wrapf(errHandlerNotExist, "cannot remove handler [%v] from [%v]",
		spec, tr.topic)
}

func (tr *topicRunner) recordInMetrics(msg *message, action ValidateAction, err error) {
	// TODO: Log and add metrics here
}
