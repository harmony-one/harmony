package profiler

import (
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/simple-rules/harmony-benchmark/log"
)

type Profiler struct {
	logger  log.Logger
	PID     int32
	ShardID string
	proc    *process.Process
}

func NewProfiler(logger log.Logger, pid int, shardID string) *Profiler {
	profiler := Profiler{logger, int32(pid), shardID, nil}
	return &profiler
}

func (profiler *Profiler) LogMemory() {
	for {
		// log mem usage
		info, _ := profiler.proc.MemoryInfo()
		memMap, _ := profiler.proc.MemoryMaps(false)
		profiler.logger.Info("Mem Report", "info", info, "map", memMap, "shardID", profiler.ShardID)

		time.Sleep(3 * time.Second)
	}
}

func (profiler *Profiler) LogCPU() {
	for {
		// log cpu usage
		percent, _ := profiler.proc.CPUPercent()
		times, _ := profiler.proc.Times()
		profiler.logger.Info("CPU Report", "percent", percent, "times", times, "shardID", profiler.ShardID)

		time.Sleep(3 * time.Second)
	}
}

func (profiler *Profiler) Start() {
	profiler.proc, _ = process.NewProcess(profiler.PID)
	go profiler.LogCPU()
	go profiler.LogMemory()
}
