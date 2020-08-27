package pubsub

import (
	"github.com/harmony-one/harmony/common/herrors"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
)

// PeerID is an alias to libp2p_peer.ID, basically a string
type PeerID libp2p_peer.ID

// HandlerSpecifier is a unique string for Handler
type HandlerSpecifier string

// Topic is the pub-sub subscribed topic, basically a string
type Topic string

// ValidateAction is an encapsulated libp2p_pubsub.ValidationResult which defines
// The action to be taken for the validation of a pubSub message.
type ValidateAction libp2p_pubsub.ValidationResult

const (
	// MsgAccept is an alias to libp2p_pubsub.ValidationAccept
	MsgAccept = ValidateAction(libp2p_pubsub.ValidationAccept)
	// ValidaMsgRejecttionReject is an alias to libp2p_pubsub.ValidationReject.
	// The action indicates an invalid message that should not be delivered to
	// the application or forwarded to the application. Furthermore the peer that
	// forwarded the message should be penalized by peer scoring routers.
	MsgReject = ValidateAction(libp2p_pubsub.ValidationReject)
	// MsgIgnore is an alias to libp2p_pubsub.ValidationIgnore.
	// This action indicates a message that should be ignored: it will be neither
	// delivered to the application nor forwarded to the network. However, in
	// contrast to ValidationReject, the peer that forwarded the message must not
	//be penalized by peer scoring routers.
	MsgIgnore = ValidateAction(libp2p_pubsub.ValidationIgnore)
)

// Compare defines the priority of ValidateAction vs target ValidateAction.
// Return 1 if larger, -1 if smaller, and 0 if equal.
// The compare rule is: MsgAccept < MsgIgnore < MsgReject.
// The compare rule is used by merging ValidateResults. Higher priority will
// Override low priority actions.
func (va ValidateAction) Compare(val ValidateAction) int {
	p1 := getValidateActionPriority(va)
	p2 := getValidateActionPriority(val)
	if p1 > p2 {
		return 1
	}
	if p1 < p2 {
		return -1
	}
	return 0
}

func getValidateActionPriority(va ValidateAction) int {
	switch va {
	case MsgAccept:
		return 0
	case MsgIgnore:
		return 1
	case MsgReject:
		return 2
	default:
	}
	return -1
}

// String return the string representation of the ValidateAction
func (va ValidateAction) String() string {
	switch va {
	case MsgAccept:
		return "accepted"
	case MsgReject:
		return "rejected"
	case MsgIgnore:
		return "ignored"
	default:
	}
	return "unknown validation result"
}

// ValidateResult is the result returned by validateMsg function by handlers.
type ValidateResult struct {
	// ValidateCache is stored in ValidateResult which will be further used in
	// DeliverMsg.
	ValidateCache

	// Action is the action to be taken for a specific message. Available Actions:
	// MsgAccept, MsgIgnore, MsgReject
	Action ValidateAction

	// Err is the error happened during the validation process
	Err error
}

// ValidateCache is the data structure for storing the cache in the process of
// handler ValidateMsg.
type ValidateCache struct {
	// GlobalCache is the data stored as cache that can be shared among different
	// handlers.
	// WARN: write GlobalCache after validation is forbidden since it will result in
	// potential race condition and inconsistent handling results.
	// TODO: abstract this cache to a getter / setter interface and separate use
	//   to protect global data during message processing in different handlers.
	GlobalCache map[string]interface{}

	// HandlerCache is the data stored as cache for further handling the message.
	HandlerCache interface{}
}

func mergeValidateResults(handlers []Handler, vrs []ValidateResult) (vData, ValidateAction, error) {
	var (
		cache  = newVData()
		action = MsgAccept
		errs   []error
	)
	for i, vr := range vrs {
		handler := handlers[i]
		if vr.Err != nil {
			err := errors.Wrapf(vr.Err, "%v", handlers[i].Specifier())
			errs = append(errs, err)
		}
		if vr.Action.Compare(action) > 0 {
			action = vr.Action
		}
		for k, v := range vr.GlobalCache {
			cache.globals[k] = v
		}
		cache.handlerData[handler.Specifier()] = vr.HandlerCache
	}
	return cache, action, herrors.Join(errs...)
}

// message is the wrapper of libp2p message. It handles the underlying validatorData
// as the cache.
type message struct {
	raw *libp2p_pubsub.Message
}

func newMessage(raw *libp2p_pubsub.Message) *message {
	return &message{raw}
}

func (msg *message) setValidateCache(cache vData) {
	msg.raw.ValidatorData = cache
}

func (msg *message) getHandlerCache(spec HandlerSpecifier) ValidateCache {
	vd := msg.getVData()

	return ValidateCache{
		GlobalCache:  vd.globals,
		HandlerCache: vd.handlerData[spec],
	}
}

func (msg *message) getVData() vData {
	if msg.raw.ValidatorData == nil {
		msg.raw.ValidatorData = newVData()
	}
	vd := msg.raw.ValidatorData.(vData)
	return vd
}

// vData is the data added to message after validation. It is used as a in-memory cache to
// prevent data computed in the validation process being calculated twice or more.
// vData consist of two parts, one is globals which can be shared among modules. Second is
// data that is used privately by each handler.
type vData struct {
	globals     map[string]interface{}
	handlerData map[HandlerSpecifier]interface{}
}

func newVData() vData {
	return vData{
		globals:     make(map[string]interface{}),
		handlerData: make(map[HandlerSpecifier]interface{}),
	}
}
