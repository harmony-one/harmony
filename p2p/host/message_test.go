package host

import (
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host/hostv2"
	"log"
	"reflect"
	"testing"
	"time"
)

func TestSendMessage(test *testing.T) {
	peer1 := p2p.Peer{IP: "127.0.0.1", Port: "9000"}
	peer2 := p2p.Peer{IP: "127.0.0.1", Port: "9001"}
	msg := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	host1 := hostv2.New(peer1)
	host2 := hostv2.New(peer2)
	go host2.BindHandlerAndServe(handler)
	SendMessage(host1, peer2, msg, nil)
	time.Sleep(3 * time.Second)

}

func handler(s p2p.Stream) {
	defer s.Close()
	content, err := p2p.ReadMessageContent(s)
	if err != nil {
		panic("Read p2p data failed")
		return
	}
	log.Printf("received: %v", content)
	golden := []byte{0x00, 0x01, 0x02, 0x03, 0x04}

	if !reflect.DeepEqual(content, golden) {
		panic("received message not equal original message")
	}

}
