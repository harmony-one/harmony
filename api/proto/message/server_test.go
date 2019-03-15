package message

import (
	"testing"
	"time"
)

func TestServerStart(t *testing.T) {
	s := NewServer(nil, nil)
	s.Start()
	time.Sleep(time.Second)
	s.Stop()
}
