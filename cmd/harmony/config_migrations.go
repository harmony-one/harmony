package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	goversion "github.com/hashicorp/go-version"
	"github.com/pelletier/go-toml" // TODO support go-toml/v2

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

	migrations["2.5.1"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("P2P.DisablePrivateIPScan") == nil {
			confTree.Set("P2P.DisablePrivateIPScan", defaultConfig.P2P.DisablePrivateIPScan)
		}

		confTree.Set("Version", "2.5.2")
		return confTree
	}

	migrations["2.5.2"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("RPCOpt.EthRPCsEnabled") == nil {
			confTree.Set("RPCOpt.EthRPCsEnabled", defaultConfig.RPCOpt.EthRPCsEnabled)
		}

		if confTree.Get("RPCOpt.StakingRPCsEnabled") == nil {
			confTree.Set("RPCOpt.StakingRPCsEnabled", defaultConfig.RPCOpt.StakingRPCsEnabled)
		}

		if confTree.Get("RPCOpt.LegacyRPCsEnabled") == nil {
			confTree.Set("RPCOpt.LegacyRPCsEnabled", defaultConfig.RPCOpt.LegacyRPCsEnabled)
		}

		if confTree.Get("RPCOpt.RpcFilterFile") == nil {
			confTree.Set("RPCOpt.RpcFilterFile", defaultConfig.RPCOpt.RpcFilterFile)
		}

		confTree.Set("Version", "2.5.3")
		return confTree
	}

	migrations["2.5.3"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("TxPool.AllowedTxsFile") == nil {
			confTree.Set("TxPool.AllowedTxsFile", defaultConfig.TxPool.AllowedTxsFile)
		}
		confTree.Set("Version", "2.5.4")
		return confTree
	}

	migrations["2.5.4"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("TxPool.GlobalSlots") == nil {
			confTree.Set("TxPool.GlobalSlots", defaultConfig.TxPool.GlobalSlots)
		}
		confTree.Set("Version", "2.5.5")
		return confTree
	}

	migrations["2.5.5"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("Log.Console") == nil {
			confTree.Set("Log.Console", defaultConfig.Log.Console)
		}
		confTree.Set("Version", "2.5.6")
		return confTree
	}

	migrations["2.5.6"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("P2P.MaxPeers") == nil {
			confTree.Set("P2P.MaxPeers", defaultConfig.P2P.MaxPeers)
		}
		confTree.Set("Version", "2.5.7")
		return confTree
	}
	migrations["2.5.7"] = func(confTree *toml.Tree) *toml.Tree {
		confTree.Delete("DNSSync.LegacySyncing")

		confTree.Set("Version", "2.5.8")
		return confTree
	}

	migrations["2.5.8"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("Sync.StagedSync") == nil {
			confTree.Set("Sync.StagedSync", defaultConfig.Sync.StagedSync)
			confTree.Set("Sync.StagedSyncCfg", defaultConfig.Sync.StagedSyncCfg)
		}
		confTree.Set("Version", "2.5.9")
		return confTree
	}

	migrations["2.5.9"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("P2P.WaitForEachPeerToConnect") == nil {
			confTree.Set("P2P.WaitForEachPeerToConnect", defaultConfig.P2P.WaitForEachPeerToConnect)
		}
		confTree.Set("Version", "2.5.10")
		return confTree
	}

	migrations["2.5.10"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("P2P.ConnManagerLowWatermark") == nil {
			confTree.Set("P2P.ConnManagerLowWatermark", defaultConfig.P2P.ConnManagerLowWatermark)
		}
		if confTree.Get("P2P.ConnManagerHighWatermark") == nil {
			confTree.Set("P2P.ConnManagerHighWatermark", defaultConfig.P2P.ConnManagerHighWatermark)
		}
		if confTree.Get("Sync.MaxAdvertiseWaitTime") == nil {
			confTree.Set("Sync.MaxAdvertiseWaitTime", defaultConfig.Sync.MaxAdvertiseWaitTime)
		}
		confTree.Set("Version", "2.5.11")
		return confTree
	}

	migrations["2.5.11"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("General.TriesInMemory") == nil {
			confTree.Set("General.TriesInMemory", defaultConfig.General.TriesInMemory)
		}
		confTree.Set("Version", "2.5.12")
		return confTree
	}

	migrations["2.5.12"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("HTTP.ReadTimeout") == nil {
			confTree.Set("HTTP.ReadTimeout", defaultConfig.HTTP.ReadTimeout)
		}
		if confTree.Get("HTTP.WriteTimeout") == nil {
			confTree.Set("HTTP.WriteTimeout", defaultConfig.HTTP.WriteTimeout)
		}
		if confTree.Get("HTTP.IdleTimeout") == nil {
			confTree.Set("HTTP.IdleTimeout", defaultConfig.HTTP.IdleTimeout)
		}
		if confTree.Get("RPCOpt.EvmCallTimeout") == nil {
			confTree.Set("RPCOpt.EvmCallTimeout", defaultConfig.RPCOpt.EvmCallTimeout)
		}
		confTree.Set("Version", "2.5.13")
		return confTree
	}

	migrations["2.5.13"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("TxPool.AccountQueue") == nil {
			confTree.Set("TxPool.AccountQueue", defaultConfig.TxPool.AccountQueue)
		}
		if confTree.Get("TxPool.GlobalQueue") == nil {
			confTree.Set("TxPool.GlobalQueue", defaultConfig.TxPool.GlobalQueue)
		}
		if confTree.Get("TxPool.Lifetime") == nil {
			confTree.Set("TxPool.Lifetime", defaultConfig.TxPool.Lifetime.String())
		}
		if confTree.Get("TxPool.PriceLimit") == nil {
			confTree.Set("TxPool.PriceLimit", defaultConfig.TxPool.PriceLimit)
		}
		if confTree.Get("TxPool.PriceBump") == nil {
			confTree.Set("TxPool.PriceBump", defaultConfig.TxPool.PriceBump)
		}

		confTree.Set("Version", "2.5.14")
		return confTree
	}

	migrations["2.5.14"] = func(confTree *toml.Tree) *toml.Tree {
		if confTree.Get("GPO.Blocks") == nil {
			confTree.Set("GPO.Blocks", defaultConfig.GPO.Blocks)
		}
		if confTree.Get("GPO.Transactions") == nil {
			confTree.Set("GPO.Transactions", defaultConfig.GPO.Transactions)
		}
		if confTree.Get("GPO.Percentile") == nil {
			confTree.Set("GPO.Percentile", defaultConfig.GPO.Percentile)
		}
		if confTree.Get("GPO.DefaultPrice") == nil {
			confTree.Set("GPO.DefaultPrice", defaultConfig.GPO.DefaultPrice)
		}
		if confTree.Get("GPO.MaxPrice") == nil {
			confTree.Set("GPO.MaxPrice", defaultConfig.GPO.MaxPrice)
		}
		if confTree.Get("GPO.LowUsageThreshold") == nil {
			confTree.Set("GPO.LowUsageThreshold", defaultConfig.GPO.LowUsageThreshold)
		}
		if confTree.Get("GPO.BlockGasLimit") == nil {
			confTree.Set("GPO.BlockGasLimit", defaultConfig.GPO.BlockGasLimit)
		}
		// upgrade minor version because of `GPO` section introduction
		confTree.Set("Version", "2.6.0")
		return confTree
	}

	// check that the latest version here is the same as in default.go
	largestKey := getNextVersion(migrations)
	if largestKey != tomlConfigVersion {
		panic(fmt.Sprintf("next migration value: %s, toml version: %s", largestKey, tomlConfigVersion))
	}
}

func getNextVersion(x map[string]configMigrationFunc) string {
	versionMap := make(map[string]interface{}, 1)
	versionMap["Version"] = "FakeVersion"
	tree, _ := toml.TreeFromMap(versionMap)

	// needs to be sorted in case the order is incorrect
	keys := make([]string, len(x))
	i := 0
	for k := range x {
		keys[i] = k
		i++
	}
	// sorting keys (versions)
	// each key is in format "x.x.x". Normal sort won't work if the versions
	// don't have same number of digits, for example 1.02.10 and 1.2.9
	// so, we need a custom sort
	sort.Slice(keys, func(i, j int) bool {
		v1 := keys[i]
		v2 := keys[j]
		v1Parts := strings.Split(v1, ".")
		v2Parts := strings.Split(v2, ".")
		if len(v1Parts) > len(v2Parts) {
			return true
		} else if len(v1Parts) < len(v2Parts) {
			return false
		}
		for i := 0; i < len(v1Parts); i++ {
			n1, err1 := strconv.ParseInt(v1Parts[i], 10, 32)
			if err1 != nil {
				panic("wrong version format")
			}
			n2, err2 := strconv.ParseInt(v2Parts[i], 10, 32)
			if err2 != nil {
				panic("wrong version format")
			}
			if n1 > n2 {
				return true
			} else if n1 < n2 {
				return false
			}
		}
		return false
	})
	requiredFunc := x[keys[0]]
	tree = requiredFunc(tree)
	return tree.Get("Version").(string)
}
