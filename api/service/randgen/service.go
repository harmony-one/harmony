package randgen

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the random generation service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// New returns random generation service.
func New() *Service {
	return &Service{}
}

// StartService starts random generation service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes random generation.
func (s *Service) Init() {
}

// Run runs random generation.
func (s *Service) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running random generation")
				// Write some logic here.
				s.DoRandomGeneration()
			case <-stopChan:
				return
			}
		}
	}()
}

// DoRandomGeneration does random generation.
func (s *Service) DoRandomGeneration() {

}

// StopService stops random generation service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping random generation service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Random generation stopped.")
}
