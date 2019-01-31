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
func (rc *RoleConversion) StartService() {
	rc.stopChan = make(chan struct{})
	rc.stoppedChan = make(chan struct{})

	rc.Init()
	rc.Run(rc.stopChan, rc.stoppedChan)
}

// Init initializes role conversion service.
func (rc *RoleConversion) Init() {
}

// Run runs role conversion.
func (rc *RoleConversion) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running role conversion")
				// TODO: Write some logic here.
				rc.DoRoleConversion()
			case <-stopChan:
				return
			}
		}
	}()
}

// DoRoleConversion does role conversion.
func (rc *RoleConversion) DoRoleConversion() {
}

// StopService stops role conversion service.
func (rc *RoleConversion) StopService() {
	utils.GetLogInstance().Info("Stopping role conversion service.")
	rc.stopChan <- struct{}{}
	<-rc.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
