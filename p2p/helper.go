package p2p

import (
	"bufio"
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
func ReadMessageContent(s Stream) (content []byte, err error) {
	var (
		r = bufio.NewReader(s)
	)
	timeoutDuration := 1 * time.Second
	if err = s.SetReadDeadline(time.Now().Add(timeoutDuration)); err != nil {
		log.Error("cannot reset deadline for message header read", "error", err)
		return
	}
	//// Read 1 byte for message type
	if _, err = r.ReadByte(); err != nil {
		log.Error("failed to read p2p message type field", "error", err)
		return
	}
	// TODO: check on msgType and take actions accordingly
	//// Read 4 bytes for message size
	fourBytes := make([]byte, 4)
	if _, err = io.ReadFull(r, fourBytes); err != nil {
		log.Error("failed to read p2p message size field", "error", err)
		return
	}

	contentLength := int(binary.BigEndian.Uint32(fourBytes))
	contentBuf := make([]byte, contentLength)
	timeoutDuration = 20 * time.Second
	if err = s.SetReadDeadline(time.Now().Add(timeoutDuration)); err != nil {
		log.Error("cannot reset deadline for message content read", "error", err)
		return
	}
	if _, err = io.ReadFull(r, contentBuf); err != nil {
		log.Error("failed to read p2p message contents", "error", err)
		return
	}
	content = contentBuf
	return
}
