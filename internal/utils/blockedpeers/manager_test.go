package blockedpeers

import (
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	var (
		peer1 libp2p_peer.ID = "peer1"
		now                  = time.Now()
		m                    = NewManager(4)
	)

	t.Run("check_empty", func(t *testing.T) {
		require.False(t, m.IsBanned(peer1, now), "peer1 should not be banned")
	})
	t.Run("ban_peer1", func(t *testing.T) {
		m.Ban(peer1, now.Add(2*time.Second))
		require.True(t, m.IsBanned(peer1, now), "peer1 should be banned")
		require.False(t, m.IsBanned(peer1, now.Add(3*time.Second)), "peer1 should not be banned after 3 seconds")
	})

}
