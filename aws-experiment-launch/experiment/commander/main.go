package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type commanderSetting struct {
	ip         string
	port       string
	configFile string
	configs    [][]string
	sessionID  string
}

var (
	setting commanderSetting
)

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
	if len(args) <= 0 {
		return
	}

	switch cmd := args[0]; cmd {
	case "init":
		{
			setting.sessionID = time.Now().Format("20060102-150405")
			log.Println("New session", setting.sessionID)
			dictateNodes(fmt.Sprintf("init %v %v %v %v", setting.ip, setting.port, setting.configFile, setting.sessionID))
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

func config(ip string, port string, configFile string) {
	setting.ip = ip
	setting.port = port
	setting.configFile = configFile
	setting.configs = readConfigFile()
	log.Println("Loaded config file", setting.configs)
}

func dictateNodes(command string) {
	for _, config := range setting.configs {
		ip := config[0]
		port := "1" + config[1] // the port number of solider is "1" + node port
		addr := strings.Join([]string{ip, port}, ":")

		go dictateNode(addr, command)
	}
}

func dictateNode(addr string, command string) {
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

	// create upload folder
	uploadFolder := fmt.Sprintf("upload/%s", setting.sessionID)
	err := os.MkdirAll(uploadFolder, os.ModePerm)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, err.Error())
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

		dst, err := os.Create(fmt.Sprintf("%s/%s", uploadFolder, part.FileName()))
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

func saveFile(w http.ResponseWriter, file multipart.File, handle *multipart.FileHeader) {
	data, err := ioutil.ReadAll(file)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = ioutil.WriteFile("./files/"+handle.Filename, data, 0666)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, "File uploaded successfully!")
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
	port := flag.String("port", "8080", "The port where you want to host the config file")
	configFile := flag.String("config_file", "test.txt", "The file name of config file which should be put in the same of folder as commander")
	flag.Parse()

	config(*ip, *port, *configFile)

	log.Println("Start to host config file at http://" + setting.ip + ":" + setting.port + "/" + setting.configFile)

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
