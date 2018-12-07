package p2p

import (
	"time"

	"github.com/dedis/kyber"
)

// Stream is abstract p2p stream from where we read message
type Stream interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	SetReadDeadline(time.Time) error
}

// StreamHandler handles incoming p2p message.
type StreamHandler func(Stream)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP          string      // IP address of the peer
	Port        string      // Port number of the peer
	PubKey      kyber.Point // Public key of the peer
	Ready       bool        // Ready is true if the peer is ready to join consensus.
	ValidatorID int         // -1 is the default value, means not assigned any validator ID in the shard
	// TODO(minhdoan, rj): use this Ready to not send/broadcast to this peer if it wasn't available.
}
