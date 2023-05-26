package knownpeers_test

import (
	"testing"

	"github.com/harmony-one/harmony/internal/knownpeers"
	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/require"
)

var (
	p1 = p2p.Peer{IP: "1", Port: "80"}
	p2 = p2p.Peer{IP: "2", Port: "80"}
	p3 = p2p.Peer{IP: "3", Port: "80"}
	p4 = p2p.Peer{IP: "4", Port: "80"}
)

type Peers = []p2p.Peer

func TestKnownHosts_GetCheckedHosts(t *testing.T) {
	n := knownpeers.NewKnownPeers(100, 100)
	n.AddChecked(p1, p2, p3)
	require.Equal(t, Peers{p1, p2}, n.GetChecked(2))
	require.Equal(t, Peers{p3, p1}, n.GetChecked(2))

	n.AddChecked(p4)
	require.Equal(t, Peers{p2, p3, p4}, n.GetChecked(3))
}

func TestKnownHosts_Unchecked(t *testing.T) {
	n := knownpeers.NewKnownPeers(100, 100)
	n.AddUnchecked(p1, p2, p3)
	require.Equal(t, Peers{p1, p2}, n.GetUnchecked(2))
	require.Equal(t, Peers{p3, p1}, n.GetUnchecked(2))

	n.AddUnchecked(p4)
	require.Equal(t, Peers{p2, p3, p4}, n.GetUnchecked(3))

	n.AddChecked(p1)
	require.NotContains(t, n.GetUnchecked(3), "1",
		"do not check order, it's detail of implementation, just check absence")

	// check no panics on empty
	n.AddChecked(p2, p3, p4)
	require.Empty(t, n.GetUnchecked(3))
}

func TestKnownHosts_CheckEmpty(t *testing.T) {
	n := knownpeers.NewKnownPeers(100, 100)
	require.Empty(t, n.GetChecked(1))
	require.Empty(t, n.GetUnchecked(1))
}

func TestKnownPeers_Limit(t *testing.T) {
	n := knownpeers.NewKnownPeers(0, 2)
	n.AddUnchecked(p1, p2, p3)
	require.Equal(t, 2, n.GetUncheckedCount())
}
