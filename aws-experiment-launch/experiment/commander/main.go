/*
Commander has two modes to setup configuration: Local and S3.

Local Config Mode

The Default Mode.

Add `-mode local` or omit `-mode` to enter local config mode. In this mode, the `commander` will host the config file `config.txt` on the commander machine and `solider`s will download the config file from `http://{commander_ip}:{commander_port}/distribution_config.txt`.

Remote Config Mode

Add `-mode remote` to enter remote config mode. In this mode, the `soldier`s will download the config file from a remote URL (use `-config_url {url}` to set the URL).
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/simple-rules/harmony-benchmark/aws-experiment-launch/experiment/utils"
	"github.com/simple-rules/harmony-benchmark/configr"
)

type commanderSetting struct {
	ip   string
	port string
	mode string
	// Options in s3 mode
	configURL string

	configr *configr.Configr
}

type sessionInfo struct {
	id           string
	uploadFolder string
}

var (
	setting commanderSetting
	session sessionInfo
)

const (
	DistributionFileName = "distribution_config.txt"
	DefaultConfigUrl     = "https://s3-us-west-2.amazonaws.com/unique-bucket-bin/distribution_config.txt"
)

func handleCommand(command string) {
	args := strings.Split(command, " ")
	if len(args) <= 0 {
		return
	}

	switch cmd := args[0]; cmd {
	case "config":
		if setting.mode == "s3" {
			// In s3 mode, download the config file from configURL first.
			if err := utils.DownloadFile(DistributionFileName, setting.configURL); err != nil {
				panic(err)
			}
		}

		err := setting.configr.ReadConfigFile(DistributionFileName)
		if err == nil {
			log.Printf("The loaded config has %v nodes\n", len(setting.configr.GetConfigEntries()))
		} else {
			log.Println("Failed to read config file")
		}
	case "init":
		session.id = time.Now().Format("150405-20060102")
		// create upload folder
		session.uploadFolder = fmt.Sprintf("upload/%s", session.id)
		err := os.MkdirAll(session.uploadFolder, os.ModePerm)
		if err != nil {
			log.Println("Failed to create upload folder", session.uploadFolder)
			return
		}
		log.Println("New session", session.id)

		dictateNodes(fmt.Sprintf("init %v %v %v %v", setting.ip, setting.port, setting.configURL, session.id))
	case "ping", "kill", "log", "log2":
		dictateNodes(command)
	default:
		log.Println("Unknown command")
	}
}

func config(ip string, port string, mode string, configURL string) {
	setting.ip = ip
	setting.port = port
	setting.mode = mode
	if mode == "local" {
		setting.configURL = fmt.Sprintf("http://%s:%s/%s", ip, port, DistributionFileName)
	} else {
		setting.configURL = configURL
	}
	setting.configr = configr.NewConfigr()
}

func dictateNodes(command string) {
	resultChan := make(chan int)
	configs := setting.configr.GetConfigEntries()
	for _, entry := range configs {
		port := "1" + entry.Port // the port number of solider is "1" + node port
		addr := strings.Join([]string{entry.IP, port}, ":")

		go func(resultChan chan int) {
			resultChan <- dictateNode(addr, command)
		}(resultChan)
	}
	count := len(configs)
	res := 0
	for ; count > 0; count-- {
		res += <-resultChan
	}

	log.Printf("Finished %s with %v nodes\n", command, res)
}

func dictateNode(addr string, command string) int {
	// creates client
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Println(err)
		return 0
	}
	defer conn.Close()

	// send command
	_, err = conn.Write([]byte(command))
	if err != nil {
		log.Printf("Failed to send command to %s", addr)
		return 0
	}
	// log.Printf("Send \"%s\" to %s", command, addr)

	// read response
	buff := make([]byte, 1024)
	if n, err := conn.Read(buff); err == nil {
		received := string(buff[:n])
		// log.Printf("Receive from %s: %s", addr, buff[:n])
		if strings.Contains(received, "Failed") {
			return 0
		} else {
			return 1
		}
	}
	return 0
}

func handleUploadRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// reject non-post requests
		jsonResponse(w, http.StatusBadRequest, "Only post request is accepted.")
		return
	}

	reader, err := r.MultipartReader()
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		dst, err := os.Create(fmt.Sprintf("%s/%s", session.uploadFolder, part.FileName()))
		log.Println(part.FileName())
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, part); err != nil {
			jsonResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func jsonResponse(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprint(w, message)
	log.Println(message)
}

func serve() {
	if setting.mode == "local" {
		// Only host config file if in local mode
		http.Handle("/", http.FileServer(http.Dir("./")))
	}
	http.HandleFunc("/upload", handleUploadRequest)
	err := http.ListenAndServe(":"+setting.port, nil)
	if err != nil {
		log.Fatalf("Failed to setup server! Error: %s", err.Error())
	}
	log.Printf("Start to host upload endpoint at http://%s:%s/upload\n", setting.ip, setting.port)
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "The ip of commander, i.e. this machine")
	port := flag.String("port", "8080", "The port which the commander uses to communicate with soldiers")
	mode := flag.String("mode", "local", "The config mode, local or s3")
	configURL := flag.String("config_url", DefaultConfigUrl, "The config URL")
	flag.Parse()

	config(*ip, *port, *mode, *configURL)

	go serve()

	scanner := bufio.NewScanner(os.Stdin)
	for true {
		log.Printf("Listening to Your Command:")
		if !scanner.Scan() {
			break
		}
		handleCommand(scanner.Text())
	}
}
