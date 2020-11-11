// Package prometheus defines a service which is used for metrics collection
// and health of a node in Harmony.
package prometheus

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"runtime/pprof"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Service provides Prometheus metrics via the /metrics route. This route will
// show all the metrics registered with the Prometheus DefaultRegisterer.
type Service struct {
	server     *http.Server
	failStatus error
}

// Handler represents a path and handler func to serve on the same port as /metrics, /healthz, /goroutinez, etc.
type Handler struct {
	Path    string
	Handler func(http.ResponseWriter, *http.Request)
}

var (
	svc = &Service{}
)

// NewService sets up a new instance for a given address host:port.
// An empty host will match with any IP so an address like ":19000" is perfectly acceptable.
func NewService(config nodeconfig.PrometheusServerConfig, additionalHandlers ...Handler) {

	if !config.HTTPEnabled {
		getLogger().Info().Msg("Prometheus http server disabled...")
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/goroutinez", svc.goroutinezHandler)

	// Register additional handlers.
	for _, h := range additionalHandlers {
		mux.HandleFunc(h.Path, h.Handler)
	}

	getLogger().Info().Int("port", config.HTTPPort).
		Str("ip", config.HTTPIp).
		Msg("Starting Prometheus server")
	endpoint := fmt.Sprintf("%s:%d", config.HTTPIp, config.HTTPPort)
	svc.server = &http.Server{Addr: endpoint, Handler: mux}

	svc.Start()
}

// StopService stop the Prometheus service
func StopService() error {
	return svc.Stop()
}

func (s *Service) goroutinezHandler(w http.ResponseWriter, _ *http.Request) {
	stack := debug.Stack()
	if _, err := w.Write(stack); err != nil {
		getLogger().Error().Err(err).Msg("Failed to write goroutines stack")
	}
	if err := pprof.Lookup("goroutine").WriteTo(w, 2); err != nil {
		getLogger().Error().Err(err).Msg("Failed to write pprof goroutines")
	}
}

// Start the prometheus service.
func (s *Service) Start() {
	go func() {
		// See if the port is already used.
		conn, err := net.DialTimeout("tcp", s.server.Addr, time.Second)
		if err == nil {
			if err := conn.Close(); err != nil {
				getLogger().Error().Err(err).Msg("Failed to close connection")
			}
			// Something on the port; we cannot use it.
			getLogger().Warn().Str("address", s.server.Addr).Msg("Port already in use; cannot start prometheus service")
		} else {
			// Nothing on that port; we can use it.
			getLogger().Debug().Str("address", s.server.Addr).Msg("Starting prometheus service")
			err := s.server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				getLogger().Error().Msgf("Could not listen to host:port :%s: %v", s.server.Addr, err)
				s.failStatus = err
			}
		}
	}()
}

// Stop the service gracefully.
func (s *Service) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Status checks for any service failure conditions.
func (s *Service) Status() error {
	if s.failStatus != nil {
		return s.failStatus
	}
	return nil
}

func getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Str("context", "prometheus").
		Logger()
	return &logger
}
