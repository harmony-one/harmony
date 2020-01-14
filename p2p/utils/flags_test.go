package p2putils

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/p2p"
)

func TestStringsToAddrs(t *testing.T) {
	bn, err := StringsToAddrs(DefaultBootNodeAddrStrings)
	if err != nil {
		t.Fatalf("unable to convert string to addresses: %v", err)
	}
	if len(bn) == 0 {
		t.Fatalf("should have more than one multiaddress returned")
	}
}

func TestAddrListFunc(t *testing.T) {
	addr := new(AddrList)
	err := addr.Set("/ip4/127.0.0.1/tcp/9999/p2p/QmayB8NwxmfGE4Usb4H61M8uwbfc7LRbmXb3ChseJgbVuf,/ip4/127.0.0.1/tcp/9877/p2p/QmS374uzJ9yEEoWcEQ6JcbSUaVUj29SKakcmVvr3HVAjKP")
	if err != nil || len(*addr) < 2 {
		t.Fatalf("unable to set addr list")
	}
	s := fmt.Sprintf("addr: %s\n", addr)
	if len(s) == 0 {
		t.Fatalf("unable to print AddrList")
	}
}

func TestStringsToPeers(t *testing.T) {
	tests := []struct {
		input    string
		expected []p2p.Peer
	}{
		{
			"127.0.0.1:9000,192.168.192.1:8888,54.32.12.3:9898",
			[]p2p.Peer{
				{IP: "127.0.0.1", Port: "9000"},
				{IP: "192.168.192.1", Port: "8888"},
				{IP: "54.32.12.3", Port: "9898"},
			},
		},
		{
			"a:b,xx:XX,hello:world",
			[]p2p.Peer{
				{IP: "a", Port: "b"},
				{IP: "xx", Port: "XX"},
				{IP: "hello", Port: "world"},
			},
		},
	}

	for _, test := range tests {
		peers := StringsToPeers(test.input)
		if len(peers) != 3 {
			t.Errorf("StringsToPeers failure")
		}
		for i, p := range peers {
			if !reflect.DeepEqual(p, test.expected[i]) {
				t.Errorf("StringToPeers: expected: %v, got: %v", test.expected[i], p)
			}
		}
	}
}
