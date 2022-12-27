package service

import (
	"fmt"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Type is service type.
type Type byte

// Constants for Type.
const (
	UnknownService Type = iota
	ClientSupport
	SupportExplorer
	Consensus
	BlockProposal
	NetworkInfo
	Pprof
	Prometheus
	Synchronize
	CrosslinkSending
	StagedStreamSync
)

func (t Type) String() string {
	switch t {
	case SupportExplorer:
		return "SupportExplorer"
	case ClientSupport:
		return "ClientSupport"
	case Consensus:
		return "Consensus"
	case BlockProposal:
		return "BlockProposal"
	case NetworkInfo:
		return "NetworkInfo"
	case Pprof:
		return "Pprof"
	case Prometheus:
		return "Prometheus"
	case Synchronize:
		return "Synchronize"
	case CrosslinkSending:
		return "CrosslinkSending"
	case StagedStreamSync:
		return "StagedStreamSync"
	default:
		return "Unknown"
	}
}

// Service is the collection of functions any service needs to implement.
type Service interface {
	Start() error
	Stop() error
}

// Manager stores all services for service manager.
type Manager struct {
	services   []Service
	serviceMap map[Type]Service

	logger zerolog.Logger
}

// NewManager creates a new manager
func NewManager() *Manager {
	return &Manager{
		services:   nil,
		serviceMap: make(map[Type]Service),
		logger:     *utils.Logger(),
	}
}

// Register registers new service to service store.
func (m *Manager) Register(t Type, service Service) {
	utils.Logger().Info().Int("service", int(t)).Msg("Register Service")
	if _, ok := m.serviceMap[t]; ok {
		utils.Logger().Error().Int("service", int(t)).Msg("This service is already included")
		return
	}
	m.services = append(m.services, service)
	m.serviceMap[t] = service
}

// GetServices returns all registered services.
func (m *Manager) GetServices() []Service {
	return m.services
}

// GetService get the specified service
func (m *Manager) GetService(t Type) Service {
	return m.serviceMap[t]
}

// StartServices run all registered services. If one of the starting service returns
// an error, closing all started services.
func (m *Manager) StartServices() (err error) {
	started := make([]Service, 0, len(m.services))

	defer func() {
		if err != nil {
			// If error is not nil, closing all services in reverse order
			if stopErr := m.stopServices(started); stopErr != nil {
				err = fmt.Errorf("%v; %v", err, stopErr)
			}
		}
	}()

	for _, service := range m.services {
		t := m.typeByService(service)
		m.logger.Info().Str("type", t.String()).Msg("Starting service")
		if err = service.Start(); err != nil {
			err = errors.Wrapf(err, "cannot start service [%v]", t.String())
			return err
		}
		started = append(started, service)
	}
	return err
}

// StopServices stops all services in the reverse order.
func (m *Manager) StopServices() error {
	return m.stopServices(m.services)
}

// stopServices stops given services in the reverse order.
func (m *Manager) stopServices(services []Service) error {
	size := len(services)
	var rErr error

	for i := size - 1; i >= 0; i-- {
		service := services[i]
		t := m.typeByService(service)

		m.logger.Info().Str("type", t.String()).Msg("Stopping service")
		if err := service.Stop(); err != nil {
			err = errors.Wrapf(err, "failed to stop service [%v]", t.String())
			if rErr != nil {
				rErr = fmt.Errorf("%v; %v", rErr, err)
			} else {
				rErr = err
			}
		}
	}
	return rErr
}

func (m *Manager) typeByService(target Service) Type {
	for t, s := range m.serviceMap {
		if s == target {
			return t
		}
	}
	return UnknownService
}
