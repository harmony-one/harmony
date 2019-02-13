package staking

import (
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	pb "github.com/golang/protobuf/proto"
	client "github.com/harmony-one/harmony/api/client/service"
	proto "github.com/harmony-one/harmony/api/client/service/proto"
	"github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

type State byte

const (
	NOT_STAKED_YET State = iota
	STAKED
	REJECTED
	APPROVED
	TRANSFORMED
)

// Service is the staking service.
// Service requires private key here which is not a right design.
// In stead in the right design, the end-user who runs mining needs to provide signed tx to this service.
type Service struct {
	host          p2p.Host
	stopChan      chan struct{}
	stoppedChan   chan struct{}
	peerChan      <-chan p2p.Peer
	messageChan   <-chan *message.Message
	accountKey    *ecdsa.PrivateKey
	stakingAmount int64
	state         State
}

// New returns staking service.
func New(accountKey *ecdsa.PrivateKey, stakingAmount int64, peerChan <-chan p2p.Peer, messageChan <-chan *message.Message) *Service {
	return &Service{
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
		peerChan:      peerChan,
		accountKey:    accountKey,
		stakingAmount: stakingAmount,
		messageChan:   messageChan,
		state:         NOT_STAKED_YET,
	}
}

// StartService starts staking service.
func (s *Service) StartService() {
	log.Info("Start Staking Service")
	s.Init()
	s.Run()
}

// Init initializes staking service.
func (s *Service) Init() {
}

// Run runs staking.
func (s *Service) Run() {
	// Wait until peer info of beacon chain is ready.
	go func() {
		defer close(s.stoppedChan)
		for {
			select {
			case peer := <-s.peerChan:
				utils.GetLogInstance().Info("Running staking service")
				// TODO: Write some logic here.
				s.DoService(peer)
			case <-s.stopChan:
				return
			}
		}
	}()
}

// DoService does staking.
func (s *Service) DoService(peer p2p.Peer) {
	utils.GetLogInstance().Info("Staking with Peer")

	stakingMessage := s.createStakingMessage(peer)
	s.state = STAKED
	if data, err := pb.Marshal(stakingMessage); err == nil {
		// Send a staking transaction to beacon chain.
		if err = s.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, data); err != nil {
			utils.GetLogInstance().Error("Error when sending staking message")
			return
		}
		tick := time.NewTicker(5 * time.Second)
		for {
			select {
			// Retry sending the staking transaction if it does not get back any response.
			case <-tick.C:
				if err = s.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, data); err != nil {
					utils.GetLogInstance().Error("Error when sending staking message")
					return
				}
			case msg := <-s.messageChan:
				if isStateResultMessage(msg) {
					if s.stakeApproved(msg) {
						s.state = APPROVED
						// TODO(minhdoan): Should send a signal to another service.
					} else {
						s.state = REJECTED
						// TODO(minhdoan): what's next?
						return
					}
				}
			}
		}
	} else {
		utils.GetLogInstance().Error("Error when creating staking message")
	}
}

func isStateResultMessage(msg *message.Message) bool {
	return true
}

func (s *Service) stakeApproved(msg *message.Message) bool {
	return true
}

func (s *Service) getStakingInfo(beaconPeer p2p.Peer) *proto.StakingContractInfoResponse {
	client := client.NewClient(beaconPeer.IP, beaconPeer.Port)
	defer client.Close()
	return client.GetStakingContractInfo(crypto.PubkeyToAddress(s.accountKey.PublicKey))
}

func (s *Service) createStakingMessage(beaconPeer p2p.Peer) *message.Message {
	stakingInfo := s.getStakingInfo(beaconPeer)
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
		return &message.Message{
			Type: message.MessageType_NEWNODE_BEACON_STAKING,
			Request: &message.Message_Staking{
				Staking: &message.StakingRequest{
					Transaction: ts.GetRlp(0),
					NodeId:      "",
				},
			},
		}
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
