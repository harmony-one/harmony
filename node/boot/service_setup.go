package bootnode

import (
	"github.com/harmony-one/harmony/api/service"
)

// RegisterService register a service to the node service manager
func (node *BootNode) RegisterService(st service.Type, s service.Service) {
	node.serviceManager.Register(st, s)
}

// StartServices runs registered services.
func (node *BootNode) StartServices() error {
	return node.serviceManager.StartServices()
}

// StopServices runs registered services.
func (node *BootNode) StopServices() error {
	return node.serviceManager.StopServices()
}
