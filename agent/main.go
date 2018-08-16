package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/simple-rules/harmony-benchmark/configr"
	"github.com/simple-rules/harmony-benchmark/log"
)

var (
	shardID string
	ip      string
	port    string
	pid     int32
)

func logMemUsage() {
	p, _ := process.NewProcess(pid)
	for {
		info, _ := p.MemoryInfo()
		memMap, _ := p.MemoryMaps(false)
		log.Info("Mem Report", "info", info, "map", memMap, "shardID", shardID)
		time.Sleep(3 * time.Second)
	}
}

func logCPUUsage() {
	p, _ := process.NewProcess(pid)
	for {
		percent, _ := p.CPUPercent()
		times, _ := p.Times()
		log.Info("CPU Report", "percent", percent, "times", times, "shardID", shardID)
		time.Sleep(3 * time.Second)
	}
}

func main() {
	_ip := flag.String("ip", "127.0.0.1", "IP of the node")
	_port := flag.String("port", "9000", "port of the node.")
	_pid := flag.Int("pid", 0, "process id of the node")
	configFile := flag.String("config_file", "config.txt", "file containing all ip addresses")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	flag.Parse()

	ip = *_ip
	port = *_port
	pid = int32(*_pid)
	config, _ := configr.ReadConfigFile(*configFile)
	shardID := configr.GetShardID(ip, port, &config)
	leader := configr.GetLeader(shardID, &config)

	var role string
	if leader.Ip == ip && leader.Port == port {
		role = "leader"
	} else {
		role = "validator"
	}
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/agent-%s-%v-%v.log", *logFolder, role, ip, port)

	h := log.MultiHandler(
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
		// log.Must.NetHandler("tcp", ":3000", log.JSONFormat()) // Log to remote
	)
	log.Root().SetHandler(h)

	go logMemUsage()
	logCPUUsage()
}
