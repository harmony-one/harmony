package service

import (
	"github.com/harmony-one/harmony/internal/utils"
)

// RandomGeneration is the consensus service.
type RandomGeneration struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// NewRandomGeneration returns random generation service.
func NewRandomGeneration() *RandomGeneration {
	return &RandomGeneration{}
}

// StartService starts random generation service.
func (cs *RandomGeneration) StartService() {
	cs.stopChan = make(chan struct{})
	cs.stoppedChan = make(chan struct{})

	cs.Init()
	cs.Run(cs.stopChan, cs.stoppedChan)
}

// Init initializes random generation.
func (cs *RandomGeneration) Init() {
}

// Run runs random generation.
func (cs *RandomGeneration) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				utils.GetLogInstance().Info("Running random generation")
				// Write some logic here.
				cs.DoRandomGeneration()
			case <-stopChan:
				return
			}
		}
	}()
}

// DoRandomGeneration does random generation.
func (cs *RandomGeneration) DoRandomGeneration() {

}

// StopService stops random generation service.
func (cs *RandomGeneration) StopService() {
	utils.GetLogInstance().Info("Stopping random generation service.")
	cs.stopChan <- struct{}{}
	<-cs.stoppedChan
	utils.GetLogInstance().Info("Random generation stopped.")
}
