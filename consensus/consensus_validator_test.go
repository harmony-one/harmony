package consensus

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	"github.com/stretchr/testify/assert"

	"github.com/harmony-one/harmony/p2p/p2pimpl"

	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/p2p"
)

func TestProcessMessageValidatorAnnounce(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "9982"}
	_, leader.PubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "9984", ValidatorID: 1}
	_, validator1.PubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "9986", ValidatorID: 2}
	_, validator2.PubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "9988", ValidatorID: 3}
	_, validator3.PubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(1)

	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	blockBytes, err := hex.DecodeString("f90242f9023da00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a02b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2aa056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000083020000808502540be400808080a000000000000000000000000000000000000000000000000000000000000000008800000000000000008400000001b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080c0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("a0b3344bd84d41e59b8d84857196080dc8bf91df2787ed5e3e7d65bf8a8cea050b")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	msg := consensusLeader.constructAnnounceMessage()

	message := consensus_proto.Message{}
	err = message.XXX_Unmarshal(msg[1:])

	if err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator1.BlockVerifier = func(block *types.Block) bool {
		return true
	}

	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	assert.Equal(test, PrepareDone, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageValidatorPrepared(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "7782"}
	_, leader.PubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7784", ValidatorID: 1}
	_, validator1.PubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7786", ValidatorID: 2}
	_, validator2.PubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7788", ValidatorID: 3}
	_, validator3.PubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(2)

	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	blockBytes, err := hex.DecodeString("f90242f9023da00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a02b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2aa056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000083020000808502540be400808080a000000000000000000000000000000000000000000000000000000000000000008800000000000000008400000001b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080c0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("a0b3344bd84d41e59b8d84857196080dc8bf91df2787ed5e3e7d65bf8a8cea050b")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	announceMsg := consensusLeader.constructAnnounceMessage()
	consensusLeader.prepareSigs[consensusLeader.nodeID] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	preparedMsg, _ := consensusLeader.constructPreparedMessage()

	if err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator1.BlockVerifier = func(block *types.Block) bool {
		return true
	}

	message := consensus_proto.Message{}
	err = message.XXX_Unmarshal(announceMsg[1:])
	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	message = consensus_proto.Message{}
	err = message.XXX_Unmarshal(preparedMsg[1:])
	consensusValidator1.processPreparedMessage(message)

	assert.Equal(test, CommitDone, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageValidatorCommitted(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "7782"}
	_, leader.PubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7784", ValidatorID: 1}
	_, validator1.PubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7786", ValidatorID: 2}
	_, validator2.PubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7788", ValidatorID: 3}
	_, validator3.PubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(2)

	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	blockBytes, err := hex.DecodeString("f90242f9023da00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a02b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2aa056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000083020000808502540be400808080a000000000000000000000000000000000000000000000000000000000000000008800000000000000008400000001b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080c0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("a0b3344bd84d41e59b8d84857196080dc8bf91df2787ed5e3e7d65bf8a8cea050b")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	announceMsg := consensusLeader.constructAnnounceMessage()
	consensusLeader.prepareSigs[consensusLeader.nodeID] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	preparedMsg, _ := consensusLeader.constructPreparedMessage()
	aggSig := bls_cosi.AggregateSig(consensusLeader.GetPrepareSigsArray())
	multiSigAndBitmap := append(aggSig.Serialize(), consensusLeader.prepareBitmap.Bitmap...)

	consensusLeader.commitSigs[consensusLeader.nodeID] = consensusLeader.priKey.SignHash(multiSigAndBitmap)
	committedMsg, _ := consensusLeader.constructCommittedMessage()

	if err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator1.BlockVerifier = func(block *types.Block) bool {
		return true
	}
	consensusValidator1.OnConsensusDone = func(newBlock *types.Block) {}

	message := consensus_proto.Message{}
	err = message.XXX_Unmarshal(announceMsg[1:])
	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	message = consensus_proto.Message{}
	err = message.XXX_Unmarshal(preparedMsg[1:])
	consensusValidator1.processPreparedMessage(message)

	message = consensus_proto.Message{}
	err = message.XXX_Unmarshal(committedMsg[1:])
	consensusValidator1.processCommittedMessage(message)

	assert.Equal(test, Finished, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}
