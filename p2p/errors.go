package p2p

import "github.com/pkg/errors"

var (
	errPubSubRegistered    = errors.New("handler already registered")
	errPubSubNotRegistered = errors.New("handler not registered")
	errPubSubStopped       = errors.New("handler already stopped")
	errPubSubNotActive     = errors.New("handler not active")
	errPubSubStarted       = errors.New("handler already started")

	errGlobalValueOverwrite = errors.New("try to overwrite global val")

	errTopicAlreadyRunning = errors.New("topic is already running")
	errTopicAlreadyStopped = errors.New("topic has already stopped")

	errUnknown = errors.New("unknown error")
)
