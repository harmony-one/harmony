package node

// go-ethereum/node/service.go

import (
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/accounts"
)

// ServiceContext is a collection of service independent options inherited from
// the protocol stack, that is passed to all constructors to be optionally used;
// as well as utility methods to operate on the service environment.
type ServiceContext struct {
	// services       map[reflect.Type]Service // Index of the already constructed services
	EventMux       *event.TypeMux    // Event multiplexer used for decoupled notifications
	AccountManager *accounts.Manager // Account manager created by the node.
}
