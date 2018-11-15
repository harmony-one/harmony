package profiler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/harmony-one/harmony/log"
)

type Profiler struct {
	// parameters
	logger           log.Logger
	pid              int32
	shardID          string
	MetricsReportURL string
	// Internal
	proc *process.Process
}

var singleton *Profiler
var once sync.Once

func GetProfiler() *Profiler {
	once.Do(func() {
		singleton = &Profiler{}
	})
	return singleton
}

func (profiler *Profiler) Config(logger log.Logger, shardID string, metricsReportURL string) {
	profiler.logger = logger
	profiler.pid = int32(os.Getpid())
	profiler.shardID = shardID
	profiler.MetricsReportURL = metricsReportURL
}

func (profiler *Profiler) LogMemory() {
	for {
		// log mem usage
		info, _ := profiler.proc.MemoryInfo()
		memMap, _ := profiler.proc.MemoryMaps(false)
		profiler.logger.Info("Mem Report", "info", info, "map", memMap, "shardID", profiler.shardID)

		time.Sleep(3 * time.Second)
	}
}

func (profiler *Profiler) LogCPU() {
	for {
		// log cpu usage
		percent, _ := profiler.proc.CPUPercent()
		times, _ := profiler.proc.Times()
		profiler.logger.Info("CPU Report", "percent", percent, "times", times, "shardID", profiler.shardID)

		time.Sleep(3 * time.Second)
	}
}

func (profiler *Profiler) LogMetrics(metrics map[string]interface{}) {
	jsonValue, _ := json.Marshal(metrics)
	rsp, err := http.Post(profiler.MetricsReportURL, "application/json", bytes.NewBuffer(jsonValue))
	if err == nil {
		defer rsp.Body.Close()
	}
}

func (profiler *Profiler) Start() {
	profiler.proc, _ = process.NewProcess(profiler.pid)
	go profiler.LogCPU()
	go profiler.LogMemory()
}
