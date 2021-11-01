package pprof

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// Config is the config for the pprof service
type Config struct {
	Enabled            bool
	ListenAddr         string
	Folder             string
	ProfileNames       []string
	ProfileIntervals   []int
	ProfileDebugValues []int
}

type Profile struct {
	Name       string
	Interval   int
	Debug      int
	ProfileRef *pprof.Profile
}

func (p Config) String() string {
	return fmt.Sprintf("%v, %v, %v, %v/%v/%v", p.Enabled, p.ListenAddr, p.Folder, p.ProfileNames, p.ProfileIntervals, p.ProfileDebugValues)
}

// Constants for profile names
const (
	CPU = "cpu"
)

// Service provides access to pprof profiles via HTTP and can save them to local disk periodically as user settings.
type Service struct {
	config   Config
	profiles map[string]Profile
}

var (
	initOnce sync.Once
	svc      = &Service{}
	cpuFile  *os.File
	cpuLock  sync.Mutex
)

// NewService creates the new pprof service
func NewService(cfg Config) *Service {
	initOnce.Do(func() {
		svc = newService(cfg)
	})
	return svc
}

func newService(cfg Config) *Service {
	if !cfg.Enabled {
		utils.Logger().Info().Msg("pprof service disabled...")
		return nil
	}

	utils.Logger().Debug().Str("cfg", cfg.String()).Msg("pprof")
	svc.config = cfg

	profiles, err := cfg.unpackProfilesIntoMap()
	if err != nil {
		log.Fatal("could not unpack pprof profiles into map")
	}
	svc.profiles = profiles

	return svc
}

// Start start the service
func (s *Service) Start() error {
	dir, err := filepath.Abs(s.config.Folder)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir, os.FileMode(0755))
	if err != nil {
		return err
	}

	go func() {
		utils.Logger().Info().Str("address", s.config.ListenAddr).Msg("starting pprof HTTP service")
		http.ListenAndServe(s.config.ListenAddr, nil)
	}()

	if _, ok := s.profiles[CPU]; ok {
		// The nature of the pprof CPU profile is fundamentally different to the other profiles, because it streams output to a file during profiling.
		// Thus it has to be started outside of the defined interval.
		go restartCpuProfile(dir)
	}

	for _, profile := range s.profiles {
		scheduleProfile(profile, dir)
	}

	return nil
}

// Stop stop the service
func (s *Service) Stop() error {
	dir, err := filepath.Abs(s.config.Folder)
	if err != nil {
		return err
	}
	for _, profile := range s.profiles {
		if profile.Name == CPU {
			stopCpuProfile()
		} else {
			err := saveProfile(profile, dir)
			if err != nil {
				utils.Logger().Error().Err(err).Msg(fmt.Sprintf("could not save pprof profile: %s", profile.Name))
			}
		}
	}
	return nil
}

// scheduleProfile schedules the provided profile based on the specified interval (e.g. saves the profile every x seconds)
func scheduleProfile(profile Profile, dir string) {
	go func() {
		if profile.Interval > 0 {
			ticker := time.NewTicker(time.Second * time.Duration(profile.Interval))
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if profile.Name == CPU {
						err := restartCpuProfile(dir)
						if err != nil {
							utils.Logger().Error().Err(err).Msg("could not start pprof CPU profile")
						}
					} else {
						err := saveProfile(profile, dir)
						if err != nil {
							utils.Logger().Error().Err(err).Msg(fmt.Sprintf("could not save pprof profile: %s", profile.Name))
						}
					}
				}
			}
		}
	}()
}

// saveProfile saves the provided profile in the specified directory
func saveProfile(profile Profile, dir string) error {
	f, err := newTempFile(dir, profile.Name, ".pb.gz")
	if err != nil {
		return err
	}
	defer f.Close()
	if profile.ProfileRef == nil {
		return nil
	}
	err = profile.ProfileRef.WriteTo(f, profile.Debug)
	if err != nil {
		return err
	}
	utils.Logger().Info().Msg(fmt.Sprintf("saved pprof profile in: %s", f.Name()))
	return nil
}

// restartCpuProfile stops the current CPU profile, if any and then starts a new CPU profile. While profiling in the background, the profile will be buffered and written to a file.
func restartCpuProfile(dir string) error {
	cpuLock.Lock()
	defer cpuLock.Unlock()
	stopCpuProfile()
	f, err := newTempFile(dir, CPU, ".pb.gz")
	if err != nil {
		return err
	}
	pprof.StartCPUProfile(f)
	cpuFile = f
	utils.Logger().Info().Msg(fmt.Sprintf("saved pprof CPU profile in: %s", f.Name()))
	return nil
}

// stopCpuProfile stops the current CPU profile, if any
func stopCpuProfile() {
	pprof.StopCPUProfile()
	if cpuFile != nil {
		cpuFile.Close()
		cpuFile = nil
	}
}

// unpackProfilesIntoMap unpacks the profiles specified in the configuration into a map
func (config *Config) unpackProfilesIntoMap() (map[string]Profile, error) {
	result := make(map[string]Profile, len(config.ProfileNames))
	if len(config.ProfileNames) == 0 {
		return nil, nil
	}
	for index, name := range config.ProfileNames {
		profile := Profile{
			Name:     name,
			Interval: 0, // 0 saves the profile when stopping the service
			Debug:    0, // 0 writes the gzip-compressed protocol buffer
		}
		// Try set interval value
		if len(config.ProfileIntervals) == len(config.ProfileNames) {
			profile.Interval = config.ProfileIntervals[index]
		} else if len(config.ProfileIntervals) > 0 {
			profile.Interval = config.ProfileIntervals[0]
		}
		// Try set debug value
		if len(config.ProfileDebugValues) == len(config.ProfileNames) {
			profile.Debug = config.ProfileDebugValues[index]
		} else if len(config.ProfileDebugValues) > 0 {
			profile.Debug = config.ProfileDebugValues[0]
		}
		// Try set the profile reference
		if profile.Name != CPU {
			if p := pprof.Lookup(profile.Name); p == nil {
				return nil, fmt.Errorf("pprof profile does not exist: %s", profile.Name)
			} else {
				profile.ProfileRef = p
			}
		}
		result[name] = profile
	}
	return result, nil
}

// newTempFile returns a new output file in dir with the provided prefix and suffix.
func newTempFile(dir, name, suffix string) (*os.File, error) {
	prefix := name + "."
	currentTime := time.Now().Unix()
	f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%s%d%s", prefix, currentTime, suffix)), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return nil, fmt.Errorf("could not create file of the form %s%d%s", prefix, currentTime, suffix)
	}
	return f, nil
}
