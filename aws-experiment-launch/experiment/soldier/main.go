package main

import (
	"bufio"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	StopCharacter = "\r\n\r\n"
)

var (
	port *int
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
			if isTransportOver(data) {
				log.Println("Tranport Over!")
				break ILOOP
			}

			go handleCommand(data, w)

		default:
			log.Fatalf("Receive data failed:%s", err)
			return
		}

	}
}

func handleCommand(command string, w *bufio.Writer) {
	args := strings.Split(command, " ")

	if len(args) <= 0 {
		return
	}

	switch command := args[0]; command {
	case "init":
		{
			handleInitCommand(args[1:], w)
		}
	case "close":
		{
			log.Println("close command")
		}
	}
}

func handleInitCommand(args []string, w *bufio.Writer) {
	log.Println("Init command", args)
	// create local config file
	localConfig := "node_config_" + strconv.Itoa(*port) + ".txt"
	out, err := os.Create(localConfig)
	if err != nil {
		log.Fatal("Failed to create local file")
	}
	defer out.Close()

	// get remote config file
	configURL := args[0]
	resp, err := http.Get(configURL)
	if err != nil {
		log.Fatal("Failed to read file content")
	}
	defer resp.Body.Close()

	// copy remote to local
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Fatal("Failed to copy file")
	}

	content, err := ioutil.ReadFile(localConfig)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully init-ed with config", content)

	w.Write([]byte("Successfully init-ed"))
	w.Flush()
}

func isTransportOver(data string) (over bool) {
	over = strings.HasSuffix(data, "\r\n\r\n")
	return
}

func main() {
	port = flag.Int("port", 3333, "port of the node.")
	flag.Parse()

	SocketServer(*port)
}
