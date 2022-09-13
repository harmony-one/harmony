package legacysync

import (
	"testing"

	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	s := NewStorage()
	p1 := p2p.Peer{IP: "127.0.0.1", Port: "1"}
	require.False(t, s.Contains(p1))

	s.AddPeer(p1)
	require.True(t, s.Contains(p1))

	peers := s.GetPeers(1)
	require.Equal(t, 1, len(peers))

	require.True(t, s.IsSuccess(p1))
	require.False(t, s.IsFail(p1))

	s.Fail(p1)

	require.False(t, s.IsSuccess(p1))
	require.True(t, s.IsFail(p1))

	require.Equal(t, 1, len(s.GetPeers(1)))
}
