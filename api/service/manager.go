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
	// WaitForStatusUpdate is the delay time to update new statum. Currently set 1 second for development. Should be 30 minutes for production.
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

// Manager stores all services for service manager.
type Manager struct {
	services      map[Type]Interface
	actionChannel chan *Action
}

// Register new service to service store.
func (m *Manager) Register(t Type, service Interface) {
	if m.services == nil {
		m.services = make(map[Type]Interface)
	}
	m.services[t] = service
}

// SetupServiceManager inits service map and start service manager.
func (m *Manager) SetupServiceManager() {
	m.InitServiceMap()
	m.actionChannel = m.StartServiceManager()
}

// RegisterService is used for testing.
func (m *Manager) RegisterService(t Type, service Interface) {
	m.Register(t, service)
}

// InitServiceMap initializes service map.
func (m *Manager) InitServiceMap() {
	m.services = make(map[Type]Interface)
}

// SendAction sends action to action channel which is observed by service manager.
func (m *Manager) SendAction(action *Action) {
	m.actionChannel <- action
}

// TakeAction is how service manager handles the action.
func (m *Manager) TakeAction(action *Action) {
	if m.services == nil {
		utils.GetLogInstance().Error("Service store is not initialized.")
		return
	}
	if service, ok := m.services[action.serviceType]; ok {
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
func (m *Manager) StartServiceManager() chan *Action {
	ch := make(chan *Action)
	go func() {
		for {
			select {
			case action := <-ch:
				m.TakeAction(action)
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
