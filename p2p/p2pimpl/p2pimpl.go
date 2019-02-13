package p2pimpl

import (
	"net"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host/hostv2"

	"github.com/harmony-one/harmony/internal/utils"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
)

// NewHost starts the host for p2p
// for hostv2, it generates multiaddress, keypair and add PeerID to peer, add priKey to host
// TODO (leo) The peerstore has to be persisted on disk.
func NewHost(self *p2p.Peer, key p2p_crypto.PrivKey) (p2p.Host, error) {
	h := hostv2.New(self, key)

	utils.GetLogInstance().Info("NewHost", "self", net.JoinHostPort(self.IP, self.Port), "PeerID", self.PeerID)

	return h, nil
}
