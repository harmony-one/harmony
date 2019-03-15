package message

import (
	"testing"
)

const (
	testIP = "127.0.0.1"
)

func TestClient(t *testing.T) {
	s := NewServer(nil, nil)
	s.Start()

	client := NewClient(testIP)
	_, err := client.Process(&Message{})
	if err == nil {
		t.Errorf("Not expected.")
	}
	s.Stop()
}
