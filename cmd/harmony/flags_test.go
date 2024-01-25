package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/harmony-one/harmony/common/denominations"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

var (
	trueBool = true
)

func TestHarmonyFlags(t *testing.T) {
	tests := []struct {
		argStr    string
		expConfig harmonyconfig.HarmonyConfig
	}{
		{
			// running staking command from legacy node.sh
			argStr: "--bootnodes /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv," +
				"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9,/ip4/13.113.101." +
				"219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX,/ip4/99.81.170.167/tcp/12019/p" +
				"2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj --ip 8.8.8.8 --port 9000 --network_type=mainn" +
				"et --dns_zone=t.hmny.io --blacklist=./.hmy/blacklist.txt --min_peers=6 --max_bls_keys_per_node=" +
				"10 --broadcast_invalid_tx=true --verbosity=3 --is_archival=false --shard_id=-1 --staking=true -" +
				"-aws-config-source file:config.json --p2p.disc.concurrency 5 --p2p.security.max-conn-per-ip 5",
			expConfig: harmonyconfig.HarmonyConfig{
				Version: tomlConfigVersion,
				General: harmonyconfig.GeneralConfig{
					NodeType:   "validator",
					NoStaking:  false,
					ShardID:    -1,
					IsArchival: false,
					DataDir:    "./",
				},
				Network: harmonyconfig.NetworkConfig{
					NetworkType: "mainnet",
					BootNodes: []string{
						"/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
						"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9",
						"/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX",
						"/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj",
					},
				},
				DNSSync: harmonyconfig.DnsSync{
					Port:       6000,
					Zone:       "t.hmny.io",
					Server:     true,
					Client:     true,
					ServerPort: nodeconfig.DefaultDNSPort,
				},
				P2P: harmonyconfig.P2pConfig{
					Port:                     9000,
					IP:                       defaultConfig.P2P.IP,
					KeyFile:                  defaultConfig.P2P.KeyFile,
					DiscConcurrency:          5,
					MaxConnsPerIP:            5,
					DisablePrivateIPScan:     false,
					MaxPeers:                 defaultConfig.P2P.MaxPeers,
					ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
					ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
					WaitForEachPeerToConnect: false,
				},
				HTTP: harmonyconfig.HttpConfig{
					Enabled:        true,
					IP:             "127.0.0.1",
					Port:           9500,
					AuthPort:       9501,
					RosettaEnabled: false,
					RosettaPort:    9700,
					ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
					WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
					IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
				},
				RPCOpt: harmonyconfig.RpcOptConfig{
					DebugEnabled:       false,
					EthRPCsEnabled:     true,
					StakingRPCsEnabled: true,
					LegacyRPCsEnabled:  true,
					RpcFilterFile:      "./.hmy/rpc_filter.txt",
					RateLimterEnabled:  true,
					RequestsPerSecond:  1000,
					EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
					PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
				},
				WS: harmonyconfig.WsConfig{
					Enabled:  true,
					IP:       "127.0.0.1",
					Port:     9800,
					AuthPort: 9801,
				},
				Consensus: &harmonyconfig.ConsensusConfig{
					MinPeers:     6,
					AggregateSig: true,
				},
				BLSKeys: harmonyconfig.BlsConfig{
					KeyDir:           "./.hmy/blskeys",
					KeyFiles:         []string{},
					MaxKeys:          10,
					PassEnabled:      true,
					PassSrcType:      "auto",
					PassFile:         "",
					SavePassphrase:   false,
					KMSEnabled:       false,
					KMSConfigSrcType: "file",
					KMSConfigFile:    "config.json",
				},
				TxPool: harmonyconfig.TxPoolConfig{
					BlacklistFile:     "./.hmy/blacklist.txt",
					AllowedTxsFile:    "./.hmy/allowedtxs.txt",
					RosettaFixFile:    "",
					AccountSlots:      16,
					GlobalSlots:       4096,
					LocalAccountsFile: "./.hmy/locals.txt",
					AccountQueue:      64,
					GlobalQueue:       5120,
					Lifetime:          30 * time.Minute,
					PriceLimit:        100e9,
					PriceBump:         1,
				},
				Pprof: harmonyconfig.PprofConfig{
					Enabled:            false,
					ListenAddr:         "127.0.0.1:6060",
					Folder:             "./profiles",
					ProfileNames:       []string{},
					ProfileIntervals:   []int{600},
					ProfileDebugValues: []int{0},
				},
				Log: harmonyconfig.LogConfig{
					Console:      false,
					Folder:       "./latest",
					FileName:     "validator-8.8.8.8-9000.log",
					RotateSize:   100,
					RotateCount:  0,
					RotateMaxAge: 0,
					Verbosity:    3,
					Context: &harmonyconfig.LogContext{
						IP:   "8.8.8.8",
						Port: 9000,
					},
					VerbosePrints: harmonyconfig.LogVerbosePrints{
						Config: true,
					},
				},
				Sys: &harmonyconfig.SysConfig{
					NtpServer: defaultSysConfig.NtpServer,
				},
				Legacy: &harmonyconfig.LegacyConfig{
					TPBroadcastInvalidTxn: &trueBool,
				},
				Prometheus: &harmonyconfig.PrometheusConfig{
					Enabled:    true,
					IP:         "0.0.0.0",
					Port:       9900,
					EnablePush: true,
					Gateway:    "https://gateway.harmony.one",
				},
				Sync: defaultMainnetSyncConfig,
				ShardData: harmonyconfig.ShardDataConfig{
					EnableShardData: false,
					DiskCount:       8,
					ShardCount:      4,
					CacheTime:       10,
					CacheSize:       512,
				},
				GPO: harmonyconfig.GasPriceOracleConfig{
					Blocks:            defaultConfig.GPO.Blocks,
					Transactions:      defaultConfig.GPO.Transactions,
					Percentile:        defaultConfig.GPO.Percentile,
					DefaultPrice:      defaultConfig.GPO.DefaultPrice,
					MaxPrice:          defaultConfig.GPO.MaxPrice,
					LowUsageThreshold: defaultConfig.GPO.LowUsageThreshold,
					BlockGasLimit:     defaultConfig.GPO.BlockGasLimit,
				},
				Cache: harmonyconfig.CacheConfig{
					Disabled:        defaultConfig.Cache.Disabled,
					TrieNodeLimit:   defaultCacheConfig.TrieNodeLimit,
					TriesInMemory:   defaultConfig.Cache.TriesInMemory,
					TrieTimeLimit:   defaultConfig.Cache.TrieTimeLimit,
					SnapshotLimit:   defaultConfig.Cache.SnapshotLimit,
					SnapshotWait:    defaultConfig.Cache.SnapshotWait,
					Preimages:       defaultConfig.Cache.Preimages,
					SnapshotNoBuild: defaultConfig.Cache.SnapshotNoBuild,
				},
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, getRootFlags(), applyRootFlags)
		hc, err := ts.run(strings.Split(test.argStr, " "))
		if err != nil {
			t.Fatalf("Test %v: %v", i, err)
		}
		if !reflect.DeepEqual(hc, test.expConfig) {
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, hc, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestGeneralFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.GeneralConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: harmonyconfig.GeneralConfig{
				NodeType:   "validator",
				NoStaking:  false,
				ShardID:    -1,
				IsArchival: false,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.legacy", "--run.shard=0",
				"--run.archive=true", "--datadir=./.hmy"},
			expConfig: harmonyconfig.GeneralConfig{
				NodeType:   "explorer",
				NoStaking:  true,
				ShardID:    0,
				IsArchival: true,
				DataDir:    "./.hmy",
			},
		},
		{
			args: []string{"--node_type", "explorer", "--staking", "--shard_id", "0",
				"--is_archival", "--db_dir", "./"},
			expConfig: harmonyconfig.GeneralConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: true,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--staking=false", "--is_archival=false"},
			expConfig: harmonyconfig.GeneralConfig{
				NodeType:   "validator",
				NoStaking:  true,
				ShardID:    -1,
				IsArchival: false,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.shard", "0"},
			expConfig: harmonyconfig.GeneralConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: false,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.shard", "0", "--run.archive=false"},
			expConfig: harmonyconfig.GeneralConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: false,
				DataDir:    "./",
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, generalFlags, applyGeneralFlags)

		got, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(got.General, test.expConfig) {
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, got.General, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestNetworkFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.HarmonyConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: harmonyconfig.HarmonyConfig{
				Network: harmonyconfig.NetworkConfig{
					NetworkType: defNetworkType,
					BootNodes:   nodeconfig.GetDefaultBootNodes(defNetworkType),
				},
				DNSSync: getDefaultDNSSyncConfig(defNetworkType)},
		},
		{
			args: []string{"-n", "stn"},
			expConfig: harmonyconfig.HarmonyConfig{
				Network: harmonyconfig.NetworkConfig{
					NetworkType: nodeconfig.Stressnet,
					BootNodes:   nodeconfig.GetDefaultBootNodes(nodeconfig.Stressnet),
				},
				DNSSync: getDefaultDNSSyncConfig(nodeconfig.Stressnet),
			},
		},
		{
			args: []string{"--network", "stk", "--bootnodes", "1,2,3,4", "--dns.zone", "8.8.8.8",
				"--dns.port", "9001", "--dns.server-port", "9002"},
			expConfig: harmonyconfig.HarmonyConfig{
				Network: harmonyconfig.NetworkConfig{
					NetworkType: "pangaea",
					BootNodes:   []string{"1", "2", "3", "4"},
				},
				DNSSync: harmonyconfig.DnsSync{
					Port:       9001,
					Zone:       "8.8.8.8",
					Server:     true,
					ServerPort: 9002,
				},
			},
		},
		{
			args: []string{"--network_type", "stk", "--bootnodes", "1,2,3,4", "--dns_zone", "8.8.8.8",
				"--dns_port", "9001"},
			expConfig: harmonyconfig.HarmonyConfig{
				Network: harmonyconfig.NetworkConfig{
					NetworkType: "pangaea",
					BootNodes:   []string{"1", "2", "3", "4"},
				},
				DNSSync: harmonyconfig.DnsSync{
					Port:       9001,
					Zone:       "8.8.8.8",
					Server:     true,
					ServerPort: nodeconfig.GetDefaultDNSPort(nodeconfig.Pangaea),
				},
			},
		},
	}
	for i, test := range tests {
		neededFlags := make([]cli.Flag, 0)
		neededFlags = append(neededFlags, networkFlags...)
		neededFlags = append(neededFlags, dnsSyncFlags...)
		ts := newFlagTestSuite(t, neededFlags, func(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
			// This is the network related logic in function getharmonyconfig.HarmonyConfig
			nt := getNetworkType(cmd)
			config.Network = getDefaultNetworkConfig(nt)
			config.DNSSync = getDefaultDNSSyncConfig(nt)
			applyNetworkFlags(cmd, config)
			applyDNSSyncFlags(cmd, config)
		})

		got, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(got.Network, test.expConfig.Network) {
			t.Errorf("Test %v: unexpected network config: \n\t%+v\n\t%+v", i, got.Network, test.expConfig.Network)
		}
		if !reflect.DeepEqual(got.DNSSync, test.expConfig.DNSSync) {
			t.Errorf("Test %v: unexpected dnssync config: \n\t%+v\n\t%+v", i, got.DNSSync, test.expConfig.DNSSync)
		}
		ts.tearDown()
	}
}

var defDataStore = ".dht-127.0.0.1"

func TestP2PFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.P2pConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.P2P,
		},
		{
			args: []string{"--p2p.port", "9001", "--p2p.keyfile", "./key.file", "--p2p.dht.datastore",
				defDataStore},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     9001,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./key.file",
				DHTDataStore:             &defDataStore,
				MaxConnsPerIP:            10,
				DisablePrivateIPScan:     false,
				MaxPeers:                 defaultConfig.P2P.MaxPeers,
				ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
				ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
				WaitForEachPeerToConnect: false,
			},
		},
		{
			args: []string{"--port", "9001", "--key", "./key.file"},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     9001,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./key.file",
				MaxConnsPerIP:            10,
				DisablePrivateIPScan:     false,
				MaxPeers:                 defaultConfig.P2P.MaxPeers,
				ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
				ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
				WaitForEachPeerToConnect: false,
			},
		},
		{
			args: []string{"--p2p.port", "9001", "--p2p.disc.concurrency", "5", "--p2p.security.max-conn-per-ip", "5"},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     9001,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./.hmykey",
				DiscConcurrency:          5,
				MaxConnsPerIP:            5,
				DisablePrivateIPScan:     false,
				MaxPeers:                 defaultConfig.P2P.MaxPeers,
				ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
				ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
				WaitForEachPeerToConnect: false,
			},
		},
		{
			args: []string{"--p2p.no-private-ip-scan"},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     nodeconfig.DefaultP2PPort,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./.hmykey",
				DiscConcurrency:          nodeconfig.DefaultP2PConcurrency,
				MaxConnsPerIP:            nodeconfig.DefaultMaxConnPerIP,
				DisablePrivateIPScan:     true,
				MaxPeers:                 defaultConfig.P2P.MaxPeers,
				ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
				ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
				WaitForEachPeerToConnect: false,
			},
		},
		{
			args: []string{"--p2p.security.max-peers", "100"},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     nodeconfig.DefaultP2PPort,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./.hmykey",
				DiscConcurrency:          nodeconfig.DefaultP2PConcurrency,
				MaxConnsPerIP:            nodeconfig.DefaultMaxConnPerIP,
				DisablePrivateIPScan:     defaultConfig.P2P.DisablePrivateIPScan,
				MaxPeers:                 100,
				ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
				ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
				WaitForEachPeerToConnect: false,
			},
		},
		{
			args: []string{"--p2p.connmgr-low", "100"},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     nodeconfig.DefaultP2PPort,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./.hmykey",
				DiscConcurrency:          nodeconfig.DefaultP2PConcurrency,
				MaxConnsPerIP:            nodeconfig.DefaultMaxConnPerIP,
				DisablePrivateIPScan:     defaultConfig.P2P.DisablePrivateIPScan,
				MaxPeers:                 defaultConfig.P2P.MaxPeers,
				ConnManagerLowWatermark:  100,
				ConnManagerHighWatermark: defaultConfig.P2P.ConnManagerHighWatermark,
				WaitForEachPeerToConnect: false,
			},
		},
		{
			args: []string{"--p2p.connmgr-high", "400"},
			expConfig: harmonyconfig.P2pConfig{
				Port:                     nodeconfig.DefaultP2PPort,
				IP:                       nodeconfig.DefaultPublicListenIP,
				KeyFile:                  "./.hmykey",
				DiscConcurrency:          nodeconfig.DefaultP2PConcurrency,
				MaxConnsPerIP:            nodeconfig.DefaultMaxConnPerIP,
				DisablePrivateIPScan:     defaultConfig.P2P.DisablePrivateIPScan,
				MaxPeers:                 defaultConfig.P2P.MaxPeers,
				ConnManagerLowWatermark:  defaultConfig.P2P.ConnManagerLowWatermark,
				ConnManagerHighWatermark: 400,
				WaitForEachPeerToConnect: false,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(p2pFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
				applyLegacyMiscFlags(cmd, config)
				applyP2PFlags(cmd, config)
			},
		)

		got, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(got.P2P, test.expConfig) {
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, got.P2P, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestRPCFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.HttpConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.HTTP,
		},
		{
			args: []string{"--http=false"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        false,
				RosettaEnabled: false,
				IP:             defaultConfig.HTTP.IP,
				Port:           defaultConfig.HTTP.Port,
				AuthPort:       defaultConfig.HTTP.AuthPort,
				RosettaPort:    defaultConfig.HTTP.RosettaPort,
				ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
				WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
				IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             "8.8.8.8",
				Port:           9001,
				AuthPort:       defaultConfig.HTTP.AuthPort,
				RosettaPort:    defaultConfig.HTTP.RosettaPort,
				ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
				WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
				IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.auth-port", "9001"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             "8.8.8.8",
				Port:           defaultConfig.HTTP.Port,
				AuthPort:       9001,
				RosettaPort:    defaultConfig.HTTP.RosettaPort,
				ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
				WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
				IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001", "--http.rosetta.port", "10001"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        true,
				RosettaEnabled: true,
				IP:             "8.8.8.8",
				Port:           9001,
				AuthPort:       defaultConfig.HTTP.AuthPort,
				RosettaPort:    10001,
				ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
				WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
				IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.rosetta.port", "10001"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        true,
				RosettaEnabled: true,
				IP:             "8.8.8.8",
				Port:           defaultConfig.HTTP.Port,
				AuthPort:       defaultConfig.HTTP.AuthPort,
				RosettaPort:    10001,
				ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
				WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
				IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             nodeconfig.DefaultPublicListenIP,
				Port:           9501,
				AuthPort:       9502,
				RosettaPort:    9701,
				ReadTimeout:    defaultConfig.HTTP.ReadTimeout,
				WriteTimeout:   defaultConfig.HTTP.WriteTimeout,
				IdleTimeout:    defaultConfig.HTTP.IdleTimeout,
			},
		},
		{
			args: []string{"--http.timeout.read", "10s", "--http.timeout.write", "20s", "--http.timeout.idle", "30s"},
			expConfig: harmonyconfig.HttpConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             defaultConfig.HTTP.IP,
				Port:           defaultConfig.HTTP.Port,
				AuthPort:       defaultConfig.HTTP.AuthPort,
				RosettaPort:    defaultConfig.HTTP.RosettaPort,
				ReadTimeout:    "10s",
				WriteTimeout:   "20s",
				IdleTimeout:    "30s",
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(httpFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
				applyLegacyMiscFlags(cmd, config)
				applyHTTPFlags(cmd, config)
			},
		)

		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.HTTP, test.expConfig) {
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, hc.HTTP, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestWSFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.WsConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.WS,
		},
		{
			args: []string{"--ws=false"},
			expConfig: harmonyconfig.WsConfig{
				Enabled:  false,
				IP:       defaultConfig.WS.IP,
				Port:     defaultConfig.WS.Port,
				AuthPort: defaultConfig.WS.AuthPort,
			},
		},
		{
			args: []string{"--ws", "--ws.ip", "8.8.8.8", "--ws.port", "9001"},
			expConfig: harmonyconfig.WsConfig{
				Enabled:  true,
				IP:       "8.8.8.8",
				Port:     9001,
				AuthPort: defaultConfig.WS.AuthPort,
			},
		},
		{
			args: []string{"--ws", "--ws.ip", "8.8.8.8", "--ws.auth-port", "9001"},
			expConfig: harmonyconfig.WsConfig{
				Enabled:  true,
				IP:       "8.8.8.8",
				Port:     defaultConfig.WS.Port,
				AuthPort: 9001,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: harmonyconfig.WsConfig{
				Enabled:  true,
				IP:       nodeconfig.DefaultPublicListenIP,
				Port:     9801,
				AuthPort: 9802,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(wsFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
				applyLegacyMiscFlags(cmd, config)
				applyWSFlags(cmd, config)
			},
		)

		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.WS, test.expConfig) {
			t.Errorf("Test %v: \n\t%+v\n\t%+v", i, hc.WS, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestRPCOptFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.RpcOptConfig
	}{
		{
			args: []string{"--rpc.debug"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       true,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.eth=false"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     false,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.staking=false"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: false,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.legacy=false"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  false,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.filterspath=./rmf.toml"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./rmf.toml",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.ratelimiter", "--rpc.ratelimit", "2000"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  2000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.ratelimiter=false", "--rpc.ratelimit", "2000"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  false,
				RequestsPerSecond:  2000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.evm-call-timeout", "10s"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     "10s",
				PreimagesEnabled:   defaultConfig.RPCOpt.PreimagesEnabled,
			},
		},

		{
			args: []string{"--rpc.preimages"},
			expConfig: harmonyconfig.RpcOptConfig{
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimterEnabled:  true,
				RequestsPerSecond:  1000,
				EvmCallTimeout:     defaultConfig.RPCOpt.EvmCallTimeout,
				PreimagesEnabled:   true,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, rpcOptFlags, applyRPCOptFlags)

		hc, _ := ts.run(test.args)

		if !reflect.DeepEqual(hc.RPCOpt, test.expConfig) {
			t.Errorf("Test %v: \n\t%+v\n\t%+v", i, hc.RPCOpt, test.expConfig)
		}

		ts.tearDown()
	}
}

func TestBLSFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.BlsConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.BLSKeys,
		},
		{
			args: []string{"--bls.dir", "./blskeys", "--bls.keys", "key1,key2",
				"--bls.maxkeys", "8", "--bls.pass", "--bls.pass.src", "auto", "--bls.pass.save",
				"--bls.kms", "--bls.kms.src", "shared",
			},
			expConfig: harmonyconfig.BlsConfig{
				KeyDir:           "./blskeys",
				KeyFiles:         []string{"key1", "key2"},
				MaxKeys:          8,
				PassEnabled:      true,
				PassSrcType:      "auto",
				PassFile:         "",
				SavePassphrase:   true,
				KMSEnabled:       true,
				KMSConfigSrcType: "shared",
				KMSConfigFile:    "",
			},
		},
		{
			args: []string{"--bls.pass.file", "xxx.pass", "--bls.kms.config", "config.json"},
			expConfig: harmonyconfig.BlsConfig{
				KeyDir:           defaultConfig.BLSKeys.KeyDir,
				KeyFiles:         defaultConfig.BLSKeys.KeyFiles,
				MaxKeys:          defaultConfig.BLSKeys.MaxKeys,
				PassEnabled:      true,
				PassSrcType:      "file",
				PassFile:         "xxx.pass",
				SavePassphrase:   false,
				KMSEnabled:       false,
				KMSConfigSrcType: "file",
				KMSConfigFile:    "config.json",
			},
		},
		{
			args: []string{"--blskey_file", "key1,key2", "--blsfolder", "./hmykeys",
				"--max_bls_keys_per_node", "5", "--blspass", "file:xxx.pass", "--save-passphrase",
				"--aws-config-source", "file:config.json",
			},
			expConfig: harmonyconfig.BlsConfig{
				KeyDir:           "./hmykeys",
				KeyFiles:         []string{"key1", "key2"},
				MaxKeys:          5,
				PassEnabled:      true,
				PassSrcType:      "file",
				PassFile:         "xxx.pass",
				SavePassphrase:   true,
				KMSEnabled:       false,
				KMSConfigSrcType: "file",
				KMSConfigFile:    "config.json",
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, blsFlags, applyBLSFlags)

		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.BLSKeys, test.expConfig) {
			t.Errorf("Test %v: \n\t%+v\n\t%+v", i, hc.BLSKeys, test.expConfig)
		}

		ts.tearDown()
	}
}

func TestConsensusFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *harmonyconfig.ConsensusConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--consensus.min-peers", "10", "--consensus.aggregate-sig=false"},
			expConfig: &harmonyconfig.ConsensusConfig{
				MinPeers:     10,
				AggregateSig: false,
			},
		},
		{
			args: []string{"--delay_commit", "10ms", "--block_period", "5", "--min_peers", "10",
				"--consensus.aggregate-sig=true"},
			expConfig: &harmonyconfig.ConsensusConfig{
				MinPeers:     10,
				AggregateSig: true,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, consensusFlags, applyConsensusFlags)

		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.Consensus, test.expConfig) {
			t.Errorf("Test %v: unexpected config \n\t%+v\n\t%+v", i, hc.Consensus, test.expConfig)
		}

		ts.tearDown()
	}
}

func TestTxPoolFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.TxPoolConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: harmonyconfig.TxPoolConfig{
				BlacklistFile:     defaultConfig.TxPool.BlacklistFile,
				AllowedTxsFile:    defaultConfig.TxPool.AllowedTxsFile,
				RosettaFixFile:    defaultConfig.TxPool.RosettaFixFile,
				AccountSlots:      defaultConfig.TxPool.AccountSlots,
				LocalAccountsFile: defaultConfig.TxPool.LocalAccountsFile,
				GlobalSlots:       defaultConfig.TxPool.GlobalSlots,
				AccountQueue:      defaultConfig.TxPool.AccountQueue,
				GlobalQueue:       defaultConfig.TxPool.GlobalQueue,
				Lifetime:          defaultConfig.TxPool.Lifetime,
				PriceLimit:        100e9,
				PriceBump:         1,
			},
		},
		{
			args: []string{"--txpool.blacklist", "blacklist.file", "--txpool.rosettafixfile", "rosettafix.file", "--txpool.allowedtxs", "allowedtxs.txt"},
			expConfig: harmonyconfig.TxPoolConfig{
				BlacklistFile:     "blacklist.file",
				AllowedTxsFile:    "allowedtxs.txt",
				RosettaFixFile:    "rosettafix.file",
				AccountSlots:      defaultConfig.TxPool.AccountSlots,
				GlobalSlots:       defaultConfig.TxPool.GlobalSlots,
				AccountQueue:      defaultConfig.TxPool.AccountQueue,
				GlobalQueue:       defaultConfig.TxPool.GlobalQueue,
				Lifetime:          defaultConfig.TxPool.Lifetime,
				LocalAccountsFile: defaultConfig.TxPool.LocalAccountsFile,
				PriceLimit:        100e9,
				PriceBump:         1,
			},
		},
		{
			args: []string{"--blacklist", "blacklist.file", "--txpool.rosettafixfile", "rosettafix.file"},
			expConfig: harmonyconfig.TxPoolConfig{
				BlacklistFile:     "blacklist.file",
				RosettaFixFile:    "rosettafix.file",
				AllowedTxsFile:    defaultConfig.TxPool.AllowedTxsFile,
				AccountSlots:      defaultConfig.TxPool.AccountSlots,
				GlobalSlots:       defaultConfig.TxPool.GlobalSlots,
				AccountQueue:      defaultConfig.TxPool.AccountQueue,
				GlobalQueue:       defaultConfig.TxPool.GlobalQueue,
				Lifetime:          defaultConfig.TxPool.Lifetime,
				LocalAccountsFile: defaultConfig.TxPool.LocalAccountsFile,
				PriceLimit:        100e9,
				PriceBump:         1,
			},
		},
		{
			args: []string{"--txpool.accountslots", "5", "--txpool.blacklist", "blacklist.file", "--txpool.rosettafixfile", "rosettafix.file"},
			expConfig: harmonyconfig.TxPoolConfig{
				AccountSlots:      5,
				BlacklistFile:     "blacklist.file",
				AllowedTxsFile:    defaultConfig.TxPool.AllowedTxsFile,
				RosettaFixFile:    "rosettafix.file",
				LocalAccountsFile: defaultConfig.TxPool.LocalAccountsFile,
				GlobalSlots:       defaultConfig.TxPool.GlobalSlots,
				AccountQueue:      defaultConfig.TxPool.AccountQueue,
				GlobalQueue:       defaultConfig.TxPool.GlobalQueue,
				Lifetime:          defaultConfig.TxPool.Lifetime,
				PriceLimit:        100e9,
				PriceBump:         1,
			},
		},
		{
			args: []string{"--txpool.locals", "locals.txt"},
			expConfig: harmonyconfig.TxPoolConfig{
				BlacklistFile:     defaultConfig.TxPool.BlacklistFile,
				AllowedTxsFile:    defaultConfig.TxPool.AllowedTxsFile,
				RosettaFixFile:    defaultConfig.TxPool.RosettaFixFile,
				AccountSlots:      defaultConfig.TxPool.AccountSlots,
				LocalAccountsFile: "locals.txt",
				GlobalSlots:       defaultConfig.TxPool.GlobalSlots,
				AccountQueue:      defaultConfig.TxPool.AccountQueue,
				GlobalQueue:       defaultConfig.TxPool.GlobalQueue,
				Lifetime:          defaultConfig.TxPool.Lifetime,
				PriceLimit:        100e9,
				PriceBump:         1,
			},
		},
		{
			args: []string{"--txpool.globalslots", "10240"},
			expConfig: harmonyconfig.TxPoolConfig{
				BlacklistFile:     defaultConfig.TxPool.BlacklistFile,
				AllowedTxsFile:    defaultConfig.TxPool.AllowedTxsFile,
				RosettaFixFile:    defaultConfig.TxPool.RosettaFixFile,
				AccountSlots:      defaultConfig.TxPool.AccountSlots,
				LocalAccountsFile: defaultConfig.TxPool.LocalAccountsFile,
				GlobalSlots:       10240,
				AccountQueue:      defaultConfig.TxPool.AccountQueue,
				GlobalQueue:       defaultConfig.TxPool.GlobalQueue,
				Lifetime:          defaultConfig.TxPool.Lifetime,
				PriceLimit:        100e9,
				PriceBump:         1,
			},
		},
		{
			args: []string{"--txpool.accountqueue", "128", "--txpool.globalqueue", "10240", "--txpool.lifetime", "15m", "--txpool.pricelimit", "100", "--txpool.pricebump", "2"},
			expConfig: harmonyconfig.TxPoolConfig{
				BlacklistFile:     defaultConfig.TxPool.BlacklistFile,
				AllowedTxsFile:    defaultConfig.TxPool.AllowedTxsFile,
				RosettaFixFile:    defaultConfig.TxPool.RosettaFixFile,
				AccountSlots:      defaultConfig.TxPool.AccountSlots,
				LocalAccountsFile: defaultConfig.TxPool.LocalAccountsFile,
				GlobalSlots:       defaultConfig.TxPool.GlobalSlots,
				AccountQueue:      128,
				GlobalQueue:       10240,
				Lifetime:          15 * time.Minute,
				PriceLimit:        100,
				PriceBump:         2,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, txPoolFlags, applyTxPoolFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.TxPool, test.expConfig) {
			t.Errorf("Test %v: unexpected config\n\t%+v\n\t%+v", i, hc.TxPool, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestPprofFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.PprofConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.Pprof,
		},
		{
			args: []string{"--pprof"},
			expConfig: harmonyconfig.PprofConfig{
				Enabled:            true,
				ListenAddr:         defaultConfig.Pprof.ListenAddr,
				Folder:             defaultConfig.Pprof.Folder,
				ProfileNames:       defaultConfig.Pprof.ProfileNames,
				ProfileIntervals:   defaultConfig.Pprof.ProfileIntervals,
				ProfileDebugValues: defaultConfig.Pprof.ProfileDebugValues,
			},
		},
		{
			args: []string{"--pprof.addr", "8.8.8.8:9001"},
			expConfig: harmonyconfig.PprofConfig{
				Enabled:            true,
				ListenAddr:         "8.8.8.8:9001",
				Folder:             defaultConfig.Pprof.Folder,
				ProfileNames:       defaultConfig.Pprof.ProfileNames,
				ProfileIntervals:   defaultConfig.Pprof.ProfileIntervals,
				ProfileDebugValues: defaultConfig.Pprof.ProfileDebugValues,
			},
		},
		{
			args: []string{"--pprof=false", "--pprof.addr", "8.8.8.8:9001"},
			expConfig: harmonyconfig.PprofConfig{
				Enabled:            false,
				ListenAddr:         "8.8.8.8:9001",
				Folder:             defaultConfig.Pprof.Folder,
				ProfileNames:       defaultConfig.Pprof.ProfileNames,
				ProfileIntervals:   defaultConfig.Pprof.ProfileIntervals,
				ProfileDebugValues: defaultConfig.Pprof.ProfileDebugValues,
			},
		},
		{
			args: []string{"--pprof.profile.names", "cpu,heap,mutex"},
			expConfig: harmonyconfig.PprofConfig{
				Enabled:            true,
				ListenAddr:         defaultConfig.Pprof.ListenAddr,
				Folder:             defaultConfig.Pprof.Folder,
				ProfileNames:       []string{"cpu", "heap", "mutex"},
				ProfileIntervals:   defaultConfig.Pprof.ProfileIntervals,
				ProfileDebugValues: defaultConfig.Pprof.ProfileDebugValues,
			},
		},
		{
			args: []string{"--pprof.profile.intervals", "0,1"},
			expConfig: harmonyconfig.PprofConfig{
				Enabled:            true,
				ListenAddr:         defaultConfig.Pprof.ListenAddr,
				Folder:             defaultConfig.Pprof.Folder,
				ProfileNames:       defaultConfig.Pprof.ProfileNames,
				ProfileIntervals:   []int{0, 1},
				ProfileDebugValues: defaultConfig.Pprof.ProfileDebugValues,
			},
		},
		{
			args: []string{"--pprof.profile.debug", "0,1,0"},
			expConfig: harmonyconfig.PprofConfig{
				Enabled:            true,
				ListenAddr:         defaultConfig.Pprof.ListenAddr,
				Folder:             defaultConfig.Pprof.Folder,
				ProfileNames:       defaultConfig.Pprof.ProfileNames,
				ProfileIntervals:   defaultConfig.Pprof.ProfileIntervals,
				ProfileDebugValues: []int{0, 1, 0},
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, pprofFlags, applyPprofFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.Pprof, test.expConfig) {
			t.Errorf("Test %v: unexpected config\n\t%+v\n\t%+v", i, hc.Pprof, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestLogFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.LogConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.Log,
		},
		{
			args: []string{"--log.dir", "latest_log", "--log.max-size", "10", "--log.rotate-count", "3",
				"--log.rotate-max-age", "0", "--log.name", "harmony.log", "--log.verb", "5",
				"--log.verbose-prints", "config"},
			expConfig: harmonyconfig.LogConfig{
				Folder:       "latest_log",
				FileName:     "harmony.log",
				RotateSize:   10,
				RotateCount:  3,
				RotateMaxAge: 0,
				Verbosity:    5,
				VerbosePrints: harmonyconfig.LogVerbosePrints{
					Config: true,
				},
				Context: nil,
			},
		},
		{
			args: []string{"--log.ctx.ip", "8.8.8.8", "--log.ctx.port", "9001"},
			expConfig: harmonyconfig.LogConfig{
				Console:       defaultConfig.Log.Console,
				Folder:        defaultConfig.Log.Folder,
				FileName:      defaultConfig.Log.FileName,
				RotateSize:    defaultConfig.Log.RotateSize,
				RotateCount:   defaultConfig.Log.RotateCount,
				RotateMaxAge:  defaultConfig.Log.RotateMaxAge,
				Verbosity:     defaultConfig.Log.Verbosity,
				VerbosePrints: defaultConfig.Log.VerbosePrints,
				Context: &harmonyconfig.LogContext{
					IP:   "8.8.8.8",
					Port: 9001,
				},
			},
		},
		{
			args: []string{"--log_folder", "latest_log", "--log_max_size", "10", "--verbosity",
				"5", "--ip", "8.8.8.8", "--port", "9001"},
			expConfig: harmonyconfig.LogConfig{
				Folder:        "latest_log",
				FileName:      "validator-8.8.8.8-9001.log",
				RotateSize:    10,
				RotateCount:   0,
				RotateMaxAge:  0,
				Verbosity:     5,
				VerbosePrints: defaultConfig.Log.VerbosePrints,
				Context: &harmonyconfig.LogContext{
					IP:   "8.8.8.8",
					Port: 9001,
				},
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(logFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
				applyLegacyMiscFlags(cmd, config)
				applyLogFlags(cmd, config)
			},
		)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.Log, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Log, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestSysFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *harmonyconfig.SysConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: &harmonyconfig.SysConfig{
				NtpServer: defaultSysConfig.NtpServer,
			},
		},
		{
			args: []string{"--sys.ntp", "0.pool.ntp.org"},
			expConfig: &harmonyconfig.SysConfig{
				NtpServer: "0.pool.ntp.org",
			},
		},
	}

	for i, test := range tests {
		ts := newFlagTestSuite(t, sysFlags, applySysFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.Sys, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Sys, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestGPOFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.GasPriceOracleConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: harmonyconfig.GasPriceOracleConfig{
				Blocks:            defaultConfig.GPO.Blocks,
				Transactions:      defaultConfig.GPO.Transactions,
				Percentile:        defaultConfig.GPO.Percentile,
				DefaultPrice:      defaultConfig.GPO.DefaultPrice,
				MaxPrice:          defaultConfig.GPO.MaxPrice,
				LowUsageThreshold: defaultConfig.GPO.LowUsageThreshold,
				BlockGasLimit:     defaultConfig.GPO.BlockGasLimit,
			},
		},
		{
			args: []string{"--gpo.blocks", "5", "--gpo.transactions", "1", "--gpo.percentile", "2", "--gpo.defaultprice", "101000000000", "--gpo.maxprice", "400000000000", "--gpo.low-usage-threshold", "60", "--gpo.block-gas-limit", "10000000"},
			expConfig: harmonyconfig.GasPriceOracleConfig{
				Blocks:            5,
				Transactions:      1,
				Percentile:        2,
				DefaultPrice:      101 * denominations.Nano,
				MaxPrice:          400 * denominations.Nano,
				LowUsageThreshold: 60,
				BlockGasLimit:     10_000_000,
			},
		},
	}

	for i, test := range tests {
		ts := newFlagTestSuite(t, gpoFlags, applyGPOFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.GPO, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.GPO, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestCacheFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.CacheConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: harmonyconfig.CacheConfig{
				Disabled:        true, // based on network type
				TrieNodeLimit:   defaultCacheConfig.TrieNodeLimit,
				TriesInMemory:   defaultCacheConfig.TriesInMemory,
				TrieTimeLimit:   defaultCacheConfig.TrieTimeLimit,
				SnapshotLimit:   defaultCacheConfig.SnapshotLimit,
				SnapshotWait:    defaultCacheConfig.SnapshotWait,
				Preimages:       defaultCacheConfig.Preimages, // based on network type
				SnapshotNoBuild: defaultCacheConfig.SnapshotNoBuild,
			},
		},
		{
			args: []string{"--cache.disabled=true", "--cache.trie_node_limit", "512", "--cache.tries_in_memory", "256", "--cache.preimages=false", "--cache.snapshot_limit", "512", "--cache.snapshot_no_build=true", "--cache.snapshot_wait=false"},
			expConfig: harmonyconfig.CacheConfig{
				Disabled:        true,
				TrieNodeLimit:   512,
				TriesInMemory:   256,
				TrieTimeLimit:   2 * time.Minute,
				SnapshotLimit:   512,
				SnapshotWait:    false,
				Preimages:       false,
				SnapshotNoBuild: true,
			},
		},
	}

	for i, test := range tests {
		ts := newFlagTestSuite(t, cacheConfigFlags, applyCacheFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.Cache, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Cache, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestDevnetFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *harmonyconfig.DevnetConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--devnet.num-shard", "3", "--devnet.shard-size", "100",
				"--devnet.hmy-node-size", "60"},
			expConfig: &harmonyconfig.DevnetConfig{
				NumShards:   3,
				ShardSize:   100,
				HmyNodeSize: 60,
			},
		},
		{
			args: []string{"--dn_num_shards", "3", "--dn_shard_size", "100", "--dn_hmy_size",
				"60"},
			expConfig: &harmonyconfig.DevnetConfig{
				NumShards:   3,
				ShardSize:   100,
				HmyNodeSize: 60,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, devnetFlags, applyDevnetFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.Devnet, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Devnet, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestRevertFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *harmonyconfig.RevertConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--revert.beacon"},
			expConfig: &harmonyconfig.RevertConfig{
				RevertBeacon: true,
				RevertTo:     defaultRevertConfig.RevertTo,
				RevertBefore: defaultRevertConfig.RevertBefore,
			},
		},
		{
			args: []string{"--revert.beacon", "--revert.to", "100", "--revert.do-before", "10000"},
			expConfig: &harmonyconfig.RevertConfig{
				RevertBeacon: true,
				RevertTo:     100,
				RevertBefore: 10000,
			},
		},
		{
			args: []string{"--revert_beacon", "--do_revert_before", "10000", "--revert_to", "100"},
			expConfig: &harmonyconfig.RevertConfig{
				RevertBeacon: true,
				RevertTo:     100,
				RevertBefore: 10000,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, revertFlags, applyRevertFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.Revert, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Revert, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestPreimageFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *harmonyconfig.PreimageConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--preimage.import", "/path/to/source.csv"},
			expConfig: &harmonyconfig.PreimageConfig{
				ImportFrom:    "/path/to/source.csv",
				ExportTo:      defaultPreimageConfig.ExportTo,
				GenerateStart: defaultPreimageConfig.GenerateStart,
				GenerateEnd:   defaultPreimageConfig.GenerateEnd,
			},
		},
		{
			args: []string{"--preimage.export", "/path/to/destination.csv"},
			expConfig: &harmonyconfig.PreimageConfig{
				ImportFrom:    defaultPreimageConfig.ImportFrom,
				ExportTo:      "/path/to/destination.csv",
				GenerateStart: defaultPreimageConfig.GenerateStart,
				GenerateEnd:   defaultPreimageConfig.GenerateEnd,
			},
		},
		{
			args: []string{"--preimage.start", "1"},
			expConfig: &harmonyconfig.PreimageConfig{
				ImportFrom:    defaultPreimageConfig.ImportFrom,
				ExportTo:      defaultPreimageConfig.ExportTo,
				GenerateStart: 1,
				GenerateEnd:   defaultPreimageConfig.GenerateEnd,
			},
		},
		{
			args: []string{"--preimage.end", "2"},
			expConfig: &harmonyconfig.PreimageConfig{
				ImportFrom:    defaultPreimageConfig.ImportFrom,
				ExportTo:      defaultPreimageConfig.ExportTo,
				GenerateStart: defaultPreimageConfig.GenerateStart,
				GenerateEnd:   2,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, preimageFlags, applyPreimageFlags)
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.Preimage, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Preimage, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestDNSSyncFlags(t *testing.T) {
	tests := []struct {
		args      []string
		network   string
		expConfig harmonyconfig.DnsSync
		expErr    error
	}{
		{
			args:      []string{},
			network:   "mainnet",
			expConfig: getDefaultDNSSyncConfig(nodeconfig.Mainnet),
		},
		{
			args:      []string{"--sync.legacy.server", "--sync.legacy.client"},
			network:   "mainnet",
			expConfig: getDefaultDNSSyncConfig(nodeconfig.Mainnet),
		},
		{
			args:    []string{"--sync.legacy.server", "--sync.legacy.client"},
			network: "testnet",
			expConfig: func() harmonyconfig.DnsSync {
				cfg := getDefaultDNSSyncConfig(nodeconfig.Mainnet)
				cfg.Client = true
				cfg.Server = true
				return cfg
			}(),
		},
		{
			args:      []string{"--dns.server", "--dns.client"},
			network:   "mainnet",
			expConfig: getDefaultDNSSyncConfig(nodeconfig.Mainnet),
		},
	}

	for i, test := range tests {
		ts := newFlagTestSuite(t, dnsSyncFlags, func(command *cobra.Command, config *harmonyconfig.HarmonyConfig) {
			config.Network.NetworkType = test.network
			applyDNSSyncFlags(command, config)
		})
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.DNSSync, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.DNSSync, test.expConfig)
		}

		ts.tearDown()
	}
}
func TestSyncFlags(t *testing.T) {
	tests := []struct {
		args      []string
		network   string
		expConfig harmonyconfig.SyncConfig
		expErr    error
	}{
		{
			args: []string{"--sync", "--sync.downloader", "--sync.concurrency", "10", "--sync.min-peers", "10",
				"--sync.init-peers", "10", "--sync.disc.soft-low-cap", "10",
				"--sync.disc.hard-low-cap", "10", "--sync.disc.hi-cap", "10",
				"--sync.disc.batch", "10",
			},
			network: "mainnet",
			expConfig: func() harmonyconfig.SyncConfig {
				cfgSync := defaultMainnetSyncConfig
				cfgSync.Enabled = true
				cfgSync.Downloader = true
				cfgSync.StagedSync = false
				cfgSync.Concurrency = 10
				cfgSync.MinPeers = 10
				cfgSync.InitStreams = 10
				cfgSync.DiscSoftLowCap = 10
				cfgSync.DiscHardLowCap = 10
				cfgSync.DiscHighCap = 10
				cfgSync.DiscBatch = 10
				return cfgSync
			}(),
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, syncFlags, func(command *cobra.Command, config *harmonyconfig.HarmonyConfig) {
			applySyncFlags(command, config)
		})
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.Sync, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Sync, test.expConfig)
		}

		ts.tearDown()
	}
}

func TestShardDataFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig harmonyconfig.ShardDataConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.ShardData,
		},
		{
			args: []string{"--sharddata.enable",
				"--sharddata.disk_count", "8",
				"--sharddata.shard_count", "4",
				"--sharddata.cache_time", "10",
				"--sharddata.cache_size", "512",
			},
			expConfig: harmonyconfig.ShardDataConfig{
				EnableShardData: true,
				DiskCount:       8,
				ShardCount:      4,
				CacheTime:       10,
				CacheSize:       512,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, shardDataFlags, func(command *cobra.Command, config *harmonyconfig.HarmonyConfig) {
			applyShardDataFlags(command, config)
		})
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(hc.ShardData, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.ShardData, test.expConfig)
		}

		ts.tearDown()
	}
}

type flagTestSuite struct {
	t *testing.T

	cmd *cobra.Command
	hc  harmonyconfig.HarmonyConfig
}

func newFlagTestSuite(t *testing.T, flags []cli.Flag, applyFlags func(*cobra.Command, *harmonyconfig.HarmonyConfig)) *flagTestSuite {
	cli.SetParseErrorHandle(func(err error) { t.Fatal(err) })

	ts := &flagTestSuite{hc: getDefaultHmyConfigCopy(defNetworkType)}
	ts.cmd = makeTestCommand(func(cmd *cobra.Command, args []string) {
		applyFlags(cmd, &ts.hc)
	})
	if err := cli.RegisterFlags(ts.cmd, flags); err != nil {
		t.Fatal(err)
	}

	return ts
}

func (ts *flagTestSuite) run(args []string) (harmonyconfig.HarmonyConfig, error) {
	ts.cmd.SetArgs(args)
	err := ts.cmd.Execute()
	return ts.hc, err
}

func (ts *flagTestSuite) tearDown() {
	cli.SetParseErrorHandle(func(error) {})
}

func makeTestCommand(run func(cmd *cobra.Command, args []string)) *cobra.Command {
	return &cobra.Command{
		Use: "test",
		Run: run,
	}
}

func assertError(gotErr, expErr error) error {
	if (gotErr == nil) != (expErr == nil) {
		return fmt.Errorf("error unexpected [%v] / [%v]", gotErr, expErr)
	}
	if gotErr == nil {
		return nil
	}
	if !strings.Contains(gotErr.Error(), expErr.Error()) {
		return fmt.Errorf("error unexpected [%v] / [%v]", gotErr, expErr)
	}
	return nil
}

func intPtr(i int) *int {
	return &i
}
