package consensus

import (
	"fmt"

	"crypto/sha256"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/harmony-one/harmony/p2p/p2pimpl"

	"github.com/harmony-one/harmony/p2p"
)

var (
	ip        = "127.0.0.1"
	blockHash = sha256.Sum256([]byte("test"))
)

func TestProcessMessageLeaderPrepare(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: ip, Port: "7777"}
	_, leader.PubKey = utils.GenKeyBLS(leader.IP, leader.Port)

	validators := make([]p2p.Peer, 3)
	hosts := make([]p2p.Host, 3)

	for i := 0; i < 3; i++ {
		port := fmt.Sprintf("%d", 7788+i)
		validators[i] = p2p.Peer{IP: ip, Port: port, ValidatorID: i + 1}
		_, validators[i].PubKey = utils.GenKeyBLS(validators[i].IP, validators[i].Port)
	}

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(3)

	consensusLeader := New(m, "0", validators, leader)
	consensusLeader.blockHash = blockHash

	consensusValidators := make([]*Consensus, 3)
	for i := 0; i < 3; i++ {
		priKey, _, _ := utils.GenKeyP2P(validators[i].IP, validators[i].Port)
		host, err := p2pimpl.NewHost(&validators[i], priKey)
		if err != nil {
			test.Fatalf("newhost error: %v", err)
		}
		hosts[i] = host

		consensusValidators[i] = New(hosts[i], "0", validators, leader)
		consensusValidators[i].blockHash = blockHash
		msg := consensusValidators[i].constructPrepareMessage()
		consensusLeader.ProcessMessageLeader(msg[1:])
	}

	assert.Equal(test, PreparedDone, consensusLeader.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageLeaderCommit(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: ip, Port: "8889"}
	_, leader.PubKey = utils.GenKeyBLS(leader.IP, leader.Port)

	validators := make([]p2p.Peer, 3)
	hosts := make([]p2p.Host, 3)

	for i := 0; i < 3; i++ {
		port := fmt.Sprintf("%d", 8788+i)
		validators[i] = p2p.Peer{IP: ip, Port: port, ValidatorID: i + 1}
		_, validators[i].PubKey = utils.GenKeyBLS(validators[i].IP, validators[i].Port)
	}

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(3)

	for i := 0; i < 3; i++ {
		priKey, _, _ := utils.GenKeyP2P(validators[i].IP, validators[i].Port)
		host, err := p2pimpl.NewHost(&validators[i], priKey)
		if err != nil {
			test.Fatalf("newhost error: %v", err)
		}
		hosts[i] = host
	}

	consensusLeader := New(m, "0", validators, leader)
	consensusLeader.state = PreparedDone
	consensusLeader.blockHash = blockHash
	consensusLeader.OnConsensusDone = func(newBlock *types.Block) {}
	consensusLeader.block, _ = rlp.EncodeToBytes(types.NewBlock(&types.Header{}, nil, nil))

	consensusValidators := make([]*Consensus, 3)

	go func() {
		<-consensusLeader.ReadySignal
		<-consensusLeader.ReadySignal
	}()
	for i := 0; i < 3; i++ {
		consensusValidators[i] = New(hosts[i], "0", validators, leader)
		consensusValidators[i].blockHash = blockHash
		msg := consensusValidators[i].constructCommitMessage()
		consensusLeader.ProcessMessageLeader(msg[1:])
	}

	assert.Equal(test, Finished, consensusLeader.state)

	time.Sleep(1 * time.Second)
}
