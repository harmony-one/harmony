package profiler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/shirou/gopsutil/process"
)

// Profiler is the profiler data structure.
type Profiler struct {
	pid              int32
	shardID          uint32
	MetricsReportURL string
	// Internal
	proc *process.Process
}

var singleton *Profiler
var once sync.Once

// GetProfiler returns a pointer of Profiler.
// TODO: This should be a New method.
func GetProfiler() *Profiler {
	once.Do(func() {
		singleton = &Profiler{}
	})
	return singleton
}

// Config configurates Profiler.
func (profiler *Profiler) Config(shardID uint32, metricsReportURL string) {
	profiler.pid = int32(os.Getpid())
	profiler.shardID = shardID
	profiler.MetricsReportURL = metricsReportURL
}

// LogMemory logs memory.
func (profiler *Profiler) LogMemory() {
	for {
		// log mem usage
		info, _ := profiler.proc.MemoryInfo()
		memMap, _ := profiler.proc.MemoryMaps(false)
		loggedMemMap := ""
		for _, mems := range *memMap {
			loggedMemMap += mems.String() + "; "
		}
		utils.Logger().Info().
			Str("info", info.String()).
			Str("map", loggedMemMap).
			Uint32("shardID", profiler.shardID).
			Msg("Mem Report")

		time.Sleep(3 * time.Second)
	}
}

// LogCPU logs CPU metrics.
func (profiler *Profiler) LogCPU() {
	for {
		// log cpu usage
		percent, _ := profiler.proc.CPUPercent()
		times, _ := profiler.proc.Times()
		utils.Logger().Info().
			Float64("percent", percent).
			Str("times", times.String()).
			Uint32("shardID", profiler.shardID).
			Msg("CPU Report")

		time.Sleep(3 * time.Second)
	}
}

// LogMetrics logs metrics.
func (profiler *Profiler) LogMetrics(metrics map[string]interface{}) {
	jsonValue, _ := json.Marshal(metrics)
	rsp, err := http.Post(profiler.MetricsReportURL, "application/json", bytes.NewBuffer(jsonValue))
	if err == nil {
		defer rsp.Body.Close()
	}
}

// Start starts profiling.
func (profiler *Profiler) Start() {
	profiler.proc, _ = process.NewProcess(profiler.pid)
	go profiler.LogCPU()
	go profiler.LogMemory()
}
