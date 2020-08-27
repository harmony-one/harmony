package pubsub

import (
	"context"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PubSub is the interface for pubSubHost.
type PubSub interface {
	// handler management
	AddHandler(psh Handler) error
	StartHandler(spec HandlerSpecifier) error
	StopHandler(spec HandlerSpecifier) error
	RemoveHandler(spec HandlerSpecifier) error
	RemoveTopic(topic Topic) error

	// message publish
	// TODO: Add encode algorithm with protobuf. Change msg type raw bytes to encoded pb message.
	SendMessageToTopic(ctx context.Context, topic Topic, msg []byte) error

	// control
	Start()
	Close()
}

// Handler is the pub sub message handler of a certain topic
// TODO: Add version string to topic to enable compatibility
// TODO: add decode algorithm with protobuf. Change arg in ValidateMsg and DeliverMsg from
//       []byte to decoded message
type Handler interface {
	// Topic is the topic the handler is subscribed to
	Topic() Topic

	// Specifier defines the uniques specifier of a pub sub handler
	Specifier() HandlerSpecifier

	// ValidateMsg validate the message from the peer with PeerID. Return ValidateResult.
	// Cache in the result is parsed to DeliverMsg and can be used for further message handling.
	ValidateMsg(ctx context.Context, peer PeerID, rawData []byte) ValidateResult

	// DeliverMsg deliver the message to target object with the validationCache from ValidateMsg
	// Note: For the same handler under a topic, DeliverMsg are executed concurrently.
	// And the error should be handled in HandleMsg for each handler.
	DeliverMsg(ctx context.Context, rawData []byte, cache ValidateCache)
}

// ValidateOptionProvider is the interface to provide pub-sub option with respect to the topic
type ValidateOptionProvider interface {
	getValidateOptions(topic Topic) []libp2p_pubsub.ValidatorOpt
}

// rawPubSub is the interface for libp2p PubSub with the adapter
type rawPubSub interface {
	Join(topic Topic) (topicHandle, error)
	RegisterTopicValidator(topic Topic, val interface{}, opts ...libp2p_pubsub.ValidatorOpt) error
	UnregisterTopicValidator(topic Topic) error
}

// topicHandle is the interface for libp2p_pubsub.Topic
type topicHandle interface {
	Subscribe() (subscription, error)
	Publish(ctx context.Context, data []byte) error
}

// subscription is the interface for libp2p_pubsub.Subscription
type subscription interface {
	Next(ctx context.Context) (*libp2p_pubsub.Message, error)
	Cancel()
}
