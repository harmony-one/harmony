package p2pimpl

import (
	"net"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host/hostv1"
	"github.com/harmony-one/harmony/p2p/host/hostv2"

	"github.com/harmony-one/harmony/internal/utils"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
	p2p_config "github.com/libp2p/go-libp2p/config"
)

// Version The version number of p2p library
// 1 - Direct socket connection
// 2 - libp2p
const Version = 2

// NewHost starts the host for p2p
// for hostv2, it generates multiaddress, keypair and add PeerID to peer, add priKey to host
// TODO (leo) The peerstore has to be persisted on disk.
func NewHost(self *p2p.Peer, key p2p_crypto.PrivKey, opts ...p2p_config.Option) (p2p.Host, error) {
	if Version == 1 {
		h := hostv1.New(self)
		return h, nil
	}

	h := hostv2.New(self, key, opts...)

	utils.GetLogInstance().Info("NewHost", "self", net.JoinHostPort(self.IP, self.Port), "PeerID", self.PeerID)

	return h, nil
}
