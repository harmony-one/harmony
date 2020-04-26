package service

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// ActionType is the input for Service Manager to operate.
type ActionType byte

// Constants for Action Type.
const (
	Start ActionType = iota
	Stop
	Notify
)

// Type is service type.
type Type byte

// Constants for Type.
const (
	ClientSupport Type = iota
	SupportExplorer
)

// Constants for timing.
const (
	// WaitForStatusUpdate is the delay time to update new status. Currently set 1 second for development. Should be 30 minutes for production.
	WaitForStatusUpdate = time.Minute * 1
)

// Action is type of service action.
type Action struct {
	Action      ActionType
	ServiceType Type
	Params      map[string]interface{}
}

// Interface is the collection of functions any service needs to implement.
type Interface interface {
	StartService()
	SetMessageChan(msgChan chan *msg_pb.Message)
	StopService()
	NotifyService(map[string]interface{})

	// APIs retrieves the list of RPC descriptors the service provides
	APIs() []rpc.API
}

// Manager stores all services for service manager.
type Manager struct {
	sync.Mutex
	services      map[Type]Interface
	actionChannel chan *Action
}

// NewManager ..
func NewManager() *Manager {
	return &Manager{
		services:      map[Type]Interface{},
		actionChannel: make(chan *Action),
	}
}

// GetServices returns all registered services.
func (m *Manager) GetServices() map[Type]Interface {
	m.Lock()
	defer m.Unlock()
	return m.services
}

// Register registers new service to service store.
func (m *Manager) Register(t Type, service Interface) {
	utils.Logger().Info().Int("service", int(t)).Msg("Register Service")
	if _, ok := m.services[t]; ok {
		utils.Logger().Error().Int("servie", int(t)).Msg("This service is already included")
		return
	}
	m.Lock()
	defer m.Unlock()
	m.services[t] = service
}

// RegisterService is used for testing.
func (m *Manager) RegisterService(t Type, service Interface) {
	m.Register(t, service)
}

// SendAction sends action to action channel which is observed by service manager.
func (m *Manager) SendAction(action *Action) {
	m.actionChannel <- action
}

// TakeAction is how service manager handles the action.
func (m *Manager) TakeAction(action *Action) {
	if service, ok := m.services[action.ServiceType]; ok {
		switch action.Action {
		case Start:
			service.StartService()
		case Stop:
			service.StopService()
		case Notify:
			service.NotifyService(action.Params)
		}
	}
}

// RunServices run registered services.
func (m *Manager) RunServices() {
	for serviceType := range m.services {
		action := &Action{
			Action:      Start,
			ServiceType: serviceType,
		}
		m.TakeAction(action)
	}
}

// SetupServiceMessageChan sets up message channel to services.
func (m *Manager) SetupServiceMessageChan(
	mapServiceTypeChan map[Type]chan *msg_pb.Message,
) {
	for serviceType, service := range m.services {
		mapServiceTypeChan[serviceType] = make(chan *msg_pb.Message)
		service.SetMessageChan(mapServiceTypeChan[serviceType])
	}
}
