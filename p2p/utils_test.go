package p2p

import (
	"reflect"
	"testing"

	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestConnectCallbacks(t *testing.T) {
	cbs := ConnectCallbacks{}
	fn := func(net libp2p_network.Network, conn libp2p_network.Conn) error {
		return nil
	}

	require.Equal(t, 0, len(cbs.GetAll()))

	cbs.Add(fn)

	require.Equal(t, 1, len(cbs.GetAll()))
	require.Equal(t, reflect.ValueOf(fn).Pointer(), reflect.ValueOf(cbs.GetAll()[0]).Pointer())
}

func TestDisConnectCallbacks(t *testing.T) {
	cbs := DisconnectCallbacks{}
	fn := func(conn libp2p_network.Conn) error {
		return nil
	}

	require.Equal(t, 0, len(cbs.GetAll()))

	cbs.Add(fn)

	require.Equal(t, 1, len(cbs.GetAll()))
	require.Equal(t, reflect.ValueOf(fn).Pointer(), reflect.ValueOf(cbs.GetAll()[0]).Pointer())
}
