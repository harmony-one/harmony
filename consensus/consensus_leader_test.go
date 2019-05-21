package consensus

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/mock/gomock"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"github.com/stretchr/testify/assert"
)

var (
	ip        = "127.0.0.1"
	blockHash = sha256.Sum256([]byte("test"))
)

func TestProcessMessageLeaderPrepare(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: ip, Port: "7777"}
	leaderPriKey := bls_cosi.RandPrivateKey()
	leaderPubKey := leaderPriKey.GetPublicKey()

	leader.ConsensusPubKey = leaderPubKey

	validators := make([]*p2p.Peer, 3)
	hosts := make([]p2p.Host, 3)

	validatorsPriKeys := [3]*bls.SecretKey{}
	for i := 0; i < 3; i++ {
		port := fmt.Sprintf("%d", 7788+i)
		validators[i] = &p2p.Peer{IP: ip, Port: port}
		validatorsPriKeys[i] = bls_cosi.RandPrivateKey()
		validators[i].ConsensusPubKey = validatorsPriKeys[i].GetPublicKey()
	}

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, gomock.Any())

	consensusLeader, err := New(m, 0, leader, leaderPriKey)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensusLeader.ResetState()
	consensusLeader.blockHash = blockHash
	consensusLeader.AddPeers(validators)

	consensusValidators := make([]*Consensus, 3)
	for i := 0; i < 3; i++ {
		priKey, _, _ := utils.GenKeyP2P(validators[i].IP, validators[i].Port)
		host, err := p2pimpl.NewHost(validators[i], priKey)
		if err != nil {
			test.Fatalf("newhost error: %v", err)
		}
		hosts[i] = host

		consensusValidators[i], err = New(hosts[i], 0, leader, validatorsPriKeys[i])
		if err != nil {
			test.Fatalf("Cannot craeate consensus: %v", err)
		}
		consensusValidators[i].blockHash = blockHash
		msg := consensusValidators[i].constructPrepareMessage()
		msgPayload, _ := proto.GetConsensusMessagePayload(msg)
		consensusLeader.ProcessMessageLeader(msgPayload)
	}

	assert.Equal(test, PreparedDone, consensusLeader.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageLeaderPrepareInvalidSignature(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: ip, Port: "7777"}
	leaderPriKey := bls_cosi.RandPrivateKey()
	leader.ConsensusPubKey = leaderPriKey.GetPublicKey()

	validators := make([]p2p.Peer, 3)
	hosts := make([]p2p.Host, 3)

	validatorKeys := [3]*bls.SecretKey{}
	for i := 0; i < 3; i++ {
		port := fmt.Sprintf("%d", 7788+i)
		validators[i] = p2p.Peer{IP: ip, Port: port}
		validatorKeys[i] = bls_cosi.RandPrivateKey()
		validators[i].ConsensusPubKey = validatorKeys[i].GetPublicKey()
	}

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)

	consensusLeader, err := New(m, 0, leader, leaderPriKey)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensusLeader.ResetState()
	consensusLeader.blockHash = blockHash

	consensusValidators := make([]*Consensus, 3)
	for i := 0; i < 3; i++ {
		priKey, _, _ := utils.GenKeyP2P(validators[i].IP, validators[i].Port)
		host, err := p2pimpl.NewHost(&validators[i], priKey)
		if err != nil {
			test.Fatalf("newhost error: %v", err)
		}
		hosts[i] = host

		consensusValidators[i], err = New(hosts[i], 0, leader, validatorKeys[i])
		if err != nil {
			test.Fatalf("Cannot craeate consensus: %v", err)
		}
		consensusValidators[i].blockHash = blockHash
		msgBytes := consensusValidators[i].constructPrepareMessage()
		msgPayload, _ := proto.GetConsensusMessagePayload(msgBytes)

		message := &msg_pb.Message{}
		if err = protobuf.Unmarshal(msgPayload, message); err != nil {
			test.Error("Error when unmarshalling")
		}
		// Put invalid signature
		message.Signature = consensusValidators[i].signMessage([]byte("random string"))
		if msgBytes, err = protobuf.Marshal(message); err != nil {
			test.Error("Error when marshalling")
		}
		consensusLeader.ProcessMessageLeader(msgBytes)
	}

	assert.Equal(test, Finished, consensusLeader.state)
	time.Sleep(1 * time.Second)
}

func TestProcessMessageLeaderCommit(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: ip, Port: "8889"}
	leaderPriKey := bls_cosi.RandPrivateKey()
	leader.ConsensusPubKey = leaderPriKey.GetPublicKey()

	validators := make([]*p2p.Peer, 3)
	hosts := make([]p2p.Host, 3)

	validatorKeys := [3]*bls.SecretKey{}
	for i := 0; i < 3; i++ {
		port := fmt.Sprintf("%d", 8788+i)
		validators[i] = &p2p.Peer{IP: ip, Port: port}
		validatorKeys[i] = bls_cosi.RandPrivateKey()
		validators[i].ConsensusPubKey = validatorKeys[i].GetPublicKey()
	}

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, gomock.Any())

	for i := 0; i < 3; i++ {
		priKey, _, _ := utils.GenKeyP2P(validators[i].IP, validators[i].Port)
		host, err := p2pimpl.NewHost(validators[i], priKey)
		if err != nil {
			test.Fatalf("newhost error: %v", err)
		}
		hosts[i] = host
	}

	consensusLeader, err := New(m, 0, leader, leaderPriKey)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensusLeader.ResetState()
	consensusLeader.AddPeers(validators)
	consensusLeader.state = PreparedDone
	consensusLeader.blockHash = blockHash
	consensusLeader.OnConsensusDone = func(newBlock *types.Block) {}
	consensusLeader.block, _ = rlp.EncodeToBytes(types.NewBlock(&types.Header{}, nil, nil))
	consensusLeader.prepareSigs[consensusLeader.SelfAddress] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	aggSig := bls_cosi.AggregateSig(consensusLeader.GetPrepareSigsArray())
	multiSigAndBitmap := append(aggSig.Serialize(), consensusLeader.prepareBitmap.Bitmap...)
	consensusLeader.aggregatedPrepareSig = aggSig

	consensusValidators := make([]*Consensus, 3)

	go func() {
		<-consensusLeader.ReadySignal
		<-consensusLeader.ReadySignal
	}()
	for i := 0; i < 3; i++ {
		consensusValidators[i], err = New(hosts[i], 0, leader, validatorKeys[i])
		if err != nil {
			test.Fatalf("Cannot craeate consensus: %v", err)
		}
		consensusValidators[i].blockHash = blockHash
		payload := consensusValidators[i].constructCommitMessage(multiSigAndBitmap)
		msg, err := proto.GetConsensusMessagePayload(payload)
		if err != nil {
			test.Error("Error when getting consensus message", "error", err)
		}
		consensusLeader.ProcessMessageLeader(msg)
	}

	assert.Equal(test, Finished, consensusLeader.state)
	time.Sleep(1 * time.Second)
}
