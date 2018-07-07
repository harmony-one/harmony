package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

type CommanderSetting struct {
	addr       string
	configFile string
	configs    [][]string
}

var (
	setting CommanderSetting
)

func socketClient(addr string, handler func(net.Conn, string)) {

}

func readConfigFile() [][]string {
	file, _ := os.Open(setting.configFile)
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	return result
}

func handleCommand(command string) {
	args := strings.Split(command, " ")

	if len(args) <= 0 {
		return
	}

	switch cmd := args[0]; cmd {
	case "config":
		{
			handleConfigCommand(args[1], args[2])
		}
	case "init":
		{
			dictateNodes("init" + setting.addr + setting.configFile)
		}
	default:
		{
			dictateNodes(command)
		}
	}
}

func handleConfigCommand(addr string, configFile string) {
	setting.addr = addr
	setting.configFile = configFile
	setting.configs = readConfigFile()
}

func dictateNodes(command string) {
	for _, config := range setting.configs {
		ip := config[0]
		port := "1" + config[1] // the port number of solider is "1" + node port
		addr := strings.Join([]string{ip, port}, ":")

		// creates client
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		// send command
		_, err = conn.Write([]byte(command))
		if err != nil {
			log.Printf("Failed to send command to %s", addr)
			return
		}
		log.Printf("Send: %s", command)

		// read response
		buff := make([]byte, 1024)
		n, _ := conn.Read(buff)
		log.Printf("Receive from %s: %s", addr, buff[:n])
	}
}

func main() {
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Listening to Your Command:")
		command, _ := reader.ReadString('\n')
		handleCommand(command)
	}
}
