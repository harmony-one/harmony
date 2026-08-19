package legacysync

import "testing"

// TestFindPeerByHashIgnoresEmptyHash checks that a lookup with no hash matches
// no peer. bytes.Equal reports nil and empty as equal, so a request that leaves
// the field out would otherwise be routed to whichever peer has not had its own
// hash set yet.
func TestFindPeerByHashIgnoresEmptyHash(t *testing.T) {
	sc := &SyncConfig{}
	sc.peers = []*SyncPeerConfig{
		{peerHash: nil},
		{peerHash: []byte{0x01, 0x02}},
	}

	for _, empty := range [][]byte{nil, {}} {
		if got := sc.FindPeerByHash(empty); got != nil {
			t.Errorf("empty hash matched a peer with hash %v", got.peerHash)
		}
	}

	if got := sc.FindPeerByHash([]byte{0x01, 0x02}); got == nil {
		t.Fatal("a named peer should still be found")
	}
}
