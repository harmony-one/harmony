package node

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/p2p"
)

var (
	priKey1 = crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 = pki.GetPublicKeyFromScalar(priKey1)
	p1      = p2p.Peer{
		IP:          "127.0.0.1",
		Port:        "9999",
		ValidatorID: -1,
		PubKey:      pubKey1,
	}
	e1 = "ping:Validator/1=>127.0.0.1:9999:-1/[90 217 28 68 64 211 160 232 61 244 159 244 160 36 61 161 237 242 236 45 147 118 237 88 234 122 198 188 157 116 90 228]"
	e3 = "ping:Client/1=>127.0.0.1:9999:-1/[90 217 28 68 64 211 160 232 61 244 159 244 160 36 61 161 237 242 236 45 147 118 237 88 234 122 198 188 157 116 90 228]"

	priKey2 = crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 = pki.GetPublicKeyFromScalar(priKey2)

	p2 = []p2p.Peer{
		{
			IP:          "127.0.0.1",
			Port:        "8888",
			PubKey:      pubKey1,
			Ready:       true,
			ValidatorID: -1,
		},
		{
			IP:          "127.0.0.1",
			Port:        "9999",
			PubKey:      pubKey2,
			Ready:       false,
			ValidatorID: -2,
		},
	}
	e2 = "pong:1=>length:2"

	pubKeys = []kyber.Point{pubKey1, pubKey2}

	buf1 []byte
	buf2 []byte
)

func TestString(test *testing.T) {
	ping1 := NewPingMessage(p1)

	r1 := fmt.Sprintf("%v", *ping1)
	if strings.Compare(r1, e1) != 0 {
		test.Errorf("expect: %v, got: %v", e1, r1)
	}

	ping1.Node.Role = ClientRole

	r3 := fmt.Sprintf("%v", *ping1)
	if strings.Compare(r3, e3) != 0 {
		test.Errorf("expect: %v, got: %v", e3, r3)
	}

	pong1 := NewPongMessage(p2, pubKeys)
	r2 := fmt.Sprintf("%v", *pong1)

	if !strings.HasPrefix(r2, e2) {
		test.Errorf("expect: %v, got: %v", e2, r2)
	}
}

func TestSerialize(test *testing.T) {
	ping1 := NewPingMessage(p1)
	buf1 = ping1.ConstructPingMessage()
	msg1, err := proto.GetMessagePayload(buf1)
	if err != nil {
		test.Error("GetMessagePayload Failed!")
	}
	ping, err := GetPingMessage(msg1)
	if err != nil {
		test.Error("Ping failed!")
	}
	if !reflect.DeepEqual(ping, ping1) {
		test.Error("Serialize/Deserialze Ping Message Failed")
	}

	pong1 := NewPongMessage(p2, pubKeys)
	buf2 = pong1.ConstructPongMessage()

	msg2, err := proto.GetMessagePayload(buf2)
	pong, err := GetPongMessage(msg2)
	if err != nil {
		test.Error("Pong failed!")
	}

	if !reflect.DeepEqual(pong, pong1) {
		test.Error("Serialize/Deserialze Pong Message Failed")
	}
}
