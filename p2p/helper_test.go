package p2p_test

import (
	"testing"
)

func setUpTestServer(times int, t *testing.T, conCreated chan struct{}) {
	//TODO(minhdoan)
	// t.Parallel()
	// ln, _ := net.Listen("tcp", ":8081")
	// conCreated <- struct{}{}
	// conn, _ := ln.Accept()
	// defer conn.Close()

	// var (
	// 	w = bufio.NewWriter(conn)
	// )
	// for times > 0 {
	// 	times--
	// 	data, err := p2p.ReadMessageContent(conn)
	// 	if err != nil {
	// 		t.Fatalf("error when ReadMessageContent %v", err)
	// 	}
	// 	data = p2p.CreateMessage(byte(1), data)
	// 	w.Write(data)
	// 	w.Flush()
	// }
}
func TestNewNewNode(t *testing.T) {
	// TODO(minhdoan):
	// times := 100
	// conCreated := make(chan struct{})
	// go setUpTestServer(times, t, conCreated)
	// <-conCreated

	// conn, _ := net.Dial("tcp", "127.0.0.1:8081")
	// defer conn.Close()

	// for times > 0 {
	// 	times--

	// 	myMsg := "minhdoan"
	// 	p2p.SendMessageContent(conn, []byte(myMsg))

	// 	data, err := p2p.ReadMessageContent(conn)
	// 	if err != nil {
	// 		t.Error("got an error when trying to receive an expected message from server.")
	// 	}
	// 	if string(data) != myMsg {
	// 		t.Error("did not receive expected message")
	// 	}
	// }
}
