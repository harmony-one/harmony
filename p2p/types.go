package p2p

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PeerID is an alias to libp2p_peer.ID, basically a string
type PeerID libp2p_peer.ID

// ValidateResult is a wrapper around libp2p_pubsub.ValidationResult. Defines
// what to do with the pub sub message and why do this.
type ValidateResult struct {
	Reason string
	Action ValidateAction
}

// ValidateAction is an encapsulated libp2p_pubsub.ValidationResult which defines
// The action to be taken for the validation of a pubsub message.
type ValidateAction libp2p_pubsub.ValidationResult

const (
	// MsgAccept is an alias to libp2p_pubsub.ValidationAccept
	MsgAccept = ValidateAction(libp2p_pubsub.ValidationAccept)
	// ValidaMsgRejecttionReject is an alias to libp2p_pubsub.ValidationReject
	MsgReject = ValidateAction(libp2p_pubsub.ValidationReject)
	// MsgIgnore is an alias to libp2p_pubsub.ValidationIgnore
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

func mergeValidateResults(vrs []ValidateResult) ValidateResult {
	var (
		msgs []string
		va   = MsgAccept
	)
	for _, vr := range vrs {
		if len(vr.Reason) != 0 {
			msgs = append(msgs, fmt.Sprintf("%s: %v", vr.Action, vr.Reason))
		}
		if vr.Action.Compare(va) > 0 {
			va = vr.Action
		}
	}
	return ValidateResult{
		Reason: strings.Join(msgs, "; "),
		Action: va,
	}
}

// Message is the wrapper of libp2p message
type Message struct {
	raw *libp2p_pubsub.Message
}

// GetRawData get the raw data from libp2p message
func (msg *Message) GetRawData() []byte {
	return msg.raw.GetData()
}

// SetValidatedGlobal set the global values shared among pubSubHandlers. If the given global key
// is already written, return err errGlobalValueOverwrite
func (msg *Message) SetVDataGlobal(key, val interface{}) error {
	vd := msg.getVData()
	err := vd.setGlobal(key, val)
	return errors.Wrapf(err, "set global [%s]=>[%s]", key, val)
}

// MustSetVDataGlobal force set or update the value to be shared among pubSubHandlers.
// Note using this function might result in some global data is overwritten with the same key.
func (msg *Message) MustSetVDataGlobal(key, val interface{}) {
	vd := msg.getVData()
	vd.mustSetGlobal(key, val)
}

func (msg *Message) setValidatorDataByHandler(spec string, data interface{}) {
	vd := msg.getVData()
	vd.setHandlerData(spec, data)
}

func (msg *Message) GetVDataGlobal(key interface{}) interface{} {
	vd := msg.getVData()
	return vd.getGlobal(key)
}

func (msg *Message) getVDataHandler(spec string) interface{} {
	vd := msg.getVData()
	return vd.getHandlerData(spec)
}

func (msg *Message) getVData() *vData {
	if msg.raw.ValidatorData == nil {
		msg.raw.ValidatorData = newVData()
	}
	vd := msg.raw.ValidatorData.(*vData)
	return vd
}

// vData is the data added to Message after validation. It is used as a in-memory cache to
// prevent data computed in the validation process being calculated twice or more.
// vData consist of two parts, one is globals which can be shared among modules. Second is
// data that is used privately by each handler.
type vData struct {
	globals     context.Context
	handlerData map[string]interface{}
}

func newVData() *vData {
	return &vData{
		globals:     context.Background(),
		handlerData: make(map[string]interface{}),
	}
}

func (vd *vData) setGlobal(key, val interface{}) error {
	if vd.globals.Value(key) != nil {
		return errGlobalValueOverwrite
	}
	vd.globals = context.WithValue(vd.globals, key, val)
	return nil
}

func (vd *vData) mustSetGlobal(key, val interface{}) {
	vd.globals = context.WithValue(vd.globals, key, val)
}

func (vd *vData) setHandlerData(spec string, val interface{}) {
	vd.handlerData[spec] = val
}

func (vd *vData) getGlobal(key interface{}) interface{} {
	return vd.globals.Value(key)
}

func (vd *vData) getHandlerData(spec string) interface{} {
	return vd.handlerData[spec]
}
