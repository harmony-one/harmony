// https://gist.github.com/kenshinx/5796276
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

// Consts
const (
	Message       = "Ping"
	StopCharacter = "\r\n\r\n"
)

var received map[int]int

// SocketClient code.
func SocketClient(ip, message string, port int) (res string) {
	addr := strings.Join([]string{ip, strconv.Itoa(port)}, ":")
	conn, err := net.Dial("tcp", addr)

	defer conn.Close()

	if err != nil {
		log.Fatalln(err)
	}

	conn.Write([]byte(message))
	conn.Write([]byte(StopCharacter))
	log.Printf("Send: %s", Message)

	buff := make([]byte, 1024)
	n, _ := conn.Read(buff)
	res = string(buff[:n])
	return
}

// Send code.
func Send(port int, message string, ch chan int) (returnMessage string) {
	ip := "127.0.0.1"
	returnMessage = SocketClient(ip, message, port)
	ch <- port
	fmt.Println(returnMessage)
	return
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func convertIntoInts(data string) []int {
	var res = []int{}
	items := strings.Split(data, " ")
	for _, value := range items {
		intValue, err := strconv.Atoi(value)
		checkError(err)
		res = append(res, intValue)
	}
	return res
}

// Receive request.
func handler(conn net.Conn) {

	defer conn.Close()

	var (
		buf = make([]byte, 1024)
		r   = bufio.NewReader(conn)
		w   = bufio.NewWriter(conn)
	)

	receivedMessage := ""
ILOOP:
	for {
		n, err := r.Read(buf)
		data := string(buf[:n])

		receivedMessage += data

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
	receivedMessage = strings.TrimSpace(receivedMessage)
	ports := convertIntoInts(receivedMessage)
	ch := make(chan int)
	for i, port := range ports {
		go Send(port, strconv.Itoa(i), ch)
	}
	count := 0
	for count < len(ports) {
		fmt.Println(<-ch)
		count++
	}
	w.Write([]byte(Message))
	w.Flush()
	log.Printf("Send: %s", Message)

}

func isTransportOver(data string) (over bool) {
	over = strings.HasSuffix(data, "\r\n\r\n")
	return
}

// SocketServer for leader.
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

func main() {
	// ports := convertIntoPorts("1 2 3")
	// fmt.Println(ports)
	// os.Exit(0)
	port := flag.Int("port", 0, "port number.")
	flag.Parse()
	if *port == 0 {
		var err error
		*port, err = strconv.Atoi(os.Getenv("LEADER_PORT"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "No port provided.")
			os.Exit(1)
		}
	}

	log.Printf("I'm a leader node with port %d", *port)
	SocketServer(*port)
}
