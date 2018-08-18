package identitychain

import "github.com/simple-rules/harmony-benchmark/p2p"

// IdentityBlock has the information of one node
type IdentityBlock struct {
	Peer          p2p.Peer
	NumIdentities int32
}
