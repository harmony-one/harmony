// Package prometheus defines a service which is used for metrics collection
// and health of a node in Harmony.
package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"runtime/pprof"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusConfig is the config for the prometheus service
type PrometheusConfig struct {
	Enabled  bool
	IP       string
	Port     int
	Gateway  string // address of the pushgateway
	Network  string // network type, used as job prefix
	Shard    uint32 // shard id, used as job suffix
	Instance string //identifier of the instance in prometheus metrics
}

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
	svc    = &Service{}
	config = PrometheusConfig{}
)

// NewService sets up a new instance for a given address host:port.
// An empty host will match with any IP so an address like ":19000" is perfectly acceptable.
func NewService(additionalHandlers ...Handler) {
	if !config.Enabled {
		utils.Logger().Info().Msg("Prometheus http server disabled...")
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/goroutinez", svc.goroutinezHandler)

	// Register additional handlers.
	for _, h := range additionalHandlers {
		mux.HandleFunc(h.Path, h.Handler)
	}

	utils.Logger().Debug().Int("port", config.Port).
		Str("ip", config.IP).
		Msg("Starting Prometheus server")
	endpoint := fmt.Sprintf("%s:%d", config.IP, config.Port)
	svc.server = &http.Server{Addr: endpoint, Handler: mux}

	// start pusher to push metrics to prometheus pushgateway
	// every minute
	go func(config PrometheusConfig) {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := PromPusher(config).Add(); err != nil {
					utils.Logger().Warn().Err(err).Msg("Pushgateway Error")
				}
			}
		}
	}(config)
	svc.Start()
}

// StopService stop the Prometheus service
func StopService() error {
	return svc.Stop()
}

func (s *Service) goroutinezHandler(w http.ResponseWriter, _ *http.Request) {
	stack := debug.Stack()
	if _, err := w.Write(stack); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to write goroutines stack")
	}
	if err := pprof.Lookup("goroutine").WriteTo(w, 2); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to write pprof goroutines")
	}
}

// Start the prometheus service.
func (s *Service) Start() {
	go func() {
		utils.Logger().Info().Str("address", s.server.Addr).Msg("Starting prometheus service")
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			utils.Logger().Error().Msgf("Could not listen to host:port :%s: %v", s.server.Addr, err)
			s.failStatus = err
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

// SetConfig initialize the prometheus config
func SetConfig(
	enabled bool,
	ip string,
	port int,
	gateway string,
	network string,
	shard uint32,
	instance string,
) {
	config.Enabled = enabled
	config.IP = ip
	config.Port = port
	config.Gateway = gateway
	config.Network = network
	config.Shard = shard
	config.Instance = instance
}

// GetConfig return the prometheus config
func GetConfig() PrometheusConfig {
	return config
}
