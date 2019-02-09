package staking

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	client "github.com/harmony-one/harmony/api/client/service"
	proto "github.com/harmony-one/harmony/api/client/service/proto"
	"github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Service is the staking service.
// Service requires private key here which is not a right design.
// In stead in the right design, the end-user who runs mining needs to provide signed tx to this service.
type Service struct {
	stopChan    chan struct{}
	stoppedChan chan struct{}
	peerChan    <-chan p2p.Peer
	accountKey  *ecdsa.PrivateKey
}

// New returns staking service.
func New(accountKey *ecdsa.PrivateKey, peerChan <-chan p2p.Peer) *Service {
	return &Service{
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		peerChan:    peerChan,
		accountKey:  accountKey,
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
				utils.GetLogInstance().Info("Running role conversion")
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

	// TODO(minhdoan): How to use the p2p or pubsub to send Staking Message to beacon chain.
	// See below of how to create a staking message.
}

func (s *Service) getAccountState(beaconPeer p2p.Peer) *proto.FetchAccountStateResponse {
	client := client.NewClient(beaconPeer.IP, beaconPeer.Port)
	defer client.Close()
	return client.GetBalance(crypto.PubkeyToAddress(s.accountKey.PublicKey))
}

func (s *Service) createStakingMessage(beaconPeer p2p.Peer) *message.Message {
	accountState := s.getAccountState(beaconPeer)
	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	gasLimit := uint64(0)
	gasPrice := big.NewInt(0)
	tx := types.NewTransaction(
		accountState.Nonce,
		toAddress,
		0, // beacon chain.
		new(big.Int).SetBytes(accountState.Balance),
		gasLimit,
		gasPrice,
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
