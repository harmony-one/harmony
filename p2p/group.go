package p2p

import (
	"context"
	"fmt"
	"io"

	peer "github.com/libp2p/go-libp2p-peer"
)

// GroupID is a multicast group ID.
//
// It is a binary string,
// conducive to layering and scoped generation using cryptographic hash.
//
// Applications define their own group ID, without central allocation.
// A cryptographically secure random string of enough length – 32 bytes for
// example – may be used.
type GroupID string

func (id GroupID) String() string {
	return fmt.Sprintf("%x", string(id))
}

// Const of group ID
const (
	GroupIDBeacon GroupID = "harmony/0.0.1/beacon"
	GroupIDGlobal GroupID = "harmony/0.0.1/global"
)

// GroupReceiver is a multicast group message receiver interface.
type GroupReceiver interface {
	// Close closes this receiver.
	io.Closer

	// Receive a message.
	Receive(ctx context.Context) (msg []byte, sender peer.ID, err error)
}
