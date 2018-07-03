package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
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
	log.Printf("Begin listen for command on port: %d", port)

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
			go handleCommand(data)
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

func handleCommand(command string) {
	// assume this is init command
	handleInitCommand(command)
}

func handleInitCommand(command string) {
	log.Println("Init command")
	out, err := os.Create("config_copy.txt")
	if err != nil {
		panic("Failed to create local file")
	}
	log.Println("Created local file")
	defer out.Close()
	resp, err := http.Get("http://localhost/config.txt")
	if err != nil {
		log.Println("Failed to read file content")
		panic("Failed to read file content")
	}
	log.Println("Read file content")
	log.Println(resp)
	log.Println(resp.Body)
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		panic("Failed to copy file")
	}
	log.Println("copy done")
	log.Println(resp.Body)
	defer resp.Body.Close()
	log.Println(n)
}

func isTransportOver(data string) (over bool) {
	over = strings.HasSuffix(data, "\r\n\r\n")
	return
}

func main() {

	port := 3333

	SocketServer(port)

}
