package p2p_test

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"net"
	"reflect"
	"testing"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

func setUpTestServer(times int, t *testing.T, conCreated chan struct{}) {
	t.Parallel()
	ln, _ := net.Listen("tcp", ":8081")
	conCreated <- struct{}{}
	conn, _ := ln.Accept()
	defer conn.Close()

	var (
		w = bufio.NewWriter(conn)
	)
	for times > 0 {
		times--
		data, err := p2p.ReadMessageContent(conn)
		if err != nil {
			t.Fatalf("error when ReadMessageContent %v", err)
		}
		data = p2p.CreateMessage(byte(1), data)
		w.Write(data)
		w.Flush()
	}
}
func TestNewNewNode(t *testing.T) {
	times := 100
	conCreated := make(chan struct{})
	go setUpTestServer(times, t, conCreated)
	<-conCreated

	conn, _ := net.Dial("tcp", "127.0.0.1:8081")
	defer conn.Close()

	for times > 0 {
		times--

		myMsg := "minhdoan"
		p2p.SendMessageContent(conn, []byte(myMsg))

		data, err := p2p.ReadMessageContent(conn)
		if err != nil {
			t.Error("got an error when trying to receive an expected message from server.")
		}
		if string(data) != myMsg {
			t.Error("did not receive expected message")
		}
	}
}

func TestGobEncode(t *testing.T) {
	block := blockchain.CreateTestingGenesisBlock()

	var tmp bytes.Buffer
	enc := gob.NewEncoder(&tmp)
	enc.Encode(*block)

	tmp2 := bytes.NewBuffer(tmp.Bytes())
	dec := gob.NewDecoder(tmp2)

	var block2 blockchain.Block
	dec.Decode(&block2)
	if !reflect.DeepEqual(*block, block2) {
		t.Error("Error in GobEncode")
	}
}
