package hostv2

import (
	"log"
	"reflect"
	"testing"
)

func TestAddrToPrivKey(test *testing.T) {
	addr := "/ip4/0.0.0.0/tcp/8999"
	priv1 := addrToPrivKey(addr)
	priv2 := addrToPrivKey(addr)
	by1, err := priv1.Bytes()
	if err != nil {
		test.Error("cannot get bytes of private key")
	}
	by2, err := priv2.Bytes()
	log.Printf("private key 1: %v", by1)
	log.Printf("private key 2: %v", by2)

	if err != nil {
		test.Error("cannot get bytes of private key")
	}
	if !reflect.DeepEqual(by1, by2) {
		test.Error("two private keys should be equal")
	}

}
