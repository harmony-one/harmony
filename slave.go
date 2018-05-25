package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	Message       = "Pong"
	StopCharacter = "\r\n\r\n"
)

func SocketServer(port int) {

	listen, err := net.Listen("tcp4", ":"+strconv.Itoa(port))
	defer listen.Close()
	if err != nil {
		log.Fatalf("Socket listen port %d failed,%s", port, err)
		os.Exit(1)
	}
	log.Printf("Begin listen port: %d", port)

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatalln(err)
			continue
		}
		go handler(conn)
	}

}

func handler(conn net.Conn) {

	defer conn.Close()

	var (
		buf = make([]byte, 1024)
		r   = bufio.NewReader(conn)
		w   = bufio.NewWriter(conn)
	)

ILOOP:
	for {
		n, err := r.Read(buf)
		data := string(buf[:n])

		switch err {
		case io.EOF:
			break ILOOP
		case nil:
			log.Println("Receive:", data)
			if isTransportOver(data) {
				break ILOOP
			}

		default:
			log.Fatalf("Receive data failed:%s", err)
			return
		}

	}
	w.Write([]byte(Message))
	w.Flush()
	log.Printf("Send: %s", Message)

}

func isTransportOver(data string) (over bool) {
	over = strings.HasSuffix(data, "\r\n\r\n")
	return
}

func main() {
	port := flag.Int("port", 0, "port number.")
	flag.Parse()
	if *port == 0 {
		fmt.Fprintf(os.Stderr, "No port provided")
		os.Exit(1)
	}
	fmt.Println("I'm a slave node with port %d.", *port)
	SocketServer(*port)
}
