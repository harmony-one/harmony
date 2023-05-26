package knownpeers

import "github.com/harmony-one/harmony/p2p"

/**
Checked peer is a peer that we have successfully connected to.
Unchecked peer is a peer that we have not yet connected to.
*/

type KnownPeers interface {
	// GetChecked returns a list of checked peers, up to limit.
	GetChecked(limit int) []p2p.Peer
	// GetUnchecked returns a list of unchecked peers, up to limit.
	GetUnchecked(limit int) []p2p.Peer
	GetUncheckedCount() int
	GetCheckedCount() int
	AddChecked(hosts ...p2p.Peer)
	AddUnchecked(hosts ...p2p.Peer)
	RemoveChecked(hosts ...p2p.Peer)
	RemoveUnchecked(hosts ...p2p.Peer)
}
