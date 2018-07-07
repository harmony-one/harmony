package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"strings"
)

type commanderSetting struct {
	ip         string
	port       string
	configFile string
	configs    [][]string
}

var (
	setting commanderSetting
)

func socketClient(addr string, handler func(net.Conn, string)) {

}

func readConfigFile() [][]string {
	file, err := os.Open(setting.configFile)
	if err != nil {
		log.Println("Failed to read config file")
		return nil
	}
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
	log.Println(args)
	if len(args) <= 0 {
		return
	}

	switch cmd := args[0]; cmd {
	case "init":
		{
			dictateNodes("init http://" + setting.ip + ":" + setting.port + "/" + setting.configFile)
		}
	default:
		{
			dictateNodes(command)
		}
	}
}

func config(ip string, port string, configFile string) {
	setting.ip = ip
	setting.port = port
	setting.configFile = configFile
	setting.configs = readConfigFile()
	log.Println("Config-ed", setting.configs)
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
	ip := flag.String("ip", "127.0.0.1", "ip of commander")
	port := flag.String("port", "8080", "port of config file")
	configFile := flag.String("config_file", "test.txt", "file name of config file")

	config(*ip, *port, *configFile)

	scanner := bufio.NewScanner(os.Stdin)
	for true {
		log.Printf("Listening to Your Command:")
		if !scanner.Scan() {
			break
		}
		handleCommand(scanner.Text())
	}
}
