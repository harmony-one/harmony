package identitychain

import (
	"fmt"
	"os"
	"testing"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

func TestIDCFormed(test *testing.T) {
	peer := p2p.Peer{Ip: "127.0.0.1", Port: "8080"}
	IDC := New(peer)
	if IDC == nil {
		fmt.Println("IDC not formed.")
		os.Exit(1)
	}
}

//TODO Mock netconnection to test whether identitychain is listening.
