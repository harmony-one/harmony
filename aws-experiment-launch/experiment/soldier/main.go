/*
Soldier is responsible for receiving commands from commander and doing tasks such as starting nodes, uploading logs.

   cd harmony-benchmark/bin
   go build -o soldier ../aws-experiment-launch/experiment/soldier/main.go
   ./soldier -ip={node_ip} -port={node_port}
*/
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/simple-rules/harmony-benchmark/aws-experiment-launch/experiment/soldier/s3"
	"github.com/simple-rules/harmony-benchmark/aws-experiment-launch/experiment/utils"
	"github.com/simple-rules/harmony-benchmark/configr"
	globalUtils "github.com/simple-rules/harmony-benchmark/utils"
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
	configr             *configr.Configr
	myConfig            configr.ConfigEntry
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

	globalSession.configr.ReadConfigFile(globalSession.localConfigFileName)
	globalSession.myConfig = *globalSession.configr.GetMyConfigEntry(setting.ip, setting.port)

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
		return globalUtils.RunCmd("Stop-Process", "-Id", command)
	} else {
		command := fmt.Sprintf("lsof -i tcp:%s | grep LISTEN | awk '{print $2}' | xargs kill -9", port)
		return globalUtils.RunCmd("bash", "-c", command)
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
		logAndReply(w, fmt.Sprintf("Failed to read log folder. Error: %s", err.Error()))
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

func runInstance() error {
	os.MkdirAll(globalSession.logFolder, os.ModePerm)

	if globalSession.myConfig.Role == "client" {
		return runClient()
	}
	return runNode()
}

func runNode() error {
	log.Println("running instance")
	return globalUtils.RunCmd("./benchmark", "-ip", setting.ip, "-port", setting.port, "-config_file", globalSession.localConfigFileName, "-log_folder", globalSession.logFolder)
}

func runClient() error {
	log.Println("running client")
	return globalUtils.RunCmd("./txgen", "-config_file", globalSession.localConfigFileName, "-log_folder", globalSession.logFolder)
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node.")
	port := flag.String("port", "9000", "port of the node.")
	flag.Parse()

	setting.ip = *ip
	setting.port = *port

	socketServer()
}
