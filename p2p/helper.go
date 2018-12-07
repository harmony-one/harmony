package p2p

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
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

// BatchSizeInByte defines the size of buffer (64MB)
const BatchSizeInByte = 1 << 16

// ReadMessageContent reads the message type and content size, and return the actual content.
func ReadMessageContent(s Stream) ([]byte, error) {
	var (
		contentBuf = bytes.NewBuffer([]byte{})
		r          = bufio.NewReader(s)
	)
	timeoutDuration := 1 * time.Second
	s.SetReadDeadline(time.Now().Add(timeoutDuration))
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
	tmpBuf := make([]byte, BatchSizeInByte)
ILOOP:
	for {
		timeoutDuration := 10 * time.Second
		s.SetReadDeadline(time.Now().Add(timeoutDuration))
		if bytesToRead < BatchSizeInByte {
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
