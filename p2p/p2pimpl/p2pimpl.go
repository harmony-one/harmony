package p2pimpl

import (
	"fmt"
	"net"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/host/hostv1"
	"github.com/harmony-one/harmony/p2p/host/hostv2"

	"github.com/harmony-one/harmony/internal/utils"
	peer "github.com/libp2p/go-libp2p-peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// Version The version number of p2p library
// 1 - Direct socket connection
// 2 - libp2p
const Version = 2

// NewHost starts the host for p2p
// for hostv2, it generates multiaddress, keypair and add PeerID to peer, add priKey to host
// TODO (leo) the PriKey of the host has to be persistent in disk, so that we don't need to regenerate it
// on the same host if the node software restarted. The peerstore has to be persistent as well.
func NewHost(self *p2p.Peer) (host.Host, error) {
	if Version == 1 {
		h := hostv1.New(self)
		return h, nil
	}

	selfAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	if err != nil {
		return nil, err
	}
	self.Addrs = append(self.Addrs, selfAddr)

	// TODO (leo), change to GenKeyP2PRand() to generate random key. Right now, the key is predicable as the
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

	fmt.Printf("NewHost => self:%s, PeerID: %v\n", net.JoinHostPort(self.IP, self.Port), self.PeerID)

	return h, nil
}
