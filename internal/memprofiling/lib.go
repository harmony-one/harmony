package memprofiling

import (
	"fmt"
	"net/http"

	"github.com/fjl/memsize/memsizeui"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for mem profiling.
const (
	MemProfilingPortDiff = 1000
)

// MemProfiling is the struct of MemProfiling.
type MemProfiling struct {
	h *memsizeui.Handler
	s *http.Server
}

// New returns MemProfiling object.
func New() *MemProfiling {
	return &MemProfiling{}
}

// Config configures mem profiling.
func (m *MemProfiling) Config() {
	m.h = new(memsizeui.Handler)
	m.s = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", nodeconfig.GetDefaultConfig().IP, utils.GetPortFromDiff(nodeconfig.GetDefaultConfig().Port, MemProfilingPortDiff)),
		Handler: m.h,
	}
	utils.GetLogInstance().Info("running mem profiling", "port", utils.GetPortFromDiff(nodeconfig.GetDefaultConfig().Port, MemProfilingPortDiff))
}

// Add adds variables to watch for profiling.
func (m *MemProfiling) Add(name string, v interface{}) {
	m.h.Add(name, v)
}

// Start starts profiling server.
func (m *MemProfiling) Start() {
	go m.s.ListenAndServe()
	utils.GetLogInstance().Info("Start memprofiling.")
}

// Stop stops mem profiling.
func (m *MemProfiling) Stop() {
	m.s.Shutdown(nil)
}
