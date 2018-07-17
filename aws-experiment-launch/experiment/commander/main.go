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
	"sync"
	"time"
)

type commanderSetting struct {
	ip         string
	port       string
	configFile string
	configURL  string
	configs    [][]string
}

type sessionInfo struct {
	id           string
	uploadFolder string
}

var (
	setting commanderSetting
	session sessionInfo
)

func readConfigFile() [][]string {
	file, err := os.Open(setting.configFile)
	defer file.Close()
	if err != nil {
		log.Fatal("Failed to read config file ", setting.configFile,
			"\nNOTE: The config path should be relative to commander.")
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
	if len(args) <= 0 {
		return
	}

	switch cmd := args[0]; cmd {
	case "config":
		{
			setting.configs = readConfigFile()
			log.Println("Loaded config file", setting.configs)
		}
	case "init":
		{
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
		}
	case "ping", "kill", "log":
		{
			dictateNodes(command)
		}
	default:
		{
			log.Println("Unknown command")
		}
	}
}

func config(ip string, port string, configFile string, configURL string) {
	setting.ip = ip
	setting.port = port
	setting.configFile = configFile
	setting.configURL = configURL
}

func dictateNodes(command string) {
	const MaxFileOpen = 5000
	var wg sync.WaitGroup
	count := MaxFileOpen
	for _, config := range setting.configs {
		ip := config[0]
		port := "1" + config[1] // the port number of solider is "1" + node port
		addr := strings.Join([]string{ip, port}, ":")

		if count == MaxFileOpen {
			wg.Add(MaxFileOpen)
		}
		go func() {
			defer wg.Done()
			dictateNode(addr, command)
		}()
		count -= 1
		// Because of the limitation of ulimit
		if count == 0 {
			wg.Wait()
			time.Sleep(time.Second)
			count = MaxFileOpen
		}
	}
}

func dictateNode(addr string, command string) {
	// creates client
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
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
	log.Printf("Send \"%s\" to %s", command, addr)

	// read response
	buff := make([]byte, 1024)
	n, _ := conn.Read(buff)
	log.Printf("Receive from %s: %s", addr, buff[:n])
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
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.HandleFunc("/upload", handleUploadRequest)
	err := http.ListenAndServe(":"+setting.port, nil)
	if err != nil {
		log.Fatalf("Failed to setup server! Error: %s", err.Error())
	}
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "The ip of commander, i.e. this machine")
	port := flag.String("port", "8080", "The port which the commander uses to communicate with soldiers")
	configFile := flag.String("config_file", "distribution_config.txt", "The file name of config file")
	configURL := flag.String("config_url", "http://unique-bucket-bin.amazonaws.com/distribution_config.txt", "The config URL")
	flag.Parse()

	config(*ip, *port, *configFile, *configURL)

	log.Println("Start to host config files at http://" + setting.ip + ":" + setting.port)

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
