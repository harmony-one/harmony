package msgq

import (
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		cap  int
	}{
		{"unbuffered", 0},
		{"buffered10", 10},
		{"buffered100", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.cap)
			if cap(got.ch) != tt.cap {
				t.Errorf("New() ch cap %d, want %d", cap(got.ch), tt.cap)
			}
		})
	}
}

func TestQueue_EnqueueItem(t *testing.T) {
	tests := []struct {
		name string
		cap  int
	}{
		{"unbuffered", 0},
		{"buffered", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &Queue{ch: make(chan interface{}, tt.cap)}
			for i := 0; i < tt.cap+10; i++ {
				var wantErr error
				if i >= tt.cap {
					wantErr = ErrOverrun
				} else {
					wantErr = nil
				}
				err := q.EnqueueItem("dummy item")
				if err != wantErr {
					t.Fatalf("EnqueueItem() iter %d, error = %v, want %v",
						i, err, wantErr)
				}
			}
		})
	}
}

type testItemHandler struct {
	t   *testing.T
	seq int
}

func (h *testItemHandler) HandleItem(item interface{}) {
	got, want := item.(string), fmt.Sprint(h.seq)
	if got != want {
		h.t.Errorf("out-of-sequence item %v, want %v", got, want)
	}
	h.seq++
}

func TestQueue_HandleItems(t *testing.T) {
	ch := make(chan interface{}, 500)
	for seq := 0; seq < cap(ch); seq++ {
		ch <- fmt.Sprint(seq)
	}
	close(ch)
	q := &Queue{ch: ch}
	q.HandleItems(&testItemHandler{t: t})
}

func TestQueue_Close(t *testing.T) {
	q := &Queue{ch: make(chan interface{}, 100)}
	err := q.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	select {
	case m, ok := <-q.ch:
		if ok {
			t.Errorf("unexpected message %v", m)
		}
	default:
		t.Error("channel closed but not ready")
	}
}
