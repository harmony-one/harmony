package p2p

import (
	"context"
	"sync"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// topicRunner runs the message handlers on a given topic.
// Currently, a topic runner is combined with multiple PubSubHandlers.
// TODO: Redesign the topics and decouple the message usage according to PubSubHandler so that
//       one topic has only one handler.
// TODO: Make options can be set dynamically during runtime.
type topicRunner struct {
	topic  string
	pubSub *libp2p_pubsub.PubSub

	// all active handlers in the topic; lock protected
	handlers []PubSubHandler
	options  []libp2p_pubsub.ValidatorOpt

	validateResultHook func(msg *Message, result ValidateResult)

	baseCtx context.Context
	cancel  func()
	stopCh  chan stopTopicTask
	lock    sync.RWMutex
	log     zerolog.Logger
}

func newTopicRunner(host *pubSubHost, topic string, handlers []PubSubHandler, options []libp2p_pubsub.ValidatorOpt) *topicRunner {
	tr := &topicRunner{
		topic:    topic,
		pubSub:   host.pubsub,
		handlers: handlers,
		options:  options,
		stopCh:   make(chan stopTopicTask, 1),
		log:      host.log.With().Str("pubSubTopic", topic).Logger(),
	}
	tr.validateResultHook = tr.recordInMetrics
	return tr
}

func (tr *topicRunner) start() error {
	sub, err := tr.subscribe()
	if err != nil {
		return err
	}
	if err := tr.registerValidations(); err != nil {
		return err
	}

	go tr.run(sub)

	return nil
}

func (tr *topicRunner) subscribe() (*libp2p_pubsub.Subscription, error) {
	topicHandle, err := tr.pubSub.Join(tr.topic)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot join topic [%v]", tr.topic)
	}
	sub, err := topicHandle.Subscribe()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot subscribe topic [%v]", tr.topic)
	}
	return sub, nil
}

func (tr *topicRunner) registerValidations() error {
	if err := tr.pubSub.RegisterTopicValidator(tr.topic, tr.validateMsg, tr.options...); err != nil {
		return errors.Wrapf(err, "cannot register topic validator [%v]", tr.topic)
	}
	return nil
}

func (tr *topicRunner) run(sub *libp2p_pubsub.Subscription) {
	//for {
	//	ctx := context.WithDeadline(tr.baseCtx, 10*time.Second)
	//	nextMsg, err := sub.Next(context.Background())
	//	// TODO: start here
	//}
}

func (tr *topicRunner) stop(ctx context.Context) error {
	errC := make(chan error)

	tr.stopCh <- stopTopicTask{
		errC: errC,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errC:
		return err
	}
}

func (tr *topicRunner) validateMsg(ctx context.Context, peer PeerID, msg *libp2p_pubsub.Message) libp2p_pubsub.ValidationResult {
	m := &Message{msg}
	handlers := tr.getHandlers()
	vResults := make([]ValidateResult, len(handlers))

	for _, handler := range handlers {
		vCache, vRes := handler.ValidateMsg(ctx, peer, m)
		m.setValidatorDataByHandler(handler.Specifier(), vCache)
		vResults = append(vResults, vRes)
	}

	finalRes := mergeValidateResults(vResults)
	tr.recordInMetrics(m, finalRes)

	return libp2p_pubsub.ValidationResult(finalRes.Action)
}

func (tr *topicRunner) getHandlers() []PubSubHandler {
	tr.lock.RLock()
	defer tr.lock.RUnlock()

	handlers := make([]PubSubHandler, len(tr.handlers))
	copy(handlers, tr.handlers)

	return handlers
}

func (tr *topicRunner) recordInMetrics(msg *Message, vRes ValidateResult) {
	// TODO: Add metrics here
}
