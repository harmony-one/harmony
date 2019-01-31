package service

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

// Type is service type.
type Type byte

// Constants for Type.
const (
	SupportSyncing Type = iota
	SupportClient
	SupportExplorer
	Consensus
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
	case Consensus:
		return "Consensus"
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
	WaitForStatusUpdate = time.Minute * 1
)

// Action is type of service action.
type Action struct {
	action      ActionType
	serviceType Type
	params      map[string]interface{}
}

// Interface is the collection of functions any service needs to implement.
type Interface interface {
	StartService()
	StopService()
}

// Store stores all services for service manager.
type Store struct {
	services      map[Type]Interface
	actionChannel chan *Action
}

// Register new service to service store.
func (ss *Store) Register(t Type, service Interface) {
	if ss.services == nil {
		ss.services = make(map[Type]Interface)
	}
	ss.services[t] = service
}

// SetupServiceManager inits service map and start service manager.
func (ss *Store) SetupServiceManager() {
	ss.InitServiceMap()
	ss.actionChannel = ss.StartServiceManager()
}

// RegisterService is used for testing.
func (ss *Store) RegisterService(t Type, service Interface) {
	ss.Register(t, service)
}

// InitServiceMap initializes service map.
func (ss *Store) InitServiceMap() {
	ss.services = make(map[Type]Interface)
}

// SendAction sends action to action channel which is observed by service manager.
func (ss *Store) SendAction(action *Action) {
	ss.actionChannel <- action
}

// TakeAction is how service manager handles the action.
func (ss *Store) TakeAction(action *Action) {
	if ss.services == nil {
		utils.GetLogInstance().Error("Service store is not initialized.")
		return
	}
	if service, ok := ss.services[action.serviceType]; ok {
		switch action.action {
		case Start:
			fmt.Printf("Start %s\n", action.serviceType)
			service.StartService()
		case Stop:
			fmt.Printf("Stop %s\n", action.serviceType)
			service.StopService()
		}
	}
}

// StartServiceManager starts service manager.
func (ss *Store) StartServiceManager() chan *Action {
	ch := make(chan *Action)
	go func() {
		for {
			select {
			case action := <-ch:
				ss.TakeAction(action)
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
