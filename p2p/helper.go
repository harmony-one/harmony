package p2p

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/log"
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
		log.Error("Error reading the p2p message type field", "io.EOF", err)
		return contentBuf.Bytes(), err
	case nil:
		//log.Printf("Received p2p message type: %x\n", msgType)
	default:
		log.Error("Error reading the p2p message type field", "msg", err)
		return contentBuf.Bytes(), err
	}
	// TODO: check on msgType and take actions accordingly
	//// Read 4 bytes for message size
	fourBytes := make([]byte, 4)
	n, err := r.Read(fourBytes)
	if err != nil {
		log.Error("Error reading the p2p message size field")
		return contentBuf.Bytes(), err
	} else if n < len(fourBytes) {
		log.Error("Failed reading the p2p message size field: only read", "Num of bytes", n)
		return contentBuf.Bytes(), err
	}

	contentLength := int(binary.BigEndian.Uint32(fourBytes))
	tmpBuf := make([]byte, contentLength)
	timeoutDuration = 20 * time.Second
	s.SetReadDeadline(time.Now().Add(timeoutDuration))
	m, err := io.ReadFull(r, tmpBuf)
	if err != nil || m < contentLength {
		log.Error("Read %v bytes, we need %v bytes", m, contentLength)
		return []byte{}, err
	}
	contentBuf.Write(tmpBuf)
	return contentBuf.Bytes(), nil
}
