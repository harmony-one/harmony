// Package msgq implements a simple, finite-sized, tail-dropping queue.  It can
// be used as a building block for a message processor pool.
package msgq

import (
	"github.com/pkg/errors"
)

// Enqueuer enqueues an item for processing.  It returns without blocking, and
// may return a queue overrun error.
type Enqueuer interface {
	EnqueueItem(item interface{}) error
}

// Handler handles an item out of a queue.
type Handler interface {
	HandleItem(item interface{})
}

// Queue is a finite-sized, tail-dropping queue.
type Queue struct {
	ch chan interface{}
}

// New returns a new queue of the given size.
func New(size int) *Queue {
	return &Queue{ch: make(chan interface{}, size)}
}

// EnqueueItem enqueues an item for processing.  It returns without blocking,
// and may return a queue overrun error.
func (q *Queue) EnqueueItem(item interface{}) error {
	select {
	case q.ch <- item:
	default:
		return ErrOverrun
	}
	return nil
}

// HandleItems dequeues and dispatches items in the queue using the given
// handler, until the queue is closed.  This function can be spawned as a
// background goroutine, potentially multiple times for a pool.
func (q *Queue) HandleItems(h Handler) {
	for item := range q.ch {
		h.HandleItem(item)
	}
}

// Close closes the given queue.
func (q *Queue) Close() error {
	close(q.ch)
	return nil
}

// ErrOverrun signals that a queue has been overrun.
var ErrOverrun = errors.New("queue overrun")
