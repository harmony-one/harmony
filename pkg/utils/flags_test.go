package utils

import (
	"fmt"
	"testing"
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
