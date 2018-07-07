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

type soliderSetting struct {
	ip          string
	port        string
	localConfig string
}

var (
	setting soliderSetting
)

func socketServer() {
	soldierPort := "1" + setting.port // the soldier port is "1" + node port
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
	case "kill":
		{
			handleKillCommand(w)
		}
	}
}

func handleInitCommand(args []string, w *bufio.Writer) {
	log.Println("Init command", args)
	// create local config file
	setting.localConfig = "node_config_" + setting.port + ".txt"
	out, err := os.Create(setting.localConfig)
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
	content, err := ioutil.ReadFile(setting.localConfig)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully downloaded config")
	log.Println(string(content))

	run()

	logAndReply(w, "Successfully init-ed")
}

func handleKillCommand(w *bufio.Writer) {
	log.Println("Kill command")
	runCmd("../kill_node.sh")
	logAndReply(w, "Kill command done.")
}

func logAndReply(w *bufio.Writer, message string) {
	log.Println(message)
	w.Write([]byte(message))
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

func runCmd(name string, args ...string) error {
	err := exec.Command(name, args...).Start()
	log.Println("Command running", name, args)
	return err
}

func run() {
	config := readConfigFile(setting.localConfig)

	myConfig := getMyConfig(setting.ip, setting.port, &config)

	logFolder := createLogFolder()
	if myConfig[2] == "client" {
		runClient(logFolder)
	} else {
		runInstance(logFolder)
	}
}

func runInstance(logFolder string) error {
	log.Println("running instance")
	return runCmd("./benchmark", "-ip", setting.ip, "-port", setting.port, "-config_file", setting.localConfig, "-log_folder", logFolder)
}

func runClient(logFolder string) error {
	log.Println("running client")
	return runCmd("./txgen", "-config_file", setting.localConfig, "-log_folder", logFolder)
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
	ip := flag.String("ip", "127.0.0.1", "IP of the node.")
	port := flag.String("port", "3000", "port of the node.")
	flag.Parse()

	setting.ip = *ip
	setting.port = *port

	socketServer()
}
