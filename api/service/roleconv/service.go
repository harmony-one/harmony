package roleconv

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the role conversion service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// NewService returns role conversion service.
func NewService() *Service {
	return &Service{}
}

// StartService starts role conversion service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes role conversion service.
func (s *Service) Init() {
}

// Run runs role conversion.
func (s *Service) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running role conversion")
				// TODO: Write some logic here.
				s.DoService()
			case <-stopChan:
				return
			}
		}
	}()
}

// DoService does role conversion.
func (s *Service) DoService() {
}

// StopService stops role conversion service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping role conversion service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}
