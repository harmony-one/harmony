package discovery

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/p2p"
)

var (
	pubKey1 = pki.GetBLSPrivateKeyFromInt(333).GetPublicKey()
	p1      = p2p.Peer{
		IP:              "127.0.0.1",
		Port:            "9999",
		ConsensusPubKey: pubKey1,
	}
	e1 = "ping:Validator/1=>127.0.0.1:9999/[120 1 130 197 30 202 78 236 84 249 5 230 132 208 242 242 246 63 100 123 96 11 211 228 4 56 64 94 57 133 3 226 254 222 231 160 178 81 252 205 40 28 45 2 90 74 207 15 68 86 138 68 143 176 221 161 108 105 133 6 64 121 92 25 134 255 9 209 156 209 119 187 13 160 23 147 240 24 196 152 100 20 163 51 118 45 100 26 179 227 184 166 147 113 50 139]"
	e3 = "ping:Client/1=>127.0.0.1:9999/[120 1 130 197 30 202 78 236 84 249 5 230 132 208 242 242 246 63 100 123 96 11 211 228 4 56 64 94 57 133 3 226 254 222 231 160 178 81 252 205 40 28 45 2 90 74 207 15 68 86 138 68 143 176 221 161 108 105 133 6 64 121 92 25 134 255 9 209 156 209 119 187 13 160 23 147 240 24 196 152 100 20 163 51 118 45 100 26 179 227 184 166 147 113 50 139]"

	pubKey2 = pki.GetBLSPrivateKeyFromInt(999).GetPublicKey()

	p2 = []p2p.Peer{
		{
			IP:              "127.0.0.1",
			Port:            "8888",
			ConsensusPubKey: pubKey1,
		},
		{
			IP:              "127.0.0.1",
			Port:            "9999",
			ConsensusPubKey: pubKey2,
		},
	}
	e2 = "pong:1=>length:2"

	leaderPubKey = pki.GetBLSPrivateKeyFromInt(888).GetPublicKey()

	pubKeys = []*bls.PublicKey{pubKey1, pubKey2}

	buf1 []byte
	buf2 []byte
)

func TestString(test *testing.T) {
	ping1 := NewPingMessage(p1, false)

	r1 := fmt.Sprintf("%v", *ping1)
	if strings.Compare(r1, e1) != 0 {
		test.Errorf("expect: %v, got: %v", e1, r1)
	}

	ping1.Node.Role = node.ClientRole

	r3 := fmt.Sprintf("%v", *ping1)
	if strings.Compare(r3, e3) != 0 {
		test.Errorf("expect: %v, got: %v", e3, r3)
	}

	pong1 := NewPongMessage(p2, pubKeys, leaderPubKey)
	r2 := fmt.Sprintf("%v", *pong1)

	if !strings.HasPrefix(r2, e2) {
		test.Errorf("expect: %v, got: %v", e2, r2)
	}
}

func TestSerialize(test *testing.T) {
	ping1 := NewPingMessage(p1, true)
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

	pong1 := NewPongMessage(p2, pubKeys, leaderPubKey)
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
