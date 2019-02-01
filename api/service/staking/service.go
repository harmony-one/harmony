package staking

import (
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Service is the struct for staking service.
type Service struct {
	Host p2p.Host
}

//StartService starts the staking service.
func (s *Service) StartService() {
	utils.GetLogInstance().Info("Starting staking service.")
}

func (s *Service) createStakingTransaction() {
	//creates staking transaction.
}

// StopService shutdowns staking service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Shutting down staking service.")
}
