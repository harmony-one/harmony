package consensus

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/abool"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/consensus/quorum"
	coretypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/stretchr/testify/require"
)

func TestEmergencyRecoveryViewIDFloorScopeFailsClosed(t *testing.T) {
	require.Equal(t, uint64(1_000_000_000), EmergencyRecoveryViewIDFloor)
	require.Equal(t, uint64(92_730_034), EmergencyRecoveryShard0RetainedBlock)
	require.Equal(t, uint64(94_978_278), EmergencyRecoveryShard1RetainedBlock)

	mainnet := &params.ChainConfig{ChainID: new(big.Int).Set(params.MainnetChainID)}
	testnet := &params.ChainConfig{ChainID: new(big.Int).Set(params.TestnetChainID)}

	tests := []struct {
		name       string
		config     *params.ChainConfig
		shardID    uint32
		headHeight uint64
		applies    bool
		wantErr    error
	}{
		{name: "nil config", shardID: shard.BeaconChainShardID, headHeight: EmergencyRecoveryShard0RetainedBlock},
		{name: "testnet shard zero", config: testnet, shardID: shard.BeaconChainShardID, headHeight: EmergencyRecoveryShard0RetainedBlock},
		{name: "testnet shard one", config: testnet, shardID: 1, headHeight: EmergencyRecoveryShard1RetainedBlock},
		{name: "unsupported mainnet shard", config: mainnet, shardID: 2, headHeight: EmergencyRecoveryShard1RetainedBlock},
		{name: "shard zero before retained block", config: mainnet, shardID: shard.BeaconChainShardID, headHeight: EmergencyRecoveryShard0RetainedBlock - 1},
		{
			name:       "shard zero at retained block",
			config:     mainnet,
			shardID:    shard.BeaconChainShardID,
			headHeight: EmergencyRecoveryShard0RetainedBlock,
			applies:    true,
		},
		{
			name:       "shard zero after retained block",
			config:     mainnet,
			shardID:    shard.BeaconChainShardID,
			headHeight: EmergencyRecoveryShard0RetainedBlock + 1,
			applies:    true,
		},
		{name: "shard one before retained block", config: mainnet, shardID: 1, headHeight: EmergencyRecoveryShard1RetainedBlock - 1},
		{
			name:       "shard one at retained block",
			config:     mainnet,
			shardID:    1,
			headHeight: EmergencyRecoveryShard1RetainedBlock,
			applies:    true,
		},
		{
			name:       "shard one after retained block",
			config:     mainnet,
			shardID:    1,
			headHeight: EmergencyRecoveryShard1RetainedBlock + 1,
			applies:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			floor, applies, err := emergencyRecoveryViewIDFloorFor(test.config, test.shardID, test.headHeight)
			require.Equal(t, test.applies, applies)
			if test.applies && EmergencyRecoveryViewIDFloor == 0 {
				require.ErrorIs(t, err, ErrEmergencyRecoveryViewIDFloorUnset)
			} else {
				require.ErrorIs(t, err, test.wantErr)
				if test.applies {
					require.Equal(t, EmergencyRecoveryViewIDFloor, floor)
				}
			}
		})
	}
}

func TestRecoveryViewIDSettersAreFlooredAndMonotonic(t *testing.T) {
	state := NewState(Normal, shard.BeaconChainShardID)
	state.SetCurBlockViewID(10)
	state.SetViewChangingID(11)
	state.SetViewIDFloor(100)

	require.Equal(t, uint64(100), state.GetViewIDFloor())
	require.Equal(t, uint64(100), state.GetCurBlockViewID())
	require.Equal(t, uint64(100), state.GetViewChangingID())

	state.SetCurBlockViewID(1)
	state.SetViewChangingID(2)
	require.Equal(t, uint64(100), state.GetCurBlockViewID())
	require.Equal(t, uint64(100), state.GetViewChangingID())

	state.SetCurBlockViewID(105)
	state.SetViewChangingID(3)
	require.Equal(t, uint64(105), state.GetCurBlockViewID())
	require.Equal(t, uint64(105), state.GetViewChangingID())

	state.SetViewIDFloor(50)
	require.Equal(t, uint64(100), state.GetViewIDFloor())
}

func TestOrdinaryViewIDSettersMayLowerWithoutRecoveryFloor(t *testing.T) {
	state := NewState(Normal, shard.BeaconChainShardID)
	state.SetCurBlockViewID(10)
	state.SetViewChangingID(11)
	state.SetCurBlockViewID(3)
	state.SetViewChangingID(4)

	require.Equal(t, uint64(3), state.GetCurBlockViewID())
	require.Equal(t, uint64(4), state.GetViewChangingID())
}

func TestRecoveryNextViewIDUsesFloorAndStrictSuccessor(t *testing.T) {
	state := NewState(Normal, shard.BeaconChainShardID)
	state.SetViewIDFloor(100)

	next, err := state.nextViewID(1)
	require.NoError(t, err)
	require.Equal(t, uint64(101), next)

	next, _, err = state.getNextViewID(nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(101), next)

	state.blockViewID = math.MaxUint64
	_, err = state.nextViewID(1)
	require.ErrorIs(t, err, ErrViewIDExhausted)
	_, err = checkedNextViewID(math.MaxUint64)
	require.ErrorIs(t, err, ErrViewIDExhausted)
	_, err = checkedAddViewID(math.MaxUint64, 1)
	require.ErrorIs(t, err, ErrViewIDExhausted)

	gap, err := checkedLeaderViewGap(100, 90)
	require.NoError(t, err)
	require.Equal(t, 9, gap)
	_, err = checkedLeaderViewGap(89, 90)
	require.Error(t, err)
}

func TestRecoveryInboundMessagesCannotLowerViewID(t *testing.T) {
	consensus := &Consensus{
		current:           NewState(Normal, shard.BeaconChainShardID),
		IgnoreViewIDCheck: abool.NewBool(true),
	}
	consensus.current.SetViewIDFloor(100)
	message := &FBFTMessage{ViewID: 99}

	require.ErrorIs(t, consensus.checkViewID(message), ErrEmergencyRecoveryViewIDBelowFloor)
	require.False(t, consensus.onViewChangeSanityCheck(message))
	require.False(t, consensus.onNewViewSanityCheck(message))
	require.True(t, consensus.IgnoreViewIDCheck.IsSet())
}

func TestRecoveryConstructRefusesCorruptViewBelowFloor(t *testing.T) {
	consensus := &Consensus{current: NewState(Normal, shard.BeaconChainShardID)}
	consensus.current.SetViewIDFloor(100)
	// Emulate memory corruption or a future call site bypassing the setter.
	consensus.current.blockViewID = 99

	_, err := consensus.construct(msg_pb.MessageType_PREPARE, nil, nil)
	require.ErrorIs(t, err, ErrEmergencyRecoveryViewIDBelowFloor)
}

func TestRecoveryValidatedAnnounceCannotMaskBlockViewID(t *testing.T) {
	header := blockfactory.ForMainnet.NewHeader(big.NewInt(0)).With().
		Number(big.NewInt(1)).
		Epoch(big.NewInt(0)).
		ShardID(shard.BeaconChainShardID).
		ViewID(big.NewInt(99)).
		Header()
	block := coretypes.NewBlockWithHeader(header)
	blockPayload, err := rlp.EncodeToBytes(block)
	require.NoError(t, err)
	log := NewFBFTLog()

	consensus := &Consensus{
		current: NewState(Normal, shard.BeaconChainShardID),
		fBFTLog: log,
	}
	consensus.current.SetViewIDFloor(100)
	consensus.current.block = blockPayload
	consensus.current.blockHash = block.Hash()

	_, err = consensus.validateNewBlock(&FBFTMessage{
		Block:     blockPayload,
		BlockHash: block.Hash(),
		ViewID:    100,
	})
	require.ErrorContains(t, err, "block ViewID does not match message ViewID")
	require.Nil(t, log.GetBlockByHash(block.Hash()))
	require.ErrorIs(t, consensus.verifyEmergencyRecoveryBlock(block), ErrEmergencyRecoveryViewIDBelowFloor)
	require.ErrorIs(t, consensus.commitBlock(block, &FBFTMessage{ViewID: 99}), ErrEmergencyRecoveryViewIDBelowFloor)
	privateKey := bls.RandPrivateKey()
	publicKey := bls.PublicKeyWrapper{Object: privateKey.GetPublicKey()}
	publicKey.Bytes.FromLibBLSPublicKey(publicKey.Object)
	_, err = consensus.construct(msg_pb.MessageType_PREPARED, nil, []*bls.PrivateKeyWrapper{{Pri: privateKey, Pub: &publicKey}})
	require.ErrorContains(t, err, "block ViewID does not match message ViewID")
	require.ErrorContains(t, consensus.preCommitAndPropose(block), "block ViewID does not match message ViewID")
}

func TestNewViewLeaderSelectionDependsOnViewID(t *testing.T) {
	state := NewState(Normal, shard.BeaconChainShardID)
	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	wrappedKeys := make([]bls.PublicKeyWrapper, 0, 3)
	for range 3 {
		publicKey := bls.RandPrivateKey().GetPublicKey()
		serialized := bls.SerializedPublicKey{}
		serialized.FromLibBLSPublicKey(publicKey)
		wrappedKeys = append(wrappedKeys, bls.PublicKeyWrapper{Object: publicKey, Bytes: serialized})
	}
	decider.UpdateParticipants(wrappedKeys, nil)
	state.setLeaderPubKey(&wrappedKeys[0])

	viewOneLeader := state.getNextLeaderKey(nil, decider, 1, nil)
	viewTwoLeader := state.getNextLeaderKey(nil, decider, 2, nil)
	require.NotNil(t, viewOneLeader)
	require.NotNil(t, viewTwoLeader)
	require.True(t, viewOneLeader.Object.IsEqual(wrappedKeys[1].Object))
	require.True(t, viewTwoLeader.Object.IsEqual(wrappedKeys[2].Object))
}

func TestRecoveryLeaderSelectionUsesClampedNextViewID(t *testing.T) {
	state := NewState(Normal, shard.BeaconChainShardID)
	state.SetViewIDFloor(100)
	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)

	wrappedKeys := make([]bls.PublicKeyWrapper, 0, 3)
	for range 3 {
		privateKey := bls.RandPrivateKey()
		publicKey := privateKey.GetPublicKey()
		serialized := bls.SerializedPublicKey{}
		serialized.FromLibBLSPublicKey(publicKey)
		wrappedKeys = append(wrappedKeys, bls.PublicKeyWrapper{Object: publicKey, Bytes: serialized})
	}
	decider.UpdateParticipants(wrappedKeys, []bls.PublicKeyWrapper{})
	state.setLeaderPubKey(&wrappedKeys[0])

	// Without a chain header, leader selection advances once relative to the
	// current leader while still clamping the supplied view to the exact floor.
	next := state.getNextLeaderKey(nil, decider, 1, nil)
	require.NotNil(t, next)
	require.True(t, next.Object.IsEqual(wrappedKeys[1].Object))
}
