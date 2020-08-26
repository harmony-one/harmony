package pubsub

import libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"

// pubSubAdapter is the adapter for libp2p_pubsub.PubSub
type pubSubAdapter struct {
	raw libp2p_pubsub.PubSub
}

func newPubSubAdapter(raw *libp2p_pubsub.PubSub) *pubSubAdapter {
	return &pubSubAdapter{raw: *raw}
}

// Join joins the given topic in pub-sub
func (ps *pubSubAdapter) Join(topic Topic) (topicHandle, error) {
	handle, err := ps.raw.Join(string(topic))
	if err != nil {
		return nil, err
	}
	return &topicHandleAdapter{*handle}, nil
}

// RegisterTopicValidator register the topic validator for the given topic
func (ps *pubSubAdapter) RegisterTopicValidator(topic Topic, val interface{}, opts ...libp2p_pubsub.ValidatorOpt) error {
	return ps.raw.RegisterTopicValidator(string(topic), val, opts...)
}

// UnregisterTopicValidator unregister the validator for the given topic
func (ps *pubSubAdapter) UnregisterTopicValidator(topic Topic) error {
	return ps.raw.UnregisterTopicValidator(string(topic))
}

// topicHandleAdapter is the adapter for libp2p_pubsub.Topic
type topicHandleAdapter struct {
	raw libp2p_pubsub.Topic
}

// Subscribe subscribe the topic.
func (topic *topicHandleAdapter) Subscribe() (subscription, error) {
	return topic.raw.Subscribe()
}
