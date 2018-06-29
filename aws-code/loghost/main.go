package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

const (
	CONN_PORT = "3000"
	CONN_TYPE = "tcp"
)

func main() {
	// Listen for incoming connections.
	l, err := net.Listen(CONN_TYPE, ":"+CONN_PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()
	fmt.Println("Listening on " + ":" + CONN_PORT)
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
	// // Make a buffer to hold incoming data.
	// buf := make([]byte, 1024)
	// // Read the incoming connection into the buffer.
	// reqLen, err := conn.Read(buf)
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Println("Error reading:", err.Error())
	}
	// fmt.Printf("Received %v: %v", reqLen, buf)
	fmt.Println(status)
	// // Send a response back to person contacting xus.
	// conn.Write([]byte("Message received."))
	// Close the connection when you're done with it.
	conn.Close()
}
