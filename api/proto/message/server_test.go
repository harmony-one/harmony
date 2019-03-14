package message

import (
	"testing"
	"time"
)

func TestServerStart(t *testing.T) {
	s := NewServer()
	s.Start()
	time.Sleep(time.Second)
	s.Stop()
}
