package message

import (
	"testing"
)

const (
	testIP = "127.0.0.1"
)

func TestClient(t *testing.T) {
	s := NewServer()
	s.Start(testIP, "")

	client := NewClient(testIP)
	client.Process(&Message{})
	s.Stop()
}
