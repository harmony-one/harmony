package gating

import (
	"testing"

	ds "github.com/ipfs/go-datastore"
	dsSync "github.com/ipfs/go-datastore/sync"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDataStore() ds.Batching {
	return dsSync.MutexWrap(ds.NewMapDatastore())
}
func TestGaterBlocking(t *testing.T) {
	store := createTestDataStore()
	gater, err := NewExtendedConnectionGater(store)
	assert.Nil(t, err, "%s", err)
	require.NotNil(t, &gater, "%s", &gater)

	gater = AddBlocking(gater, true)

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
	store := createTestDataStore()
	gater, err := NewExtendedConnectionGater(store)
	assert.Nil(t, err, "%s", err)
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
