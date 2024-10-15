package bootnode

import (
	"github.com/harmony-one/harmony/api/service"
)

// RegisterService register a service to the node service manager
func (bootnode *BootNode) RegisterService(st service.Type, s service.Service) {
	bootnode.serviceManager.Register(st, s)
}

// StartServices runs registered services.
func (bootnode *BootNode) StartServices() error {
	return bootnode.serviceManager.StartServices()
}

// StopServices runs registered services.
func (bootnode *BootNode) StopServices() error {
	return bootnode.serviceManager.StopServices()
}
