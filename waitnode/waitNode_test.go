package waitnode

import (
	"testing"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

func TestNewNode(test *testing.T) {
	p := p2p.Peer{Ip: "127.0.0.1", Port: "8080"}
	wn := New(p)
	b := wn.SerializeWaitNode()
	wnd := DeserializeWaitNode(b)
	if *wn != *wnd {
		test.Error("Serialization is not working")
	}

}
