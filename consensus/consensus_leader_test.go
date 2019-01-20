package consensus

import (
	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/internal/utils"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/harmony-one/harmony/p2p/p2pimpl"

	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/p2p"
)

func TestProcessMessageLeaderCommit(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "7777"}
	_, leader.PubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7778", ValidatorID: 1}
	_, validator1.PubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7776", ValidatorID: 2}
	_, validator2.PubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7779", ValidatorID: 3}
	_, validator3.PubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(3)

	consensusLeader := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusLeader.blockHash = [32]byte{}

	host1, _ := p2pimpl.NewHost(&validator1)
	consensusValidator1 := New(host1, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator1.blockHash = [32]byte{}
	_, msg := consensusValidator1.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensusLeader.ProcessMessageLeader(msg[1:])

	host2, _ := p2pimpl.NewHost(&validator2)
	consensusValidator2 := New(host2, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator2.blockHash = [32]byte{}
	_, msg = consensusValidator2.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensusLeader.ProcessMessageLeader(msg[1:])

	host3, _ := p2pimpl.NewHost(&validator3)
	consensusValidator3 := New(host3, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator3.blockHash = [32]byte{}
	_, msg = consensusValidator3.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensusLeader.ProcessMessageLeader(msg[1:])

	assert.Equal(test, ChallengeDone, consensusLeader.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageLeaderResponse(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "8889"}
	_, leader.PubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "8887", ValidatorID: 1}
	_, validator1.PubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "8888", ValidatorID: 2}
	_, validator2.PubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "8899", ValidatorID: 3}
	_, validator3.PubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(6)

	consensusLeader := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusLeader.blockHash = [32]byte{}

	host1, _ := p2pimpl.NewHost(&validator1)
	consensusValidator1 := New(host1, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator1.blockHash = [32]byte{}
	_, msg := consensusValidator1.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensusLeader.ProcessMessageLeader(msg[1:])

	host2, _ := p2pimpl.NewHost(&validator2)
	consensusValidator2 := New(host2, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator2.blockHash = [32]byte{}
	_, msg = consensusValidator2.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensusLeader.ProcessMessageLeader(msg[1:])

	host3, _ := p2pimpl.NewHost(&validator3)
	consensusValidator3 := New(host3, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator3.blockHash = [32]byte{}
	_, msg = consensusValidator3.constructCommitMessage(consensus_proto.MessageType_COMMIT)
	consensusLeader.ProcessMessageLeader(msg[1:])

	msg = consensusValidator1.constructResponseMessage(consensus_proto.MessageType_RESPONSE, crypto.Ed25519Curve.Scalar().One())
	consensusLeader.ProcessMessageLeader(msg[1:])

	msg = consensusValidator2.constructResponseMessage(consensus_proto.MessageType_RESPONSE, crypto.Ed25519Curve.Scalar().One())
	consensusLeader.ProcessMessageLeader(msg[1:])

	msg = consensusValidator3.constructResponseMessage(consensus_proto.MessageType_RESPONSE, crypto.Ed25519Curve.Scalar().One())
	consensusLeader.ProcessMessageLeader(msg[1:])

	assert.Equal(test, CollectiveSigDone, consensusLeader.state)

	time.Sleep(1 * time.Second)
}
