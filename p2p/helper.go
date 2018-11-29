package p2p

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"time"
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

// ReadMessageContent Read the message type and content size, and return the actual content.
func ReadMessageContent(conn net.Conn) ([]byte, error) {
	var (
		contentBuf = bytes.NewBuffer([]byte{})
		r          = bufio.NewReader(conn)
	)
	timeoutDuration := 1 * time.Second
	conn.SetReadDeadline(time.Now().Add(timeoutDuration))
	//// Read 1 byte for message type
	_, err := r.ReadByte()
	switch err {
	case io.EOF:
		log.Printf("Error reading the p2p message type field: %s", err)
		return contentBuf.Bytes(), err
	case nil:
		//log.Printf("Received p2p message type: %x\n", msgType)
	default:
		log.Printf("Error reading the p2p message type field: %s", err)
		return contentBuf.Bytes(), err
	}
	// TODO: check on msgType and take actions accordingly
	//// Read 4 bytes for message size
	fourBytes := make([]byte, 4)
	n, err := r.Read(fourBytes)
	if err != nil {
		log.Printf("Error reading the p2p message size field")
		return contentBuf.Bytes(), err
	} else if n < len(fourBytes) {
		log.Printf("Failed reading the p2p message size field: only read %d bytes", n)
		return contentBuf.Bytes(), err
	}
	//log.Print(fourBytes)
	// Number of bytes for the message content
	bytesToRead := binary.BigEndian.Uint32(fourBytes)
	//log.Printf("The content size is %d bytes.", bytesToRead)
	//// Read the content in chunk of 16 * 1024 bytes
	tmpBuf := make([]byte, BATCH_SIZE)
ILOOP:
	for {
		timeoutDuration := 10 * time.Second
		conn.SetReadDeadline(time.Now().Add(timeoutDuration))
		if bytesToRead < BATCH_SIZE {
			// Read the last number of bytes less than 1024
			tmpBuf = make([]byte, bytesToRead)
		}
		n, err := r.Read(tmpBuf)
		contentBuf.Write(tmpBuf[:n])
		switch err {
		case io.EOF:
			// TODO: should we return error here, or just ignore it?
			log.Printf("EOF reached while reading p2p message")
			break ILOOP
		case nil:
			bytesToRead -= uint32(n) // TODO: think about avoid the casting in every loop
			if bytesToRead <= 0 {
				break ILOOP
			}
		default:
			log.Printf("Error reading p2p message")
			return []byte{}, err
		}
	}
	return contentBuf.Bytes(), nil
}

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
