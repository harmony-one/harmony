package consensus

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"

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

type MockChainReader struct{}

func (MockChainReader) Config() *params.ChainConfig {
	return nil
}

func (MockChainReader) CurrentHeader() *types.Header {
	return &types.Header{}
}

func (MockChainReader) GetHeader(hash common.Hash, number uint64) *types.Header {
	return &types.Header{}
}

func (MockChainReader) GetHeaderByNumber(number uint64) *types.Header {
	return &types.Header{}
}

func (MockChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	return &types.Header{}
}

// GetBlock retrieves a block from the database by hash and number.
func (MockChainReader) GetBlock(hash common.Hash, number uint64) *types.Block {
	return &types.Block{}
}

func TestProcessMessageValidatorAnnounce(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "9982"}
	var leaderPriKey *bls.SecretKey
	leaderPriKey, leader.ConsensusPubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "9984", ValidatorID: 1}
	_, validator1.ConsensusPubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "9986", ValidatorID: 2}
	_, validator2.ConsensusPubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "9988", ValidatorID: 3}
	_, validator3.ConsensusPubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, gomock.Any())

	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader, leaderPriKey)
	blockBytes, err := hex.DecodeString("f902a5f902a0a00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a02b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2aa056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000083020000808502540be400808080a000000000000000000000000000000000000000000000000000000000000000008800000000000000008400000001b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000c0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("bdd66a8211ffcbf0ad431b506c854b49264951fd9f690928e9cf44910c381053")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	msgBytes := consensusLeader.constructAnnounceMessage()
	msgBytes, err = proto.GetConsensusMessagePayload(msgBytes)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}

	message := &msg_pb.Message{}
	if err = protobuf.Unmarshal(msgBytes, message); err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader, bls_cosi.RandPrivateKey())
	consensusValidator1.ChainReader = MockChainReader{}

	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	assert.Equal(test, PrepareDone, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageValidatorPrepared(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "7782"}
	var leaderPriKey *bls.SecretKey
	leaderPriKey, leader.ConsensusPubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7784", ValidatorID: 1}
	_, validator1.ConsensusPubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7786", ValidatorID: 2}
	_, validator2.ConsensusPubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7788", ValidatorID: 3}
	_, validator3.ConsensusPubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, gomock.Any()).Times(2)

	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader, leaderPriKey)
	blockBytes, err := hex.DecodeString("f902a5f902a0a00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a02b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2aa056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000083020000808502540be400808080a000000000000000000000000000000000000000000000000000000000000000008800000000000000008400000001b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000c0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("bdd66a8211ffcbf0ad431b506c854b49264951fd9f690928e9cf44910c381053")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	announceMsg := consensusLeader.constructAnnounceMessage()
	consensusLeader.prepareSigs[consensusLeader.nodeID] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	preparedMsg, _ := consensusLeader.constructPreparedMessage()

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader, bls_cosi.RandPrivateKey())
	consensusValidator1.ChainReader = MockChainReader{}

	// Get actual consensus messages.
	announceMsg, err = proto.GetConsensusMessagePayload(announceMsg)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}
	preparedMsg, err = proto.GetConsensusMessagePayload(preparedMsg)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}

	message := &msg_pb.Message{}
	if err = protobuf.Unmarshal(announceMsg, message); err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	if err = protobuf.Unmarshal(preparedMsg, message); err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	consensusValidator1.processPreparedMessage(message)

	assert.Equal(test, CommitDone, consensusValidator1.state)
	time.Sleep(time.Second)
}

func TestProcessMessageValidatorCommitted(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "7782"}
	var leaderPriKey *bls.SecretKey
	leaderPriKey, leader.ConsensusPubKey = utils.GenKey(leader.IP, leader.Port)

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7784", ValidatorID: 1}
	_, validator1.ConsensusPubKey = utils.GenKey(validator1.IP, validator1.Port)
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7786", ValidatorID: 2}
	_, validator2.ConsensusPubKey = utils.GenKey(validator2.IP, validator2.Port)
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7788", ValidatorID: 3}
	_, validator3.ConsensusPubKey = utils.GenKey(validator3.IP, validator3.Port)

	m := mock_host.NewMockHost(ctrl)
	// Asserts that the first and only call to Bar() is passed 99.
	// Anything else will fail.
	m.EXPECT().GetSelfPeer().Return(leader)
	m.EXPECT().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, gomock.Any()).Times(2)

	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	message := &msg_pb.Message{}
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader, leaderPriKey)
	blockBytes, err := hex.DecodeString("f902a5f902a0a00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a02b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2aa056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000083020000808502540be400808080a000000000000000000000000000000000000000000000000000000000000000008800000000000000008400000001b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000c0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("bdd66a8211ffcbf0ad431b506c854b49264951fd9f690928e9cf44910c381053")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	announceMsg := consensusLeader.constructAnnounceMessage()
	consensusLeader.prepareSigs[consensusLeader.nodeID] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	preparedMsg, _ := consensusLeader.constructPreparedMessage()
	aggSig := bls_cosi.AggregateSig(consensusLeader.GetPrepareSigsArray())
	multiSigAndBitmap := append(aggSig.Serialize(), consensusLeader.prepareBitmap.Bitmap...)

	consensusLeader.commitSigs[consensusLeader.nodeID] = consensusLeader.priKey.SignHash(multiSigAndBitmap)
	committedMsg, _ := consensusLeader.constructCommittedMessage()

	// Get actual consensus messages.
	announceMsg, err = proto.GetConsensusMessagePayload(announceMsg)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}
	preparedMsg, err = proto.GetConsensusMessagePayload(preparedMsg)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}
	committedMsg, err = proto.GetConsensusMessagePayload(committedMsg)
	if err != nil {
		test.Errorf("Failed to get consensus message")
	}

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader, bls_cosi.RandPrivateKey())
	consensusValidator1.ChainReader = MockChainReader{}
	consensusValidator1.OnConsensusDone = func(newBlock *types.Block) {}

	if err = protobuf.Unmarshal(announceMsg, message); err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}
	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	if err = protobuf.Unmarshal(preparedMsg, message); err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}
	consensusValidator1.processPreparedMessage(message)

	if err = protobuf.Unmarshal(committedMsg, message); err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}
	consensusValidator1.processCommittedMessage(message)

	assert.Equal(test, Finished, consensusValidator1.state)
	time.Sleep(1 * time.Second)
}
