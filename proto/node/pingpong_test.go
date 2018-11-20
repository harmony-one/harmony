package node

import (
	"fmt"
	"strings"
	"testing"
)

var (
	p1 = PingMessageType{"127.0.0.1", "9999", "0x12345678901234567890"}
	e1 = "127.0.0.1:9999/0x12345678901234567890"

	p2 = PongMessageType{
		[]string{
			"0x1111111111111",
			"0x2222222222222",
			"0x3333333333333",
		},
	}
	e2 = "# Keys: 3"

	buf1 []byte
	buf2 []byte
)

func TestString(test *testing.T) {
	r1 := fmt.Sprintf("%v", p1)
	if strings.Compare(r1, e1) != 0 {
		test.Errorf("expect: %v, got: %v", e1, r1)
	} else {
		fmt.Printf("Ping:%v\n", p1)
	}

	r2 := fmt.Sprintf("%v", p2)

	if strings.Compare(r2, e2) != 0 {
		test.Errorf("expect: %v, got: %v", e2, r2)
	} else {
		fmt.Printf("Pong:%v\n", p2)
	}
}

func TestSerialize(test *testing.T) {
	buf1 = p1.ConstructPingMessage()
	fmt.Printf("buf: %v\n", buf1)

	buf2 = p2.ConstructPongMessage()
	fmt.Printf("buf: %v\n", buf2)
}

func TestDeserialize(test *testing.T) {
	ping, err := GetPingMessage(buf1)
	if err != nil {
		test.Error("Ping failed!")
	}
	fmt.Printf("Ping:%v\n", ping)

	pong, err := GetPongMessage(buf2)
	if err != nil {
		test.Error("Pong failed!")
	}
	fmt.Printf("Pong:%v\n", pong)

}
