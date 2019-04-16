package consensus

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/mock/gomock"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
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
	leaderPriKey := bls_cosi.RandPrivateKey()
	leader.ConsensusPubKey = leaderPriKey.GetPublicKey()

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "9984"}
	validator1.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "9986"}
	validator2.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "9988"}
	validator3.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()

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
	consensusLeader, err := New(host, 0, leader, leaderPriKey)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	blockBytes, err := testBlockBytes()
	if err != nil {
		test.Fatalf("Cannot decode blockByte: %v", err)
	}
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("bdd66a8211ffcbf0ad431b506c854b49264951fd9f690928e9cf44910c381053")
	if err != nil {
		test.Fatalf("Cannot decode hashByte: %v", err)
	}

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

	consensusValidator1, err := New(m, 0, leader, bls_cosi.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensusValidator1.ChainReader = MockChainReader{}

	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	assert.Equal(test, PrepareDone, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}

func testBlockBytes() ([]byte, error) {
	return hex.DecodeString("" +
		// BEGIN 675-byte Block
		"f902a3" +
		// BEGIN 670-byte header
		"f9029e" +
		// 32-byte ParentHash
		"a0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// 20-byte Coinbase
		"94" +
		"0000000000000000000000000000000000000000" +
		// 32-byte Root
		"a0" +
		"2b418211410ee3e75b32abd925bbeba215172afa509d65c1953d4b4e505a4a2a" +
		// 32-byte TxHash
		"a0" +
		"56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" +
		// 32-byte ReceiptHash
		"a0" +
		"56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" +
		// 256-byte Bloom
		"b90100" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// 3-byte Difficulty
		"83" +
		"020000" +
		// 0-byte Number
		"80" +
		"" +
		// 5-byte GasLimit
		"85" +
		"02540be400" +
		// 0-byte GasUsed
		"80" +
		"" +
		// 0-byte Time
		"80" +
		"" +
		// 0-byte Extra
		"80" +
		"" +
		// 32-byte MixDigest
		"a0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// 8-byte Nonce
		"88" +
		"0000000000000000" +
		// 0-byte Epoch
		"80" +
		"" +
		// single-byte ShardID
		"01" +
		// 48-byte PrepareSignature
		"b0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000" +
		// 0-byte PrepareBitmap
		"80" +
		"" +
		// 48-byte CommitSignature
		"b0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000" +
		// 0-byte CommitBitmap
		"80" +
		"" +
		// 32-byte RandPreimage
		"a0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// 32-byte RandSeed
		"a0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// 32-byte ShardStateHash
		"a0" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		// BEGIN 0-byte ShardState
		"c0" +
		// END ShardState
		// END header
		// BEGIN 0-byte uncles
		"c0" +
		// END uncles
		// BEGIN 0-byte transactions
		"c0" +
		// END transactions
		// END Block
		"")
}

func TestProcessMessageValidatorPrepared(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	leader := p2p.Peer{IP: "127.0.0.1", Port: "7782"}
	leaderPriKey := bls_cosi.RandPrivateKey()
	leader.ConsensusPubKey = leaderPriKey.GetPublicKey()

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7784"}
	validator1.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7786"}
	validator2.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7788"}
	validator3.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()

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
	consensusLeader, err := New(host, 0, leader, leaderPriKey)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	blockBytes, err := testBlockBytes()
	if err != nil {
		test.Fatalf("Cannot decode blockByte: %v", err)
	}
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("bdd66a8211ffcbf0ad431b506c854b49264951fd9f690928e9cf44910c381053")
	if err != nil {
		test.Fatalf("Cannot decode hashByte: %v", err)
	}

	copy(consensusLeader.blockHash[:], hashBytes[:])

	announceMsg := consensusLeader.constructAnnounceMessage()
	consensusLeader.prepareSigs[consensusLeader.SelfAddress] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	preparedMsg, _ := consensusLeader.constructPreparedMessage()

	consensusValidator1, err := New(m, 0, leader, bls_cosi.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
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
	leaderPriKey := bls_cosi.RandPrivateKey()
	leader.ConsensusPubKey = leaderPriKey.GetPublicKey()

	validator1 := p2p.Peer{IP: "127.0.0.1", Port: "7784"}
	validator1.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()
	validator2 := p2p.Peer{IP: "127.0.0.1", Port: "7786"}
	validator2.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()
	validator3 := p2p.Peer{IP: "127.0.0.1", Port: "7788"}
	validator3.ConsensusPubKey = bls_cosi.RandPrivateKey().GetPublicKey()

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
	consensusLeader, err := New(host, 0, leader, leaderPriKey)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	blockBytes, err := testBlockBytes()
	if err != nil {
		test.Fatalf("Cannot decode blockByte: %v", err)
	}
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("bdd66a8211ffcbf0ad431b506c854b49264951fd9f690928e9cf44910c381053")
	if err != nil {
		test.Fatalf("Cannot decode hashByte: %v", err)
	}

	copy(consensusLeader.blockHash[:], hashBytes[:])

	announceMsg := consensusLeader.constructAnnounceMessage()
	consensusLeader.prepareSigs[consensusLeader.SelfAddress] = consensusLeader.priKey.SignHash(consensusLeader.blockHash[:])

	preparedMsg, _ := consensusLeader.constructPreparedMessage()
	aggSig := bls_cosi.AggregateSig(consensusLeader.GetPrepareSigsArray())
	multiSigAndBitmap := append(aggSig.Serialize(), consensusLeader.prepareBitmap.Bitmap...)

	consensusLeader.commitSigs[consensusLeader.SelfAddress] = consensusLeader.priKey.SignHash(multiSigAndBitmap)
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

	consensusValidator1, err := New(m, 0, leader, bls_cosi.RandPrivateKey())
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
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
