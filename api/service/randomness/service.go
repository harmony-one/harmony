package randomness

import (
	"github.com/harmony-one/harmony/drand"
	"github.com/harmony-one/harmony/internal/utils"
)

// Service is the randomness generation service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	DRand       *drand.DRand
}

// New returns randomness generation service.
func New(dRand *drand.DRand) *Service {
	return &Service{DRand: dRand}
}

// StartService starts randomness generation service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes randomness generation.
func (s *Service) Init() {
	for {
		newBlock := <-s.DRand.ConfirmedBlockChannel
		_ = newBlock
		// TODO: process newBlock
	}
}

// Run runs randomness generation.
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

// DoRandomGeneration does rarandomnessndom generation.
func (s *Service) DoRandomGeneration() {

}

// StopService stops randomness generation service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping random generation service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Random generation stopped.")
}
