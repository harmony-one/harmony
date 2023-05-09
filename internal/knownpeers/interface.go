package knownpeers

import "github.com/harmony-one/harmony/p2p"

type KnownPeers interface {
	GetChecked(limit int) []p2p.Peer
	GetUnchecked(limit int) []p2p.Peer
	AddChecked(hosts ...p2p.Peer)
	AddUnchecked(hosts ...p2p.Peer)
}
