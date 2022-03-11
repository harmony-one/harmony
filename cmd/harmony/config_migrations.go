package main

import (
	"errors"
	"fmt"

	goversion "github.com/hashicorp/go-version"
	"github.com/pelletier/go-toml"

	"github.com/harmony-one/harmony/api/service/legacysync"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const legacyConfigVersion = "1.0.4"

func doMigrations(confVersion string, confTree *toml.Tree) error {
	Ver, err := goversion.NewVersion(confVersion)
	if err != nil {
		return fmt.Errorf("invalid or missing config file version - '%s'", confVersion)
	}
	legacyVer, _ := goversion.NewVersion(legacyConfigVersion)
	migrationKey := confVersion
	if Ver.LessThan(legacyVer) {
		migrationKey = legacyConfigVersion
	}

	migration, found := migrations[migrationKey]

	// Version does not match any of the migration criteria
	if !found {
		return fmt.Errorf("unrecognized config version - %s", confVersion)
	}

	for confVersion != tomlConfigVersion {
		confTree = migration(confTree)
		confVersion = confTree.Get("Version").(string)
		migration = migrations[confVersion]
	}
	return nil
}

func migrateConf(confBytes []byte) (harmonyconfig.HarmonyConfig, string, error) {
	var (
		migratedFrom string
	)
	confTree, err := toml.LoadBytes(confBytes)
	if err != nil {
		return harmonyconfig.HarmonyConfig{}, "", fmt.Errorf("config file parse error - %s", err.Error())
	}
	confVersion, found := confTree.Get("Version").(string)
	if !found {
		return harmonyconfig.HarmonyConfig{}, "", errors.New("config file invalid - no version entry found")
	}
	migratedFrom = confVersion
	if confVersion != tomlConfigVersion {
		err = doMigrations(confVersion, confTree)
		if err != nil {
			return harmonyconfig.HarmonyConfig{}, "", err
		}
	}

	// At this point we must be at current config version so
	// we can safely unmarshal it
	var config harmonyconfig.HarmonyConfig
	if err := confTree.Unmarshal(&config); err != nil {
		return harmonyconfig.HarmonyConfig{}, "", err
	}
	return config, migratedFrom, nil
}

var (
	migrations = make(map[string]configMigrationFunc)
)

type configMigrationFunc func(*toml.Tree) *toml.Tree

func init() {
	migrations["1.0.4"] = func(confTree *toml.Tree) *toml.Tree {
		ntStr := confTree.Get("Network.NetworkType").(string)
		nt := parseNetworkType(ntStr)

		defDNSSyncConf := getDefaultDNSSyncConfig(nt)
		defSyncConfig := getDefaultSyncConfig(nt)

		// Legacy conf missing fields
		if confTree.Get("Sync") == nil {
			confTree.Set("Sync", defSyncConfig)
		}

		if confTree.Get("HTTP.RosettaPort") == nil {
			confTree.Set("HTTP.RosettaPort", defaultConfig.HTTP.RosettaPort)
		}

		if confTree.Get("RPCOpt.RateLimterEnabled") == nil {
			confTree.Set("RPCOpt.RateLimterEnabled", defaultConfig.RPCOpt.RateLimterEnabled)
		}

		if confTree.Get("RPCOpt.RequestsPerSecond") == nil {
			confTree.Set("RPCOpt.RequestsPerSecond", defaultConfig.RPCOpt.RequestsPerSecond)
		}

		if confTree.Get("P2P.IP") == nil {
			confTree.Set("P2P.IP", defaultConfig.P2P.IP)
		}

		if confTree.Get("Prometheus") == nil {
			if defaultConfig.Prometheus != nil {
				confTree.Set("Prometheus", *defaultConfig.Prometheus)
			}
		}

		zoneField := confTree.Get("Network.DNSZone")
		if zone, ok := zoneField.(string); ok {
			confTree.Set("DNSSync.Zone", zone)
		}

		portField := confTree.Get("Network.DNSPort")
		if p, ok := portField.(int64); ok {
			p = p - legacysync.SyncingPortDifference
			confTree.Set("DNSSync.Port", p)
		} else {
			confTree.Set("DNSSync.Port", nodeconfig.DefaultDNSPort)
		}

		syncingField := confTree.Get("Network.LegacySyncing")
		if syncing, ok := syncingField.(bool); ok {
			confTree.Set("DNSSync.LegacySyncing", syncing)
		}

		clientField := confTree.Get("Sync.LegacyClient")
		if client, ok := clientField.(bool); ok {
			confTree.Set("DNSSync.Client", client)
		} else {
			confTree.Set("DNSSync.Client", defDNSSyncConf.Client)
		}

		serverField := confTree.Get("Sync.LegacyServer")
		if server, ok := serverField.(bool); ok {
			confTree.Set("DNSSync.Server", server)
		} else {
			confTree.Set("DNSSync.Server", defDNSSyncConf.Client)
		}

		serverPort := defDNSSyncConf.ServerPort
		serverPortField := confTree.Get("Sync.LegacyServerPort")
		if port, ok := serverPortField.(int64); ok {
			serverPort = int(port)
		}
		confTree.Set("DNSSync.ServerPort", serverPort)

		downloaderEnabledField := confTree.Get("Sync.Downloader")
		if downloaderEnabled, ok := downloaderEnabledField.(bool); ok && downloaderEnabled {
			// If we enabled downloader previously, run stream sync protocol.
			confTree.Set("Sync.Enabled", true)
		}

		confTree.Set("Version", "2.0.0")
		return confTree
	}

	migrations["2.0.0"] = func(confTree *toml.Tree) *toml.Tree {
		// Legacy conf missing fields
		if confTree.Get("Log.VerbosePrints") == nil {
			confTree.Set("Log.VerbosePrints", defaultConfig.Log.VerbosePrints)
		}

		confTree.Set("Version", "2.1.0")
		return confTree
	}

	migrations["2.1.0"] = func(confTree *toml.Tree) *toml.Tree {
		// Legacy conf missing fields
		if confTree.Get("Pprof.Enabled") == nil {
			confTree.Set("Pprof.Enabled", true)
		}
		if confTree.Get("Pprof.Folder") == nil {
			confTree.Set("Pprof.Folder", defaultConfig.Pprof.Folder)
		}
		if confTree.Get("Pprof.ProfileNames") == nil {
			confTree.Set("Pprof.ProfileNames", defaultConfig.Pprof.ProfileNames)
		}
		if confTree.Get("Pprof.ProfileIntervals") == nil {
			confTree.Set("Pprof.ProfileIntervals", defaultConfig.Pprof.ProfileIntervals)
		}
		if confTree.Get("Pprof.ProfileDebugValues") == nil {
			confTree.Set("Pprof.ProfileDebugValues", defaultConfig.Pprof.ProfileDebugValues)
		}
		if confTree.Get("P2P.DiscConcurrency") == nil {
			confTree.Set("P2P.DiscConcurrency", defaultConfig.P2P.DiscConcurrency)
		}

		confTree.Set("Version", "2.2.0")
		return confTree
	}

	migrations["2.2.0"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("HTTP.AuthPort") == nil {
			confTree.Set("HTTP.AuthPort", defaultConfig.HTTP.AuthPort)
		}

		confTree.Set("Version", "2.3.0")
		return confTree
	}

	migrations["2.3.0"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("P2P.MaxConnsPerIP") == nil {
			confTree.Set("P2P.MaxConnsPerIP", defaultConfig.P2P.MaxConnsPerIP)
		}

		confTree.Set("Version", "2.4.0")
		return confTree
	}

	migrations["2.4.0"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("WS.AuthPort") == nil {
			confTree.Set("WS.AuthPort", defaultConfig.WS.AuthPort)
		}

		confTree.Set("Version", "2.5.0")
		return confTree
	}

	migrations["2.5.0"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("TxPool.AccountSlots") == nil {
			confTree.Set("TxPool.AccountSlots", defaultConfig.TxPool.AccountSlots)
		}

		confTree.Set("Version", "2.5.1")
		return confTree
	}
}
