package p2p

import (
	"context"
	"fmt"
	"io"

	libp2p_peer "github.com/libp2p/go-libp2p-peer"
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
	GroupIDBeacon       GroupID = "harmony/0.0.1/beacon"
	GroupIDBeaconClient GroupID = "harmony/0.0.1/beacon/client"
	GroupIDGlobal       GroupID = "harmony/0.0.1/global"
	GroupIDUnknown      GroupID = "B1acKh0lE"
)

// ActionType lists action on group
type ActionType uint

// Const of different Action type
const (
	ActionStart ActionType = iota
	ActionPause
	ActionResume
	ActionStop
	ActionUnknown
)

func (a ActionType) String() string {
	switch a {
	case ActionStart:
		return "ActionStart"
	case ActionPause:
		return "ActionPause"
	case ActionResume:
		return "ActionResume"
	case ActionStop:
		return "ActionStop"
	}
	return "ActionUnknown"
}

// GroupAction specify action on corresponding group
type GroupAction struct {
	Name   GroupID
	Action ActionType
}

func (g GroupAction) String() string {
	return fmt.Sprintf("%s/%s", g.Name, g.Action)
}

// GroupReceiver is a multicast group message receiver interface.
type GroupReceiver interface {
	// Close closes this receiver.
	io.Closer

	// Receive a message.
	Receive(ctx context.Context) (msg []byte, sender libp2p_peer.ID, err error)
}
