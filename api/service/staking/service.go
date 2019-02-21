package staking

import (
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	protobuf "github.com/golang/protobuf/proto"
	proto "github.com/harmony-one/harmony/api/client/service/proto"
	"github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

const (
	// WaitTime is the delay time for resending staking transaction if the previous transaction did not get approved.
	WaitTime = 5 * time.Second
	// StakingContractAddress is the staking deployed contract address
	StakingContractAddress = "TODO(minhdoan): Create a PR to generate staking contract address"
)

// State is the state of staking service.
type State byte

// Service is the staking service.
// Service requires private key here which is not a right design.
// In stead in the right design, the end-user who runs mining needs to provide signed tx to this service.
type Service struct {
	host          p2p.Host
	stopChan      chan struct{}
	stoppedChan   chan struct{}
	accountKey    *ecdsa.PrivateKey
	stakingAmount int64
	state         State
	beaconChain   *core.BlockChain
}

// New returns staking service.
func New(host p2p.Host, accountKey *ecdsa.PrivateKey, stakingAmount int64, beaconChain *core.BlockChain) *Service {
	return &Service{
		host:          host,
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
		accountKey:    accountKey,
		stakingAmount: stakingAmount,
		beaconChain:   beaconChain,
	}
}

// StartService starts staking service.
func (s *Service) StartService() {
	log.Info("Start Staking Service")
	s.Run()
}

// Run runs staking.
func (s *Service) Run() {
	tick := time.NewTicker(WaitTime)
	go func() {
		defer close(s.stoppedChan)
		// Do service first time and after that doing it every 5 minutes.
		// The reason we have to do it in every x minutes because of beacon chain syncing.
		s.DoService()
		for {
			select {
			case <-tick.C:
				if s.IsStaked() {
					return
				}
				s.DoService()
			case <-s.stopChan:
				return
			}
		}
	}()
}

// IsStaked checks if the txn gets accepted and approved in the beacon chain.
func (s *Service) IsStaked() bool {
	return false
}

// DoService does staking.
func (s *Service) DoService() {
	utils.GetLogInstance().Info("Trying to send a staking transaction.")
	if s.beaconChain == nil {
		utils.GetLogInstance().Info("Can not send a staking transaction because of nil beacon chain.")
		return
	}

	if msg := s.createStakingMessage(); msg != nil {
		s.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, host.ConstructP2pMessage(byte(17), msg))
	} else {
		utils.GetLogInstance().Error("Can not create staking transaction")
	}
}

func (s *Service) getStakingInfo() *proto.StakingContractInfoResponse {
	address := crypto.PubkeyToAddress(s.accountKey.PublicKey)
	state, err := s.beaconChain.State()
	if err != nil {
		utils.GetLogInstance().Error("error to get beacon chain state when getting staking info")
		return nil
	}
	balance := state.GetBalance(address)
	if balance == common.Big0 {
		utils.GetLogInstance().Error("account balance empty when getting staking info")
		return nil
	}
	nonce := state.GetNonce(address)
	if nonce == 0 {
		utils.GetLogInstance().Error("nonce zero when getting staking info")
		return nil
	}
	return &proto.StakingContractInfoResponse{
		ContractAddress: StakingContractAddress,
		Balance:         balance.Bytes(),
		Nonce:           nonce,
	}
}

// Constructs the staking message
func constructStakingMessage(ts types.Transactions) []byte {
	msg := &message.Message{
		Type: message.MessageType_NEWNODE_BEACON_STAKING,
		Request: &message.Message_Staking{
			Staking: &message.StakingRequest{
				Transaction: ts.GetRlp(0),
				NodeId:      "",
			},
		},
	}
	if data, err := protobuf.Marshal(msg); err == nil {
		return data
	}
	utils.GetLogInstance().Error("Error when creating staking message")
	return nil
}

func (s *Service) createStakingMessage() []byte {
	stakingInfo := s.getStakingInfo()
	toAddress := common.HexToAddress(stakingInfo.ContractAddress)
	tx := types.NewTransaction(
		stakingInfo.Nonce,
		toAddress,
		0, // beacon chain.
		big.NewInt(s.stakingAmount),
		params.CallValueTransferGas*2,           // hard-code
		big.NewInt(int64(params.Sha256BaseGas)), // pick some predefined gas price.
		nil)

	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, s.accountKey); err == nil {
		ts := types.Transactions{signedTx}
		return constructStakingMessage(ts)
	}
	return nil
}

// StopService stops staking service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Stopping staking service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.GetLogInstance().Info("Role conversion stopped.")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}
