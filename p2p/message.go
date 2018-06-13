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
1 byte            - message type
                    0x00 - normal message (no need to forward)
                    0x11 - p2p message (may need to forward to other neighbors)
4 bytes           - message size n in number of bytes
payload (n bytes) - actual message payload
----  message end  -----

*/

// Read the message type and payload size, and return the actual payload.
func ReadMessagePayload(conn net.Conn) ([]byte, error) {
	var (
		payloadBuf = bytes.NewBuffer([]byte{})
		r          = bufio.NewReader(conn)
	)

	timeoutDuration := 1 * time.Second
	conn.SetReadDeadline(time.Now().Add(timeoutDuration))

	//// Read 1 byte for messge type
	_, err := r.ReadByte()
	switch err {
	case io.EOF:
		log.Printf("Error reading the p2p message type field: %s", err)
		return payloadBuf.Bytes(), err
	case nil:

		//log.Printf("Received p2p message type: %x\n", msgType)
	default:
		log.Printf("Error reading the p2p message type field: %s", err)
		return payloadBuf.Bytes(), err
	}
	// TODO: check on msgType and take actions accordingly

	//// Read 4 bytes for message size
	fourBytes := make([]byte, 4)
	n, err := r.Read(fourBytes)
	if err != nil {
		log.Printf("Error reading the p2p message size field")
		return payloadBuf.Bytes(), err
	} else if n < len(fourBytes) {
		log.Printf("Failed reading the p2p message size field: only read %d bytes", n)
		return payloadBuf.Bytes(), err
	}

	//log.Print(fourBytes)
	// Number of bytes for the message payload
	bytesToRead := binary.BigEndian.Uint32(fourBytes)
	//log.Printf("The payload size is %d bytes.", bytesToRead)

	//// Read the payload in chunk of 1024 bytes
	tmpBuf := make([]byte, 1024)
ILOOP:
	for {
		timeoutDuration := 1 * time.Second
		conn.SetReadDeadline(time.Now().Add(timeoutDuration))
		if bytesToRead < 1024 {
			// Read the last number of bytes less than 1024
			tmpBuf = make([]byte, bytesToRead)
		}
		n, err := r.Read(tmpBuf)
		payloadBuf.Write(tmpBuf[:n])

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
	return payloadBuf.Bytes(), nil
}
