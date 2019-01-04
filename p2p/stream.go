package p2p

import "time"

// Stream is abstract p2p stream from where we read message
type Stream interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	SetReadDeadline(time.Time) error
}
