package p2pv2

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/harmony-one/harmony/log"
	libp2p "github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	multiaddr "github.com/multiformats/go-multiaddr"
)

var (
	myHost host.Host // TODO(ricl): this should be a field in node.
)

const BATCH_SIZE = 1 << 16

func InitHost(port string) {
	addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%s", port)
	sourceAddr, err := multiaddr.NewMultiaddr(addr)
	catchError(err)
	priv := portToPrivKey(port)
	myHost, err = libp2p.New(context.Background(),
		libp2p.ListenAddrs(sourceAddr),
		libp2p.Identity(priv))
	catchError(err)
	myHost.SetStreamHandler("/harmony/0.0.1", readStream)
	log.Debug("Host is up!", "port", port, "id", myHost.ID().Pretty(), "addrs", sourceAddr)
}

func readStream(s net.Stream) {
	log.Debug("READ!!")
	timeoutDuration := 1 * time.Second
	s.SetReadDeadline(time.Now().Add(timeoutDuration))

	// Create a buffered stream so that read and writes are non blocking.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// Create a thread to read and write data.
	go readData(rw)
}

func readData(rw *bufio.ReadWriter) {
	contentBuf := bytes.NewBuffer([]byte{})
	// Read 1 byte for message type
	_, err := rw.ReadByte()
	switch err {
	case nil:
		//log.Printf("Received p2p message type: %x\n", msgType)
	case io.EOF:
		fallthrough
	default:
		log.Error("Error reading the p2p message type field", "err", err)
		return //contentBuf.Bytes(), err
	}
	// TODO: check on msgType and take actions accordingly

	// Read 4 bytes for message size
	fourBytes := make([]byte, 4)
	n, err := rw.Read(fourBytes)
	if err != nil {
		log.Error("Error reading the p2p message size field", "err", err)
		return //contentBuf.Bytes(), err
	} else if n < len(fourBytes) {
		log.Error("Invalid byte size", "bytes", n)
		return //contentBuf.Bytes(), err
	}

	//log.Print(fourBytes)
	// Number of bytes for the message content
	bytesToRead := binary.BigEndian.Uint32(fourBytes)
	//log.Printf("The content size is %d bytes.", bytesToRead)

	// Read the content in chunk of 16 * 1024 bytes
	tmpBuf := make([]byte, BATCH_SIZE)
ILOOP:
	for {
		// TODO(ricl): is this necessary? If yes, figure out how to make it work
		// timeoutDuration := 10 * time.Second
		// s.SetReadDeadline(time.Now().Add(timeoutDuration))
		if bytesToRead < BATCH_SIZE {
			// Read the last number of bytes less than 1024
			tmpBuf = make([]byte, bytesToRead)
		}
		n, err := rw.Read(tmpBuf)
		contentBuf.Write(tmpBuf[:n])

		switch err {
		case io.EOF:
			// TODO: should we return error here, or just ignore it?
			log.Error("EOF reached while reading p2p message")
			break ILOOP
		case nil:
			bytesToRead -= uint32(n) // TODO: think about avoid the casting in every loop
			if bytesToRead <= 0 {
				break ILOOP
			}
		default:
			log.Error("Error reading p2p message")
			return //[]byte{}, err
		}
	}
	//return contentBuf.Bytes(), nil
}

func writeData(w *bufio.Writer, data []byte) {
	for {
		w.Write(data)
		w.Flush()
	}
}

func GetHost() host.Host {
	return myHost
}
