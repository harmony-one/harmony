package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

const (
	CONN_HOST = "localhost"
	CONN_PORT = "3000"
	CONN_TYPE = "tcp"
	CONN_URL  = CONN_HOST + ":" + CONN_PORT
)

func main() {
	// Listen for incoming connections.
	l, err := net.Listen(CONN_TYPE, CONN_URL)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()
	fmt.Println("Listening on " + CONN_URL)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {
	for {
		data, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Println("Error reading:", err.Error())
			break
		}
		fmt.Println(data)
	}
}
