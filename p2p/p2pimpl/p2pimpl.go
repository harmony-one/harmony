package p2pimpl

import (
	"net"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host/hostv1"
	"github.com/harmony-one/harmony/p2p/host/hostv2"

	"github.com/harmony-one/harmony/internal/utils"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Version The version number of p2p library
// 1 - Direct socket connection
// 2 - libp2p
const Version = 2

// NewHost starts the host for p2p
// for hostv2, it generates multiaddress, keypair and add PeerID to peer, add priKey to host
// TODO (leo) the PriKey of the host has to be persistent in disk, so that we don't need to regenerate it
// on the same host if the node software restarted. The peerstore has to be persistent as well.
func NewHost(self *p2p.Peer) (p2p.Host, error) {
	if Version == 1 {
		h := hostv1.New(self)
		return h, nil
	}

	// TODO (leo), change to GenKeyP2PRand() to generate random key. Right now, the key is predictable as the
	// seed is fixed.
	priKey, pubKey, err := utils.GenKeyP2P(self.IP, self.Port)
	if err != nil {
		return nil, err
	}

	peerID, err := peer.IDFromPublicKey(pubKey)

	if err != nil {
		return nil, err
	}

	self.PeerID = peerID
	h := hostv2.New(*self, priKey)

	utils.GetLogInstance().Info("NewHost", "self", net.JoinHostPort(self.IP, self.Port), "PeerID", self.PeerID)

	return h, nil
}
