package p2p

import (
	"sync"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
)

// pubSubHost is the host used for pubsub message handling
type pubSubHost struct {
	pubsub *libp2p_pubsub.PubSub

	activeHandlers map[string][]PubSubHandler // a map from topic to running handlers
	handlers       map[string]PubSubHandler   // a map from handler specifier to the handler
	stopped        map[string]PubSubHandler   // a map from handler specifier to stopped handler
	handlerTopics  map[string]string          // a map from handler specifier to the topic

	lock sync.RWMutex
	log  *zerolog.Logger
}

func newPubSubHost(pubsub *libp2p_pubsub.PubSub) *pubSubHost {
	return &pubSubHost{
		pubsub: pubsub,

		activeHandlers: make(map[string][]PubSubHandler),
		handlers:       make(map[string]PubSubHandler),
		stopped:        make(map[string]PubSubHandler),
		handlerTopics:  make(map[string]string),
	}
}

// AddPubSubHandler add a pub sub handler
func (psh *pubSubHost) AddPubSubHandler(handler PubSubHandler) error {
	psh.lock.Lock()
	defer psh.lock.Unlock()

	spec := handler.Specifier()
	if _, ok := psh.handlers[spec]; ok {
		return errPubSubRegistered
	}

	topicHandlers := psh.activeHandlers[handler.Topic()]
	psh.activeHandlers[handler.Topic()] = append(topicHandlers, handler)
	psh.handlers[spec] = handler
	psh.stopped[spec] = handler
	psh.handlerTopics[spec] = handler.Topic()

	return nil
}

// StopPubSubHandler stop the pub sub handler with the given specifier.
// TODO: also cancel all running contexts. Need to implement cancel feature in each
//       PubSubHandler
func (psh *pubSubHost) StopPubSubHandler(spec string) error {
	psh.lock.Lock()
	defer psh.lock.Unlock()

	if _, ok := psh.handlers[spec]; !ok {
		return errPubSubNotRegistered
	}
	if _, ok := psh.stopped[spec]; ok {
		return errPubSubStopped
	}
	return nil
}

// stopPubSubHandler stop a pub sub handler. Lock is assumed at caller.
func (psh *pubSubHost) stopPubSubHandler(spec string) error {
	topic := psh.handlerTopics[spec]
	for i, handler := range psh.activeHandlers[topic] {
		if handler.Specifier() != spec {
			continue
		}
		raw := psh.activeHandlers[topic]
		psh.stopped[spec] = handler
		psh.activeHandlers[topic] = append(raw[:i], raw[i+1:]...)
		return nil
	}
	return errPubSubNotActive
}

// RemovePubSubHandler removes a pub sub handler
// TODO: context level management. Cancel all on-going message handlers
func (psh *pubSubHost) RemovePubSubHandler(spec string) error {
	psh.lock.Lock()
	defer psh.lock.Unlock()

	if _, ok := psh.handlers[spec]; !ok {
		return errPubSubNotRegistered
	}
	return nil
}

func (psh *pubSubHost) StartPubSubHandler(spec string) error {
	return nil
}
