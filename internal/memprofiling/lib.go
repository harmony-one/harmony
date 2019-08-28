package memprofiling

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/fjl/memsize"
	"github.com/fjl/memsize/memsizeui"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for mem profiling.
const (
	MemProfilingPortDiff = 1000
	// Constants of for scanning mem size.
	memSizeScanTime = 30 * time.Second
	// Run garbage collector every 30 minutes.
	gcTime = 10 * time.Minute
	// Print out memstat every memStatTime.
	memStatTime = 300 * time.Second
)

// MemProfiling is the struct to watch objects for memprofiling.
type MemProfiling struct {
	h              *memsizeui.Handler
	s              *http.Server
	observedObject map[string]interface{}
	mu             sync.Mutex
}

// New returns MemProfiling object.
func New() *MemProfiling {
	return &MemProfiling{
		observedObject: map[string]interface{}{},
		h:              new(memsizeui.Handler),
	}
}

// Config configures mem profiling.
func (m *MemProfiling) Config() {
	m.s = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", nodeconfig.GetDefaultConfig().IP, utils.GetPortFromDiff(nodeconfig.GetDefaultConfig().Port, MemProfilingPortDiff)),
		Handler: m.h,
	}
	utils.Logger().Info().
		Str("port", utils.GetPortFromDiff(nodeconfig.GetDefaultConfig().Port, MemProfilingPortDiff)).
		Msgf("running mem profiling")
}

// Add adds variables to watch for profiling.
func (m *MemProfiling) Add(name string, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v != nil {
		rv := reflect.ValueOf(v)
		if !(rv.Kind() != reflect.Ptr || rv.IsNil()) {
			m.h.Add(name, v)
			m.observedObject[name] = v
		}
	}
}

// Start starts profiling server.
func (m *MemProfiling) Start() {
	go m.s.ListenAndServe()
	m.PeriodicallyScanMemSize()
	utils.Logger().Info().Msg("Start memprofiling.")
}

// Stop stops mem profiling.
func (m *MemProfiling) Stop() {
	m.s.Shutdown(nil)
}

// PeriodicallyScanMemSize scans memsize of the observed objects every 30 seconds.
func (m *MemProfiling) PeriodicallyScanMemSize() {
	go func() {
		for {
			select {
			case <-time.After(memSizeScanTime):
				m.mu.Lock()
				m := GetMemProfiling()
				for k, v := range m.observedObject {
					s := memsize.Scan(v)
					r := s.Report()
					utils.Logger().Info().Msgf("memsize report for %s:\n %s", k, r)
				}
				m.mu.Unlock()
			}
		}
	}()
}

// MaybeCallGCPeriodically runs GC manually every gcTime minutes. This is one of the options to mitigate the OOM issue.
func MaybeCallGCPeriodically() {
	go func() {
		for {
			select {
			case <-time.After(gcTime):
				PrintMemUsage("Memory stats before GC")
				runtime.GC()
				PrintMemUsage("Memory stats after GC")
			}
		}
	}()
	go func() {
		for {
			select {
			case <-time.After(memStatTime):
				PrintMemUsage("Memory stats")
			}
		}
	}()
}

// PrintMemUsage prints memory usage.
func PrintMemUsage(msg string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	utils.Logger().Info().
		Uint64("alloc", bToMb(m.Alloc)).
		Uint64("totalalloc", bToMb(m.TotalAlloc)).
		Uint64("sys", bToMb(m.Sys)).
		Uint32("numgc", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
