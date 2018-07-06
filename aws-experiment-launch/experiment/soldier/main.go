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
	"os/exec"
	"strings"
	"time"
)

const (
	StopCharacter = "\r\n\r\n"
)

var (
	ip          *string
	port        *string
	localConfig string
)

func socketServer() {
	soldierPort := "1" + *port // the soldier port is "1" + node port
	listen, err := net.Listen("tcp4", ":"+soldierPort)
	if err != nil {
		log.Fatalf("Socket listen port %s failed,%s", soldierPort, err)
		os.Exit(1)
	}
	defer listen.Close()
	log.Printf("Begin listen for command on port: %s", soldierPort)

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
			log.Println("Received command", data)
			if isTransportOver(data) {
				log.Println("Tranport Over!")
				break ILOOP
			}

			handleCommand(data, w)

			log.Println("Waiting for new command...")

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
	localConfig = "node_config_" + *port + ".txt"
	out, err := os.Create(localConfig)
	if err != nil {
		log.Fatal("Failed to create local file", err)
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

	// log config file
	content, err := ioutil.ReadFile(localConfig)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully downloaded config")
	log.Println(string(content))

	run()

	log.Println("Successfully init-ed")
	w.Write([]byte("Successfully init-ed"))
	w.Flush()
}

func createLogFolder() string {
	t := time.Now().Format("20060102-150405")
	logFolder := "../tmp_log/log-" + t
	err := os.MkdirAll(logFolder, os.ModePerm)
	if err != nil {
		log.Fatal("Failed to create log folder")
	}
	log.Println("Created log folder", logFolder)
	return logFolder
}

func runCmd(name string, args []string) {
	log.Println(name, args)
	err := exec.Command(name, args...).Start()
	if err != nil {
		log.Fatal("Failed to run command: ", err)
	}

	log.Println("Command running")
}

func run() {
	config := readConfigFile(localConfig)

	myConfig := getMyConfig(*ip, *port, &config)

	log.Println(myConfig)
	if myConfig[2] == "client" {
		runClient()
	} else {
		runInstance()
	}
}

func runInstance() {
	log.Println("running instance")
	logFolder := createLogFolder()
	runCmd("./benchmark", []string{"-ip", *ip, "-port", *port, "-config_file", localConfig, "-log_folder", logFolder})
}

func runClient() {
	log.Println("running client")
	logFolder := createLogFolder()
	runCmd("./txgen", []string{"-config_file", localConfig, "-log_folder", logFolder})
}

func isTransportOver(data string) (over bool) {
	over = strings.HasSuffix(data, "\r\n\r\n")
	return
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

func getMyConfig(myIP string, myPort string, config *[][]string) []string {
	for _, node := range *config {
		ip, port := node[0], node[1]
		if ip == myIP && port == myPort {
			return node
		}
	}
	return nil
}

// go build -o bin/soldier aws-experiment-launch/experiment/soldier/main.go
// cd bin/
// ./soldier --port=xxxx
func main() {
	ip = flag.String("ip", "127.0.0.1", "IP of the node.")
	port = flag.String("port", "3000", "port of the node.")
	flag.Parse()

	socketServer()
}
