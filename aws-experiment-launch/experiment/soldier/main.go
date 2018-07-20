package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"harmony-benchmark/aws-experiment-launch/experiment/soldier/s3"
	"harmony-benchmark/aws-experiment-launch/experiment/utils"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type soliderSetting struct {
	ip   string
	port string
}

type sessionInfo struct {
	id                  string
	commanderIP         string
	commanderPort       string
	localConfigFileName string
	logFolder           string
}

const (
	bucketName      = "richard-bucket-test"
	logFolderPrefix = "../tmp_log/"
)

var (
	setting       soliderSetting
	globalSession sessionInfo
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
	case "ping":
		{
			handlePingCommand(w)
		}
	case "init":
		{
			handleInitCommand(args[1:], w)
		}
	case "kill":
		{
			handleKillCommand(w)
		}
	case "log":
		{
			handleLogCommand(w)
		}
	case "log2":
		{
			handleLog2Command(w)
		}
	}
}

func handleInitCommand(args []string, w *bufio.Writer) {
	// init ip port config_file sessionID
	log.Println("Init command", args)

	// read arguments
	ip := args[0]
	globalSession.commanderIP = ip
	port := args[1]
	globalSession.commanderPort = port
	configURL := args[2]
	sessionID := args[3]
	globalSession.id = sessionID
	globalSession.logFolder = fmt.Sprintf("%slog-%v", logFolderPrefix, sessionID)

	// create local config file
	globalSession.localConfigFileName = fmt.Sprintf("node_config_%v_%v.txt", setting.port, globalSession.id)
	utils.DownloadFile(globalSession.localConfigFileName, configURL)
	log.Println("Successfully downloaded config")

	if err := runInstance(); err == nil {
		logAndReply(w, "Done init.")
	} else {
		logAndReply(w, "Failed.")
	}
}

func handleKillCommand(w *bufio.Writer) {
	log.Println("Kill command")
	if err := killPort(setting.port); err == nil {
		logAndReply(w, "Done kill.")
	} else {
		logAndReply(w, "Failed.")
	}
}

func killPort(port string) error {
	if runtime.GOOS == "windows" {
		command := fmt.Sprintf("(Get-NetTCPConnection -LocalPort %s).OwningProcess -Force", port)
		return runCmd("Stop-Process", "-Id", command)
	} else {
		command := fmt.Sprintf("lsof -i tcp:%s | grep LISTEN | awk '{print $2}' | xargs kill -9", port)
		return runCmd("bash", "-c", command)
	}
}

func handlePingCommand(w *bufio.Writer) {
	log.Println("Ping command")
	logAndReply(w, "I'm alive")
}

func handleLogCommand(w *bufio.Writer) {
	log.Println("Log command")

	files, err := ioutil.ReadDir(globalSession.logFolder)
	if err != nil {
		logAndReply(w, fmt.Sprintf("Failed to create log folder. Error: %s", err.Error()))
		return
	}

	filePaths := make([]string, len(files))
	for i, f := range files {
		filePaths[i] = fmt.Sprintf("%s/%s", globalSession.logFolder, f.Name())
	}

	req, err := newUploadFileRequest(
		fmt.Sprintf("http://%s:%s/upload", globalSession.commanderIP, globalSession.commanderPort),
		"file",
		filePaths,
		nil)
	if err != nil {
		logAndReply(w, fmt.Sprintf("Failed to create upload request. Error: %s", err.Error()))
		return
	}
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		logAndReply(w, fmt.Sprintf("Failed to upload log. Error: %s", err.Error()))
		return
	}
	logAndReply(w, "Upload log done!")
}

// Creates a new file upload http request with optional extra params
func newUploadFileRequest(uri string, paramName string, paths []string, params map[string]string) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		part, err := writer.CreateFormFile(paramName, filepath.Base(path))
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(part, file)
		log.Printf(path)
	}

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err := writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

func logAndReply(w *bufio.Writer, message string) {
	log.Println(message)
	w.Write([]byte(message))
	w.Flush()
}

func handleLog2Command(w *bufio.Writer) {
	log.Println("Log command")

	files, err := ioutil.ReadDir(globalSession.logFolder)
	if err != nil {
		logAndReply(w, fmt.Sprintf("Failed to create log folder. Error: %s", err.Error()))
		return
	}

	filePaths := make([]string, len(files))
	for i, f := range files {
		filePaths[i] = fmt.Sprintf("%s/%s", globalSession.logFolder, f.Name())
	}

	// TODO: currently only upload the first file.
	_, err = s3.UploadFile(bucketName, filePaths[0], strings.Replace(filePaths[0], logFolderPrefix, "", 1))
	if err != nil {
		logAndReply(w, fmt.Sprintf("Failed to create upload request. Error: %s", err.Error()))
		return
	}
	logAndReply(w, "Upload log done!")
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		return err
	} else {
		log.Println("Command running", name, args)
		go func() {
			if err = cmd.Wait(); err != nil {
				log.Printf("Command finished with error: %v", err)
			} else {
				log.Printf("Command finished successfully")
			}
		}()
		return nil
	}
}

func runInstance() error {
	config := readConfigFile(globalSession.localConfigFileName)

	myConfig := getMyConfig(setting.ip, setting.port, &config)

	os.MkdirAll(globalSession.logFolder, os.ModePerm)

	if myConfig[2] == "client" {
		return runClient()
	} else {
		return runNode()
	}
}

func runNode() error {
	log.Println("running instance")
	return runCmd("./benchmark", "-ip", setting.ip, "-port", setting.port, "-config_file", globalSession.localConfigFileName, "-log_folder", globalSession.logFolder)
}

func runClient() error {
	log.Println("running client")
	return runCmd("./txgen", "-config_file", globalSession.localConfigFileName, "-log_folder", globalSession.logFolder)
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

// cd harmony-benchmark
// go build -o soldier ../aws-experiment-launch/experiment/soldier/main.go
// ./soldier -ip=xx -port=xx
func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node.")
	port := flag.String("port", "9000", "port of the node.")
	flag.Parse()

	setting.ip = *ip
	setting.port = *port

	socketServer()
}
