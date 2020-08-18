package p2p

import (
	"context"
	"sync"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type topicRunner struct {
	topic  string
	pubSub *libp2p_pubsub.PubSub

	// all active handlers in the topic; lock protected
	handlers []PubSubHandler

	stopCh chan stopTopicTask
	lock   sync.RWMutex
	log    zerolog.Logger
}

func newTopicRunner(host *pubSubHost, topic string, handlers []PubSubHandler) *topicRunner {
	return &topicRunner{
		topic:    topic,
		pubSub:   host.pubsub,
		handlers: handlers,
		stopCh:   make(chan stopTopicTask, 1),
		log:      host.log.With().Str("pubSubTopic", topic).Logger(),
	}
}

func (tr *topicRunner) start() error {
	topicHandle, err := tr.pubSub.Join(tr.topic)
	if err != nil {
		return errors.Wrapf(err, "cannot join topic [%v]", tr.topic)
	}
	sub, err := topicHandle.Subscribe()
	if err != nil {
		return errors.Wrapf(err, "cannot subscribe topic [%v]", tr.topic)
	}
	if err := tr.pubSub.RegisterTopicValidator(tr.topic, tr.validateMsg, tr.getPubSubOptions()...); err != nil {
		return errors.Wrapf(err, "cannot register topic validator [%v]", tr.topic)
	}
	go tr.run(sub)

	return nil
}

func (tr *topicRunner) run(sub *libp2p_pubsub.Subscription) {
	for {
		nextMsg, err := sub.Next(context.Background())
		// TODO: start here
	}
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
	tr.handleValidateResult(m, finalRes)

	return libp2p_pubsub.ValidationResult(finalRes.Action)
}

func (tr *topicRunner) getPubSubOptions() []libp2p_pubsub.ValidatorOpt {
	handlers := tr.getHandlers()
	var opts []libp2p_pubsub.ValidatorOpt

	for _, handler := range handlers {
		opts = append(opts, handler.Options()...)
	}
	return opts
}

func (tr *topicRunner) getHandlers() []PubSubHandler {
	tr.lock.RLock()
	defer tr.lock.RUnlock()

	handlers := make([]PubSubHandler, len(tr.handlers))
	copy(handlers, tr.handlers)

	return handlers
}

func (tr *topicRunner) handleValidateResult(msg *Message, vRes ValidateResult) {
	// TODO: Add metrics here
}
