package pubsub

import "github.com/pkg/errors"

var (
	errTopicNotRegistered  = errors.New("topic runner not registered")
	errTopicAlreadyRunning = errors.New("topic is already running")
	errTopicAlreadyStopped = errors.New("topic has already stopped")
	errTopicClosed         = errors.New("topic has been closed")
	errHandlerAlreadyExist = errors.New("handler has already been registered")
	errHandlerNotExist     = errors.New("handler not exist in topic")
)
