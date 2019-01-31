package service

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// RoleConversion is the role conversion service.
type RoleConversion struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// NewRoleConversion returns role conversion service.
func NewRoleConversion() *RoleConversion {
	return &RoleConversion{}
}

// StartService starts role conversion service.
func (cs *RoleConversion) StartService() {
	cs.stopChan = make(chan struct{})
	cs.stoppedChan = make(chan struct{})

	cs.Init()
	cs.Run(cs.stopChan, cs.stoppedChan)
}

// Init initializes role conversion service.
func (cs *RoleConversion) Init() {
}

// Run runs role conversion.
func (cs *RoleConversion) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running role conversion")
			case <-stopChan:
				return
			}
		}
	}()
}

// StopService stops role conversion service.
func (cs *RoleConversion) StopService() {
	utils.GetLogInstance().Info("Stopping role conversion service.")
	cs.stopChan <- struct{}{}
	<-cs.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
