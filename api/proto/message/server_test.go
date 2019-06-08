package message

import (
	"testing"
	"time"
)

func TestServerStart(t *testing.T) {
	s := NewServer(nil, nil, nil)
	if _, err := s.Start(); err != nil {
		t.Fatalf("cannot start server: %s", err)
	}
	time.Sleep(time.Second)
	s.Stop()
}
