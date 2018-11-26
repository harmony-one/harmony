package downloader

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/pki"
)

const (
	serverPort = "9997"
	serverIP   = "127.0.0.1"
	clientPort = "9999"
)

var (
	PriIntOne      = 111
	PriIntTwo      = 222
	TestAddressOne = pki.GetAddressFromInt(PriIntOne)
	TestAddressTwo = pki.GetAddressFromInt(PriIntTwo)
)

func TestGetBlockHashes(t *testing.T) {
	s := NewServer(nil)
	grcpServer, err := s.Start(serverIP, serverPort)
	if err != nil {
		t.Error(err)
	}
	defer grcpServer.Stop()

	client := ClientSetup(serverIP, serverPort)
	payload := client.GetHeaders()
	if payload[2] != 2 {
		t.Error("minh")
	}

	defer client.Close()
	client.GetHeaders()
}
