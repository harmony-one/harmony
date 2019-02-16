package p2p

import "time"

//go:generate mockgen -source stream.go -package p2p -destination=mock_stream.go

// Stream is abstract p2p stream from where we read message
type Stream interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	SetReadDeadline(time.Time) error
}
