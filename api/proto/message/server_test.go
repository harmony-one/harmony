package message

import (
	"testing"
	"time"
)

func TestServerStart(t *testing.T) {
	s := NewServer()
	s.Start("127.0.0.1", "")
	time.Sleep(time.Second)
	s.Stop()
}
