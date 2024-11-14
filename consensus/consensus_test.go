package consensus

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/harmony-one/harmony/test/helpers"
	"github.com/stretchr/testify/assert"
)

func TestConsensusInitialization(t *testing.T) {
	host, multiBLSPrivateKey, consensus, _, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	messageSender := &MessageSender{host: host, retryTimes: int(phaseDuration.Seconds()) / RetryIntervalInSec}
	state := NewState(Normal)

	timeouts := createTimeout()
	expectedTimeouts := make(map[TimeoutType]time.Duration)
	expectedTimeouts[timeoutConsensus] = phaseDuration
	expectedTimeouts[timeoutViewChange] = viewChangeDuration
	expectedTimeouts[timeoutBootstrap] = bootstrapDuration

	assert.Equal(t, host, consensus.host)
	assert.Equal(t, messageSender, consensus.msgSender)

	// FBFTLog
	assert.NotNil(t, consensus.FBFTLog())

	assert.Equal(t, FBFTAnnounce, consensus.phase)

	// State / consensus.current
	assert.Equal(t, state.mode, consensus.current.mode)
	assert.Equal(t, state.GetViewChangingID(), consensus.current.GetViewChangingID())

	// FBFT timeout
	assert.IsType(t, make(map[TimeoutType]*utils.Timeout), consensus.consensusTimeout)
	for timeoutType, timeout := range expectedTimeouts {
		duration := consensus.consensusTimeout[timeoutType].Duration()
		assert.Equal(t, timeouts[timeoutType].Duration().Nanoseconds(), duration.Nanoseconds())
		assert.Equal(t, timeout.Nanoseconds(), duration.Nanoseconds())
	}

	// MultiBLS
	assert.Equal(t, multiBLSPrivateKey, consensus.priKey)
	assert.Equal(t, multiBLSPrivateKey.GetPublicKeys(), consensus.GetPublicKeys())

	// Misc
	assert.Equal(t, uint64(0), consensus.GetViewChangingID())
	assert.Equal(t, uint32(shard.BeaconChainShardID), consensus.ShardID)

	assert.Equal(t, false, consensus.start)

	assert.IsType(t, make(chan slash.Record), consensus.SlashChan)
	assert.NotNil(t, consensus.SlashChan)

	assert.IsType(t, make(chan [vdfAndSeedSize]byte), consensus.RndChannel)
	assert.NotNil(t, consensus.RndChannel)

	assert.IsType(t, abool.NewBool(false), consensus.IgnoreViewIDCheck)
	assert.NotNil(t, consensus.IgnoreViewIDCheck)
}

// GenerateConsensusForTesting - helper method to generate a basic consensus
func GenerateConsensusForTesting() (p2p.Host, multibls.PrivateKeys, *Consensus, quorum.Decider, error) {
	hostData := helpers.Hosts[0]
	host, _, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	multiBLSPrivateKey := multibls.GetPrivateKeys(bls.RandPrivateKey())

	consensus, err := New(host, shard.BeaconChainShardID, multiBLSPrivateKey, registry.New(), decider, 3, false)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return host, multiBLSPrivateKey, consensus, decider, nil
}

func TestCountSigned(t *testing.T) {

	multiBLSPrivateKey := multibls.GetPrivateKeys(bls.RandPrivateKey(), bls.RandPrivateKey(), bls.RandPrivateKey())
	bitmaps := make([][]byte, 0, 3)

	for i := 0; i < blocksCountAliveness; i++ {
		mask := bls.NewMask(multiBLSPrivateKey.GetPublicKeys())
		for _, k := range multiBLSPrivateKey.GetPublicKeys() {
			ok, err := mask.KeyEnabled(k.Bytes)
			require.NoError(t, err)
			require.False(t, ok, "key is not enabled")

			mask.SetKey(k.Bytes, true)

			ok, err = mask.KeyEnabled(k.Bytes)
			require.NoError(t, err)
			require.True(t, ok, "key should exist")
		}
		bitmaps = append(bitmaps, mask.Mask())
	}
	// as we signed all bitmaps, we should achieve 4 sigs
	cs := CountSigned(bls.NewMask(multiBLSPrivateKey.GetPublicKeys()), bitmaps, multiBLSPrivateKey[0].Pub)
	require.Equal(t, blocksCountAliveness, cs)

	// check unknown public key
	unknownKey := multibls.GetPrivateKeys(bls.RandPrivateKey())
	cs = CountSigned(bls.NewMask(multiBLSPrivateKey.GetPublicKeys()), bitmaps, unknownKey[0].Pub)

	require.Equal(t, 0, cs, "no signature should be counted")

}
