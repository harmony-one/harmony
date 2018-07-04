package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"strings"
)

const (
	message       = "init http://localhost:8080/config.txt"
	StopCharacter = "\r\n\r\n"
)

func SocketClient(addr string) {
	conn, err := net.Dial("tcp", addr)

	defer conn.Close()

	if err != nil {
		log.Fatalln(err)
	}

	conn.Write([]byte(message))
	// conn.Write([]byte(StopCharacter))
	log.Printf("Send: %s", message)

	buff := make([]byte, 1024)
	n, _ := conn.Read(buff)
	log.Printf("Receive from %s: %s", addr, buff[:n])
}

func main() {
	configFile := flag.String("config_file", "config.txt", "file containing all ip addresses")
	flag.Parse()

	configs := readConfigFile(*configFile)

	for _, config := range configs {
		ip := config[0]
		port := config[1]
		addr := strings.Join([]string{ip, port}, ":")
		SocketClient(addr)
	}
}

func readConfigFile(configFile string) [][]string {
	file, _ := os.Open(configFile)
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	return result
}

func Map(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}
