package blockedpeers

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	var (
		peer1 peer.ID = "peer1"
		now           = time.Now()
		m             = NewManager(4)
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
