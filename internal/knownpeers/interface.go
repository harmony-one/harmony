package knownpeers

import "github.com/harmony-one/harmony/p2p"

type KnownPeers interface {
	GetChecked(limit int) []p2p.Peer
	GetUnchecked(limit int) []p2p.Peer
	GetUncheckedCount() int
	AddChecked(hosts ...p2p.Peer)
	AddUnchecked(hosts ...p2p.Peer)
}
