package node

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// ActionType ...
type ActionType byte

// Constants for Action Type.
const (
	Start ActionType = iota
	Stop
)

// Type is service type.
type Type byte

// Constants for Type.
const (
	SupportSyncing Type = iota
	SupportClient
	SupportExplorer
	Test
	Done
)

func (t Type) String() string {
	switch t {
	case SupportClient:
		return "SupportClient"
	case SupportSyncing:
		return "SyncingSupport"
	case SupportExplorer:
		return "SupportExplorer"
	case Test:
		return "Test"
	case Done:
		return "Done"
	default:
		return "Unknown"
	}
}

// Constants for timing.
const (
	// WaitForStatusUpdate is the delay time to update new status. Currently set 1 second for development. Should be 30 minutes for production.
	WaitForStatusUpdate = time.Second * 1
)

// Action is type of service action.
type Action struct {
	action      ActionType
	serviceType Type
	params      map[string]interface{}
}

// ServiceInterface ...
type ServiceInterface interface {
	Start()
	Stop()
}

// ServiceStore stores all services for service manager.
type ServiceStore struct {
	services map[Type]ServiceInterface
}

// Register ...
func (ss *ServiceStore) Register(t Type, service ServiceInterface) {
	if ss.services == nil {
		ss.services = make(map[Type]ServiceInterface)
	}
	ss.services[t] = service
}

// Start node.
func (node *Node) Start() {
	node.RegisterServices()
	node.actionChannel = node.StartServiceManager()
}

// RegisterService is used for testing.
func (node *Node) RegisterService(t Type, service ServiceInterface) {
	node.serviceStore.Register(t, service)
}

// RegisterServices ...
func (node *Node) RegisterServices() {
	node.serviceStore = &ServiceStore{services: make(map[Type]ServiceInterface)}
	// services := node.serviceStore
	// services.Register(SupportSyncing, node.SupportSyncing)
}

// SendAction ...
func (node *Node) SendAction(action *Action) {
	node.actionChannel <- action
}

// TakeAction ...
func (node *Node) TakeAction(action *Action) {
	if node.serviceStore == nil {
		utils.GetLogInstance().Error("Service store is not initialized.")
		return
	}
	if service, ok := node.serviceStore.services[action.serviceType]; ok {
		switch action.action {
		case Start:
			fmt.Printf("Start %s\n", action.serviceType)
			service.Start()
		case Stop:
			fmt.Printf("Stop %s\n", action.serviceType)
			service.Stop()
		}
	}
}

// StartServiceManager ...
func (node *Node) StartServiceManager() chan *Action {
	ch := make(chan *Action)
	go func() {
		for {
			select {
			case action := <-ch:
				node.TakeAction(action)
				if action.serviceType == Done {
					return
				}
			case <-time.After(WaitForStatusUpdate):
				utils.GetLogInstance().Info("Waiting for new action.")
			}
		}
	}()
	return ch
}
