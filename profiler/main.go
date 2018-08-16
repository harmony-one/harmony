package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/simple-rules/harmony-benchmark/log"
)

type profilerSetting struct {
	pid     int32
	shardID string
}

var (
	setting profilerSetting
)

func logPerf() {
	p, _ := process.NewProcess(setting.pid)
	for {
		// log mem usage
		info, _ := p.MemoryInfo()
		memMap, _ := p.MemoryMaps(false)
		log.Info("Mem Report", "info", info, "map", memMap, "shardID", setting.shardID)

		// log cpu usage
		percent, _ := p.CPUPercent()
		times, _ := p.Times()
		log.Info("CPU Report", "percent", percent, "times", times, "shardID", setting.shardID)

		time.Sleep(3 * time.Second)
	}
}

func main() {
	pid := flag.Int("pid", 0, "process id of the node")
	shardID := flag.String("shard_id", "0", "the shard id of this node")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	flag.Parse()

	setting.pid = int32(*pid)
	setting.shardID = *shardID
	logFileName := fmt.Sprintf("./%v/profiler-%v.log", *logFolder, *shardID)
	h := log.MultiHandler(
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
		// log.Must.NetHandler("tcp", ":3000", log.JSONFormat()) // Log to remote
	)
	log.Root().SetHandler(h)

	logPerf()
}
