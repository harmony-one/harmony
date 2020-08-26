package pubsub

import (
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
)

// pubSubHost is the host used for pub-sub message handling.
// All requests in pubSubHost are handled in a non-concurrent fashion.
type pubSubHost struct {
	pubSub pubSub

	topicRunners  map[Topic]*topicRunner             // is a map from topic to topicRunner
	topicHandlers map[Topic][]PubSubHandler          // a map from topic to handlers
	handlers      map[HandlerSpecifier]PubSubHandler // a map from specifier to pubSubHandler

	optionProvider ValidateOptionProvider

	log zerolog.Logger
}

func newPubSubHost(pubSub *libp2p_pubsub.PubSub, log zerolog.Logger) *pubSubHost {
	return &pubSubHost{
		pubSub: newPubSubAdapter(pubSub),

		topicRunners:  make(map[Topic]*topicRunner),
		topicHandlers: make(map[Topic][]PubSubHandler),
		handlers:      make(map[HandlerSpecifier]PubSubHandler),

		log: log.With().Str("module", "pub-sub").Logger(),
	}
}

// AddPubSubHandler add a pub sub handler
func (psh *pubSubHost) AddPubSubHandler(handler PubSubHandler) error {
	return nil
}

// StopPubSubHandler stop the pub sub handler with the given specifier.
func (psh *pubSubHost) StopPubSubHandler(spec string) error {
	return nil
}

// RemovePubSubHandler removes a pub sub handler
func (psh *pubSubHost) RemovePubSubHandler(spec string) error {
	return nil
}

func (psh *pubSubHost) StartPubSubHandler(spec string) error {
	return nil
}

func (psh *pubSubHost) addPubSubHandler(task *addHandlerTask) error {
	var (
		handler   = task.handler
		topic     = handler.Topic()
		specifier = handler.Specifier()
	)
	if _, exist := psh.handlers[specifier]; exist {
		return errHandlerAlreadyExist
	}
	if err := psh.addTopicRunnerIfNotExist(topic); err != nil {
		return err
	}
	//psh.handlers[specifier]
	// TODO: implement here
	return nil
}

func (psh *pubSubHost) addTopicRunnerIfNotExist(topic Topic) error {
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

func (psh *pubSubHost) startTopicRunner(topic Topic) error {
	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}
	return tr.start()
}

func (psh *pubSubHost) stopTopicRunner(topic Topic) error {
	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}
	return tr.stop()
}

func (psh *pubSubHost) removeTopicRunner(topic Topic) error {
	tr, err := psh.getTopicRunner(topic)
	if err != nil {
		return err
	}
	return tr.close()
}

func (psh *pubSubHost) getTopicRunner(topic Topic) (*topicRunner, error) {
	tr, exist := psh.topicRunners[topic]
	if !exist {
		return nil, errTopicNotRegistered
	}
	return tr, nil
}
