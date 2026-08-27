package p2ptests

import (
	"testing"
	"time"

	"github.com/harmony-one/harmony/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostSetup(t *testing.T) {
	t.Parallel()

	hostData := helpers.Hosts[0]
	host, pubKey, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	assert.NoError(t, err)

	peer := host.GetSelfPeer()

	assert.Equal(t, hostData.IP, peer.IP)
	assert.Equal(t, hostData.Port, peer.Port)
	assert.Equal(t, pubKey, peer.ConsensusPubKey)
	assert.NotEmpty(t, peer.PeerID)
	assert.Equal(t, peer.PeerID, host.GetID())
	assert.Empty(t, peer.Addrs)
}

func TestAddPeer(t *testing.T) {
	t.Parallel()

	hostData := helpers.Hosts[0]
	host, _, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	assert.NoError(t, err)
	assert.NotEmpty(t, host.GetID())

	discoveredHostData := helpers.Hosts[1]
	discoveredHost, _, err := helpers.GenerateHost(discoveredHostData.IP, discoveredHostData.Port)
	assert.NoError(t, err)
	assert.NotEmpty(t, discoveredHost.GetID())

	discoveredPeer := discoveredHost.GetSelfPeer()

	assert.Empty(t, host.GetP2PHost().Peerstore().Addrs(discoveredHost.GetSelfPeer().PeerID))

	err = host.AddPeer(&discoveredPeer)
	assert.NoError(t, err)

	assert.NotEmpty(t, host.GetP2PHost().Peerstore().Addrs(discoveredHost.GetSelfPeer().PeerID))
	assert.Equal(t, 2, host.GetPeerCount())
}

/*func TestTopicJoining(t *testing.T) {
	t.Parallel()

	hostData := hosts[0]
	host, _, err := createNode(hostData.IP, hostData.Port)
	assert.NoError(t, err)
	assert.NotEmpty(t, host.GetID())

	for _, topicName := range topics {
		topic, err := host.GetOrJoin(topicName)
		assert.NoError(t, err)
		assert.NotNil(t, topic)
	}
}*/

func TestConnectionToInvalidPeer(t *testing.T) {
	t.Parallel()

	hostData := helpers.Hosts[0]
	host, _, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, host.Close()) })
	assert.NotEmpty(t, host.GetID())

	discoveredHostData := helpers.Hosts[1]
	discoveredHost, _, err := helpers.GenerateHost(discoveredHostData.IP, discoveredHostData.Port)
	require.NoError(t, err)
	assert.NotEmpty(t, discoveredHost.GetID())

	discoveredPeer := discoveredHost.GetSelfPeer()
	require.NoError(t, discoveredHost.Close())

	started := time.Now()
	err = host.ConnectHostPeer(discoveredPeer)
	assert.Error(t, err)
	if elapsed := time.Since(started); elapsed >= 5*time.Second {
		t.Fatalf("connection attempt took %s; expected failure within 5s", elapsed)
	}
}
