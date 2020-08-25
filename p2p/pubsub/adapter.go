package pubsub

import libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"

// pubSubAdapter is the adapter for libp2p_pubsub.PubSub
type pubSubAdapter struct {
	raw libp2p_pubsub.PubSub
}

// Join joins the given topic in pub-sub
func (ps *pubSubAdapter) Join(topic string) (psTopic, error) {
	handle, err := ps.raw.Join(topic)
	if err != nil {
		return nil, err
	}
	return &topicAdapter{*handle}, nil
}

// RegisterTopicValidator register the topic validator for the given topic
func (ps *pubSubAdapter) RegisterTopicValidator(topic string, val interface{}, opts ...libp2p_pubsub.ValidatorOpt) error {
	return ps.raw.RegisterTopicValidator(topic, val, opts...)
}

// UnregisterTopicValidator unregister the validator for the given topic
func (ps *pubSubAdapter) UnregisterTopicValidator(topic string) error {
	return ps.raw.UnregisterTopicValidator(topic)
}

// topicAdapter is the adapter for libp2p_pubsub.Topic
type topicAdapter struct {
	raw libp2p_pubsub.Topic
}

// Subscribe subscribe the topic.
func (topic *topicAdapter) Subscribe() (subscription, error) {
	return topic.raw.Subscribe()
}
