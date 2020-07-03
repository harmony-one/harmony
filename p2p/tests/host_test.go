package p2ptests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostSetup(t *testing.T) {
	t.Parallel()

	hostData := hosts[0]
	host, pubKey, err := createNode(hostData.IP, hostData.Port)
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

	hostData := hosts[0]
	host, _, err := createNode(hostData.IP, hostData.Port)
	assert.NoError(t, err)
	assert.NotEmpty(t, host.GetID())

	discoveredHostData := hosts[1]
	discoveredHost, _, err := createNode(discoveredHostData.IP, discoveredHostData.Port)
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

	hostData := hosts[0]
	host, _, err := createNode(hostData.IP, hostData.Port)
	assert.NoError(t, err)
	assert.NotEmpty(t, host.GetID())

	discoveredHostData := hosts[1]
	discoveredHost, _, err := createNode(discoveredHostData.IP, discoveredHostData.Port)
	assert.NoError(t, err)
	assert.NotEmpty(t, discoveredHost.GetID())

	discoveredPeer := discoveredHost.GetSelfPeer()

	err = host.ConnectHostPeer(discoveredPeer)
	assert.Error(t, err)
}
