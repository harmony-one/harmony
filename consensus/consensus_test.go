package consensus

import (
	"sync"
	"testing"
	"time"

	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/harmony-one/harmony/test/helpers"
	"github.com/stretchr/testify/assert"
)

func TestConsensusInitialization(t *testing.T) {
	host, multiBLSPrivateKey, consensus, decider, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	peer := host.GetSelfPeer()

	messageSender := &MessageSender{host: host, retryTimes: int(phaseDuration.Seconds()) / RetryIntervalInSec}
	fbtLog := NewFBFTLog()
	state := State{mode: Normal}

	timeouts := createTimeout()
	expectedTimeouts := make(map[TimeoutType]time.Duration)
	expectedTimeouts[timeoutConsensus] = phaseDuration
	expectedTimeouts[timeoutViewChange] = viewChangeDuration
	expectedTimeouts[timeoutBootstrap] = bootstrapDuration

	assert.Equal(t, decider, consensus.Decider)
	assert.Equal(t, host, consensus.host)
	assert.Equal(t, messageSender, consensus.msgSender)
	assert.IsType(t, make(chan struct{}), consensus.BlockNumLowChan)

	// FBFTLog
	assert.Equal(t, fbtLog, consensus.FBFTLog)
	assert.Equal(t, maxLogSize, consensus.FBFTLog.maxLogSize)

	assert.Equal(t, FBFTAnnounce, consensus.phase)

	// State / consensus.current
	assert.Equal(t, state.mode, consensus.current.mode)
	assert.Equal(t, state.viewID, consensus.current.viewID)

	// FBFT timeout
	assert.IsType(t, make(map[TimeoutType]*utils.Timeout), consensus.consensusTimeout)
	for timeoutType, timeout := range expectedTimeouts {
		duration := consensus.consensusTimeout[timeoutType].Duration()
		assert.Equal(t, timeouts[timeoutType].Duration().Nanoseconds(), duration.Nanoseconds())
		assert.Equal(t, timeout.Nanoseconds(), duration.Nanoseconds())
	}

	// Validators sync.Map
	assert.IsType(t, sync.Map{}, consensus.validators)
	assert.NotEmpty(t, consensus.validators)

	validatorLength := 0
	consensus.validators.Range(func(_, _ interface{}) bool {
		validatorLength++
		return true
	})

	assert.Equal(t, 1, validatorLength)
	retrievedPeer, loadedOk := consensus.validators.Load(peer.ConsensusPubKey.SerializeToHexStr())
	assert.Equal(t, true, loadedOk)
	assert.Equal(t, retrievedPeer, peer)

	// MultiBLS
	assert.Equal(t, multiBLSPrivateKey, consensus.priKey)
	assert.Equal(t, multiBLSPrivateKey.GetPublicKeys(), consensus.GetPublicKeys())

	// Misc
	assert.Equal(t, uint64(0), consensus.viewID)
	assert.Equal(t, uint32(shard.BeaconChainShardID), consensus.ShardID)

	assert.IsType(t, make(chan struct{}), consensus.syncReadyChan)
	assert.NotNil(t, consensus.syncReadyChan)

	assert.IsType(t, make(chan struct{}), consensus.syncNotReadyChan)
	assert.NotNil(t, consensus.syncNotReadyChan)

	assert.IsType(t, make(chan slash.Record), consensus.SlashChan)
	assert.NotNil(t, consensus.SlashChan)

	assert.IsType(t, make(chan uint64), consensus.commitFinishChan)
	assert.NotNil(t, consensus.commitFinishChan)

	assert.IsType(t, make(chan struct{}), consensus.ReadySignal)
	assert.NotNil(t, consensus.ReadySignal)

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

	peer := host.GetSelfPeer()

	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	multiBLSPrivateKey := multibls.GetPrivateKeys(bls.RandPrivateKey())

	consensus, err := New(host, shard.BeaconChainShardID, peer, multiBLSPrivateKey, decider)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return host, multiBLSPrivateKey, consensus, decider, nil
}
