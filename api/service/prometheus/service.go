// Package prometheus defines a service which is used for metrics collection
// and health of a node in Harmony.
package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

// Config is the config for the prometheus service
type Config struct {
	Enabled    bool
	IP         string
	Port       int
	EnablePush bool   // enable pushgateway support
	Gateway    string // address of the pushgateway
	Network    string // network type, used as job prefix
	Legacy     bool   // legacy or not, legacy is harmony internal node
	NodeType   string // node type, validator or exlorer node
	Shard      uint32 // shard id, used as job suffix
	Instance   string //identifier of the instance in prometheus metrics
}

func (p Config) String() string {
	return fmt.Sprintf("%v, %v:%v, %v/%v, %v/%v/%v/%v:%v", p.Enabled, p.IP, p.Port, p.EnablePush, p.Gateway, p.Network, p.Legacy, p.NodeType, p.Shard, p.Instance)
}

// Service provides Prometheus metrics via the /metrics route. This route will
// show all the metrics registered with the Prometheus DefaultRegisterer.
type Service struct {
	server     *http.Server
	registry   *prometheus.Registry
	pusher     *push.Pusher
	failStatus error
}

// Handler represents a path and handler func to serve on the same port as /metrics, /healthz, /goroutinez, etc.
type Handler struct {
	Path    string
	Handler func(http.ResponseWriter, *http.Request)
}

var (
	registryOnce sync.Once
	svc          = &Service{}
	config       Config
)

func getJobName(config Config) string {
	var node string

	// legacy nodes are harmony nodes: s0,s1,s2,s3
	if config.Legacy {
		node = "s"
	} else {
		if config.NodeType == "validator" {
			// regular validator nodes are: v0,v1,v2,v3
			node = "v"
		} else {
			// explorer nodes are: e0,e1,e2,e3
			node = "e"
		}
	}

	return fmt.Sprintf("%s/%s%d", config.Network, node, config.Shard)
}

// NewService sets up a new instance for a given address host:port.
// An empty host will match with any IP so an address like ":19000" is perfectly acceptable.
func NewService(additionalHandlers ...Handler) {
	if !config.Enabled {
		utils.Logger().Info().Msg("Prometheus http server disabled...")
		return
	}

	utils.Logger().Debug().Str("Config", config.String()).Msg("Prometheus")
	if svc.registry == nil {
		svc.registry = prometheus.NewRegistry()
	}
	handler := promhttp.InstrumentMetricHandler(
		svc.registry,
		promhttp.HandlerFor(svc.registry, promhttp.HandlerOpts{}),
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)
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

	if config.EnablePush {
		job := getJobName(config)
		utils.Logger().Info().Str("Job", job).Msg("Prometheus enabled pushgateway support ...")
		svc.pusher = push.New(config.Gateway, job).
			Gatherer(svc.registry).
			Grouping("instance", config.Instance)

		// start pusher to push metrics to prometheus pushgateway
		// every minute
		go func(config Config) {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := svc.pusher.Add(); err != nil {
						utils.Logger().Warn().Err(err).Msg("Pushgateway Error")
					}
				}
			}
		}(config)
	}
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
	enablepush bool,
	gateway string,
	network string,
	legacy bool,
	nodetype string,
	shard uint32,
	instance string,
) {
	config.Enabled = enabled
	config.IP = ip
	config.Port = port
	config.EnablePush = enablepush
	config.Gateway = gateway
	config.Network = network
	config.Legacy = legacy
	config.NodeType = nodetype
	config.Shard = shard
	config.Instance = instance
}

// GetConfig return the prometheus config
func GetConfig() Config {
	return config
}

// PromRegistry return the registry of prometheus service
func PromRegistry() *prometheus.Registry {
	registryOnce.Do(func() {
		if svc.registry == nil {
			svc.registry = prometheus.NewRegistry()
		}
	})
	return svc.registry
}
