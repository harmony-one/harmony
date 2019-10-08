// Package msgq implements a simple, finite-sized message queue.  It can be used
// as a building block for a message processor pool.
package msgq

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
)

type message struct {
	content []byte
	sender  peer.ID
}

// MessageHandler is a message handler.
type MessageHandler interface {
	HandleMessage(content []byte, sender peer.ID)
}

// Queue is a finite-sized message queue.
type Queue struct {
	ch chan message
}

// New returns a new message queue of the given size.
func New(size int) *Queue {
	return &Queue{ch: make(chan message, size)}
}

// AddMessage enqueues a received message for processing.  It returns without
// blocking, and may return a queue overrun error.
func (q *Queue) AddMessage(content []byte, sender peer.ID) error {
	select {
	case q.ch <- message{content, sender}:
	default:
		return ErrRxOverrun
	}
	return nil
}

// HandleMessages dequeues and dispatches incoming messages using the given
// message handler, until the message queue is closed.  This function can be
// spawned as a background goroutine, potentially multiple times for a pool.
func (q *Queue) HandleMessages(h MessageHandler) {
	for msg := range q.ch {
		h.HandleMessage(msg.content, msg.sender)
	}
}

// ErrRxOverrun signals that a receive queue has been overrun.
var ErrRxOverrun = errors.New("rx overrun")
