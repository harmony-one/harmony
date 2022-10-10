package p2p

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGaterBlocking(t *testing.T) {
	gater := NewGater(true)
	require.NotNil(t, &gater, "%s", &gater)

	public, err := ma.NewMultiaddr("/ip4/1.1.1.1/udp/53")
	assert.Nil(t, err, "%s", err)
	allowed := gater.InterceptAddrDial("somePeer", public)
	assert.True(t, allowed, "%b", allowed)

	private, err := ma.NewMultiaddr("/ip4/192.168.1.1/tcp/80")
	assert.Nil(t, err, "%s", err)
	allowed = gater.InterceptAddrDial("somePeer", private)
	assert.False(t, allowed, "%b", allowed)
}

func TestGaterNotBlocking(t *testing.T) {
	gater := NewGater(false)
	require.NotNil(t, &gater, "%s", &gater)

	public, err := ma.NewMultiaddr("/ip4/1.1.1.1/udp/53")
	assert.Nil(t, err, "%s", err)
	allowed := gater.InterceptAddrDial("somePeer", public)
	assert.True(t, allowed, "%b", allowed)

	private, err := ma.NewMultiaddr("/ip4/192.168.1.1/tcp/80")
	assert.Nil(t, err, "%s", err)
	allowed = gater.InterceptAddrDial("somePeer", private)
	assert.True(t, allowed, "%b", allowed)
}
