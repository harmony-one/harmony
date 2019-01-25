package node

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// ActionType is the input for Service Manager to operate.
type ActionType byte

// Constants for Action Type.
const (
	Start ActionType = iota
	Stop
)

// ServiceType is service type.
type ServiceType byte

// Constants for Type.
const (
	SupportSyncing ServiceType = iota
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

// ServiceInterface is the collection of functions any service needs to implement.
type ServiceInterface interface {
	Start()
	Stop()
}

// ServiceStore stores all services for service manager.
type ServiceStore struct {
	services map[Type]ServiceInterface
}

// Register new service to service store.
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

// RegisterServices registers all service for a node with its role.
func (node *Node) RegisterServices() {
	node.serviceStore = &ServiceStore{services: make(map[Type]ServiceInterface)}
}

// SendAction sends action to action channel which is observed by service manager.
func (node *Node) SendAction(action *Action) {
	node.actionChannel <- action
}

// TakeAction is how service manager handles the action.
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

// StartServiceManager starts service manager.
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
