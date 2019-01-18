package consensus

import (
	"encoding/hex"
	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

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

	host, _ := p2pimpl.NewHost(&leader)
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	blockBytes, err := hex.DecodeString("f90461f90222a0f7007987c6f26b20cbd6384e3587445eca556beb6716f8eb6a2f590ce8ed3925940000000000000000000000000000000000000000a0f4f2f8416b65c98890630b105f016370abaab236c92faf7fc73a13d037958c52a0db025c6f785698feb447b509908fe488486062e4607afaae85c3336692445b01a03688be0d6b3d0651911204b4539e11096045cacbb676401e2655653823014c8cb90100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008001850254a0e6f88303295d845c2e4f0e80a00000000000000000000000000000000000000000000000000000000000000000880000000000000000840000000080b842000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f90238f9023580808083081650808b069e10de76676d08000000b901db6080604052678ac7230489e8000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058203e799228fee2fa7c5d15e71c04267a0cc2687c5eff3b48b98f21f355e1064ab300291ba0a87b9130f7f127af3a713a270610da48d56dedc9501e624bdfe04871859c88f3a05a94b087c05c6395825c5fc35d5ce96b2e61f0ce5f2d67b28f9b2d1178fa90f0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("2e002b2b91a08b6e94d21200103828d9f2ae7cd9eb0c26d2679966699486dee1")

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

	assert.Equal(test, CommitDone, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}

func TestProcessMessageValidatorChallenge(test *testing.T) {
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

	host, _ := p2pimpl.NewHost(&leader)
	consensusLeader := New(host, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	blockBytes, err := hex.DecodeString("f90461f90222a0f7007987c6f26b20cbd6384e3587445eca556beb6716f8eb6a2f590ce8ed3925940000000000000000000000000000000000000000a0f4f2f8416b65c98890630b105f016370abaab236c92faf7fc73a13d037958c52a0db025c6f785698feb447b509908fe488486062e4607afaae85c3336692445b01a03688be0d6b3d0651911204b4539e11096045cacbb676401e2655653823014c8cb90100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008001850254a0e6f88303295d845c2e4f0e80a00000000000000000000000000000000000000000000000000000000000000000880000000000000000840000000080b842000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f90238f9023580808083081650808b069e10de76676d08000000b901db6080604052678ac7230489e8000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058203e799228fee2fa7c5d15e71c04267a0cc2687c5eff3b48b98f21f355e1064ab300291ba0a87b9130f7f127af3a713a270610da48d56dedc9501e624bdfe04871859c88f3a05a94b087c05c6395825c5fc35d5ce96b2e61f0ce5f2d67b28f9b2d1178fa90f0c0")
	consensusLeader.block = blockBytes
	hashBytes, err := hex.DecodeString("2e002b2b91a08b6e94d21200103828d9f2ae7cd9eb0c26d2679966699486dee1")

	copy(consensusLeader.blockHash[:], hashBytes[:])

	commitMsg := consensusLeader.constructAnnounceMessage()
	challengeMsg, _, _ := consensusLeader.constructChallengeMessage(consensus_proto.MessageType_CHALLENGE)

	if err != nil {
		test.Errorf("Failed to unmarshal message payload")
	}

	consensusValidator1 := New(m, "0", []p2p.Peer{validator1, validator2, validator3}, leader)
	consensusValidator1.BlockVerifier = func(block *types.Block) bool {
		return true
	}

	message := consensus_proto.Message{}
	err = message.XXX_Unmarshal(commitMsg[1:])
	copy(consensusValidator1.blockHash[:], hashBytes[:])
	consensusValidator1.processAnnounceMessage(message)

	message = consensus_proto.Message{}
	err = message.XXX_Unmarshal(challengeMsg[1:])
	consensusValidator1.processChallengeMessage(message, ResponseDone)

	assert.Equal(test, ResponseDone, consensusValidator1.state)

	time.Sleep(1 * time.Second)
}
