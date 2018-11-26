package p2p

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
)

/*
P2p Message data structure:

---- message start -----
1 byte            - p2p type
                    0x00: unicast (no need to forward)
                    0x01: broadcast (may need to forward to other neighbors)
4 bytes           - message content size n in number of bytes
content (n bytes) - actual message content
----  message end  -----


*/

const BATCH_SIZE = 1 << 16

func CreateMessage(msgType byte, data []byte) []byte {
	buffer := bytes.NewBuffer([]byte{})

	buffer.WriteByte(msgType)

	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, uint32(len(data)))
	buffer.Write(fourBytes)

	buffer.Write(data)
	return buffer.Bytes()
}

func SendMessageContent(conn net.Conn, data []byte) {
	msgToSend := CreateMessage(byte(1), data)
	w := bufio.NewWriter(conn)
	w.Write(msgToSend)
	w.Flush()
}
