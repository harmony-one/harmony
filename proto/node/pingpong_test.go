package node

import (
	"fmt"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
)

var (
	priKey1 = crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 = pki.GetPublicKeyFromScalar(priKey1)
	p1      = p2p.Peer{
		Ip:     "127.0.0.1",
		Port:   "9999",
		PubKey: pubKey1,
	}
	e1 = "1=>127.0.0.1:9999/5ad91c4440d3a0e83df49ff4a0243da1edf2ec2d9376ed58ea7ac6bc9d745ae4"

	priKey2 = crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 = pki.GetPublicKeyFromScalar(priKey2)

	p2 = []p2p.Peer{
		{
			Ip:          "127.0.0.1",
			Port:        "8888",
			PubKey:      pubKey1,
			Ready:       true,
			ValidatorID: 1,
		},
		{
			Ip:          "127.0.0.1",
			Port:        "9999",
			PubKey:      pubKey2,
			Ready:       false,
			ValidatorID: 2,
		},
	}
	e2 = "1=># Peers: 2"

	buf1 []byte
	buf2 []byte
)

func TestString(test *testing.T) {
	ping1 := NewPingMessage(p1)

	r1 := fmt.Sprintf("%v", *ping1)
	if strings.Compare(r1, e1) != 0 {
		test.Errorf("expect: %v, got: %v", e1, r1)
	} else {
		fmt.Printf("Ping:%v\n", r1)
	}

	pong1 := NewPongMessage(p2)
	r2 := fmt.Sprintf("%v", *pong1)

	if !strings.HasPrefix(r2, e2) {
		test.Errorf("expect: %v, got: %v", e2, r2)
	} else {
		fmt.Printf("Pong:%v\n", r2)
	}
}

func TestSerialize(test *testing.T) {
	ping1 := NewPingMessage(p1)
	buf1 = ping1.ConstructPingMessage()
	fmt.Printf("buf ping: %v\n", buf1)

	pong1 := NewPongMessage(p2)
	buf2 = pong1.ConstructPongMessage()
	fmt.Printf("buf pong: %v\n", buf2)
}

func TestDeserialize(test *testing.T) {
	msg1, err := proto.GetMessagePayload(buf1)
	if err != nil {
		test.Error("GetMessagePayload Failed!")
	}
	ping, err := GetPingMessage(msg1)
	if err != nil {
		test.Error("Ping failed!")
	}
	fmt.Printf("Ping:%v\n", ping)

	msg2, err := proto.GetMessagePayload(buf2)
	pong, err := GetPongMessage(msg2)
	if err != nil {
		test.Error("Pong failed!")
	}
	fmt.Printf("Pong:%v\n", pong)

}
