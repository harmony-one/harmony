package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/cmd/harmony/config"
	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var (
	trueBool = true
)

func TestHarmonyFlags(t *testing.T) {
	tests := []struct {
		argStr    string
		expConfig config.HarmonyConfig
	}{
		{
			// running staking command from legacy node.sh
			argStr: "--bootnodes /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv," +
				"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9,/ip4/13.113.101." +
				"219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX,/ip4/99.81.170.167/tcp/12019/p" +
				"2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj --ip 8.8.8.8 --port 9000 --network_type=mainn" +
				"et --dns_zone=t.hmny.io --blacklist=./.hmy/blacklist.txt --min_peers=6 --max_bls_keys_per_node=" +
				"10 --broadcast_invalid_tx=true --verbosity=3 --is_archival=false --shard_id=-1 --staking=true -" +
				"-aws-config-source file:config.json",
			expConfig: config.HarmonyConfig{
				Version: config.TOMLConfigVersion,
				General: config.GeneralConfig{
					NodeType:   "validator",
					NoStaking:  false,
					ShardID:    -1,
					IsArchival: false,
					DataDir:    "./",
				},
				Network: config.NetworkConfig{
					NetworkType: "mainnet",
					BootNodes: []string{
						"/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
						"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9",
						"/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX",
						"/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj",
					},
				},
				P2P: config.P2PConfig{
					Port:    9000,
					IP:      config.DefaultConfig.P2P.IP,
					KeyFile: config.DefaultConfig.P2P.KeyFile,
				},
				HTTP: config.HTTPConfig{
					Enabled:        true,
					IP:             "127.0.0.1",
					Port:           9500,
					RosettaEnabled: false,
					RosettaPort:    9700,
				},
				WS: config.WSConfig{
					Enabled: true,
					IP:      "127.0.0.1",
					Port:    9800,
				},
				Consensus: &config.ConsensusConfig{
					MinPeers:     6,
					AggregateSig: true,
				},
				BLSKeys: config.BLSConfig{
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
				TxPool: config.TxPoolConfig{
					BlacklistFile: "./.hmy/blacklist.txt",
				},
				Pprof: config.PprofConfig{
					Enabled:    false,
					ListenAddr: "127.0.0.1:6060",
				},
				Log: config.LogConfig{
					Folder:     "./latest",
					FileName:   "validator-8.8.8.8-9000.log",
					RotateSize: 100,
					Verbosity:  3,
					Context: &config.LogContext{
						IP:   "8.8.8.8",
						Port: 9000,
					},
				},
				Sys: &config.SysConfig{
					NtpServer: config.DefaultSysConfig.NtpServer,
				},
				Legacy: &config.LegacyConfig{
					TPBroadcastInvalidTxn: &trueBool,
				},
				Prometheus: &config.PrometheusConfig{
					Enabled:    true,
					IP:         "0.0.0.0",
					Port:       9900,
					EnablePush: true,
					Gateway:    "https://gateway.harmony.one",
				},
				Sync: config.DefaultMainnetSyncConfig,
				DNSSync: config.DNSSyncConfig{
					DNSZone:      "t.hmny.io",
					DNSPort:      9000,
					LegacyServer: true,
					LegacyClient: true,
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
		assert.Equal(t, test.expConfig, hc, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestGeneralFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.GeneralConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: config.GeneralConfig{
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
			expConfig: config.GeneralConfig{
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
			expConfig: config.GeneralConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: true,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--staking=false", "--is_archival=false"},
			expConfig: config.GeneralConfig{
				NodeType:   "validator",
				NoStaking:  true,
				ShardID:    -1,
				IsArchival: false,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.shard", "0"},
			expConfig: config.GeneralConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: true,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.shard", "0", "--run.archive=false"},
			expConfig: config.GeneralConfig{
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
		assert.Equal(t, test.expConfig, got.General, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestNetworkFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.NetworkConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: config.NetworkConfig{
				NetworkType: config.DefNetworkType,
				BootNodes:   nodeconfig.GetDefaultBootNodes(config.DefNetworkType),
			},
		},
		{
			args: []string{"-n", "stn"},
			expConfig: config.NetworkConfig{
				NetworkType: nodeconfig.Stressnet,
				BootNodes:   nodeconfig.GetDefaultBootNodes(nodeconfig.Stressnet),
			},
		},
		{
			args: []string{"--network", "stk", "--bootnodes", "1,2,3,4"},
			expConfig: config.NetworkConfig{
				NetworkType: "pangaea",
				BootNodes:   []string{"1", "2", "3", "4"},
			},
		},
		{
			args: []string{"--network_type", "stk", "--bootnodes", "1,2,3,4"},
			expConfig: config.NetworkConfig{
				NetworkType: "pangaea",
				BootNodes:   []string{"1", "2", "3", "4"},
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, networkFlags, func(cmd *cobra.Command, cfg *config.HarmonyConfig) {
			// This is the network related logic in function getHarmonyConfig
			nt := getNetworkType(cmd)
			cfg.Network = config.GetDefaultNetworkConfig(nt)
			applyNetworkFlags(cmd, cfg)
		})

		got, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		assert.Equal(t, test.expConfig, got.Network, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

var defDataStore = ".dht-127.0.0.1"

func TestP2PFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.P2PConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: config.DefaultConfig.P2P,
		},
		{
			args: []string{"--p2p.port", "9001", "--p2p.keyfile", "./key.file", "--p2p.dht.datastore",
				defDataStore},
			expConfig: config.P2PConfig{
				Port:         9001,
				IP:           nodeconfig.DefaultPublicListenIP,
				KeyFile:      "./key.file",
				DHTDataStore: &defDataStore,
			},
		},
		{
			args: []string{"--port", "9001", "--key", "./key.file"},
			expConfig: config.P2PConfig{
				Port:    9001,
				IP:      nodeconfig.DefaultPublicListenIP,
				KeyFile: "./key.file",
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(p2pFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *config.HarmonyConfig) {
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
		assert.Equal(t, test.expConfig, got.P2P, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestRPCFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.HTTPConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: config.DefaultConfig.HTTP,
		},
		{
			args: []string{"--http=false"},
			expConfig: config.HTTPConfig{
				Enabled:        false,
				RosettaEnabled: false,
				IP:             config.DefaultConfig.HTTP.IP,
				Port:           config.DefaultConfig.HTTP.Port,
				RosettaPort:    config.DefaultConfig.HTTP.RosettaPort,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001"},
			expConfig: config.HTTPConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             "8.8.8.8",
				Port:           9001,
				RosettaPort:    config.DefaultConfig.HTTP.RosettaPort,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001", "--http.rosetta.port", "10001"},
			expConfig: config.HTTPConfig{
				Enabled:        true,
				RosettaEnabled: true,
				IP:             "8.8.8.8",
				Port:           9001,
				RosettaPort:    10001,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.rosetta.port", "10001"},
			expConfig: config.HTTPConfig{
				Enabled:        true,
				RosettaEnabled: true,
				IP:             "8.8.8.8",
				Port:           config.DefaultConfig.HTTP.Port,
				RosettaPort:    10001,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: config.HTTPConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             nodeconfig.DefaultPublicListenIP,
				Port:           9501,
				RosettaPort:    9701,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(httpFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *config.HarmonyConfig) {
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

		assert.Equal(t, test.expConfig, hc.HTTP, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestWSFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.WSConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: config.DefaultConfig.WS,
		},
		{
			args: []string{"--ws=false"},
			expConfig: config.WSConfig{
				Enabled: false,
				IP:      config.DefaultConfig.WS.IP,
				Port:    config.DefaultConfig.WS.Port,
			},
		},
		{
			args: []string{"--ws", "--ws.ip", "8.8.8.8", "--ws.port", "9001"},
			expConfig: config.WSConfig{
				Enabled: true,
				IP:      "8.8.8.8",
				Port:    9001,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: config.WSConfig{
				Enabled: true,
				IP:      nodeconfig.DefaultPublicListenIP,
				Port:    9801,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(wsFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *config.HarmonyConfig) {
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

		assert.Equal(t, test.expConfig, hc.WS, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestRPCOptFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.RPCOptConfig
	}{
		{
			args: []string{"--rpc.debug"},
			expConfig: config.RPCOptConfig{
				DebugEnabled: true,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, rpcOptFlags, applyRPCOptFlags)

		hc, _ := ts.run(test.args)

		assert.Equal(t, test.expConfig, hc.RPCOpt, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestBLSFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.BLSConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: config.DefaultConfig.BLSKeys,
		},
		{
			args: []string{"--bls.dir", "./blskeys", "--bls.keys", "key1,key2",
				"--bls.maxkeys", "8", "--bls.pass", "--bls.pass.src", "auto", "--bls.pass.save",
				"--bls.kms", "--bls.kms.src", "shared",
			},
			expConfig: config.BLSConfig{
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
			expConfig: config.BLSConfig{
				KeyDir:           config.DefaultConfig.BLSKeys.KeyDir,
				KeyFiles:         config.DefaultConfig.BLSKeys.KeyFiles,
				MaxKeys:          config.DefaultConfig.BLSKeys.MaxKeys,
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
			expConfig: config.BLSConfig{
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
		assert.Equal(t, test.expConfig, hc.BLSKeys, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestConsensusFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *config.ConsensusConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--consensus.min-peers", "10", "--consensus.aggregate-sig=false"},
			expConfig: &config.ConsensusConfig{
				MinPeers:     10,
				AggregateSig: false,
			},
		},
		{
			args: []string{"--delay_commit", "10ms", "--block_period", "5", "--min_peers", "10",
				"--consensus.aggregate-sig=true"},
			expConfig: &config.ConsensusConfig{
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
		assert.Equal(t, test.expConfig, hc.Consensus, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestTxPoolFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.TxPoolConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: config.TxPoolConfig{
				BlacklistFile: config.DefaultConfig.TxPool.BlacklistFile,
			},
		},
		{
			args: []string{"--txpool.blacklist", "blacklist.file"},
			expConfig: config.TxPoolConfig{
				BlacklistFile: "blacklist.file",
			},
		},
		{
			args: []string{"--blacklist", "blacklist.file"},
			expConfig: config.TxPoolConfig{
				BlacklistFile: "blacklist.file",
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

		assert.Equal(t, test.expConfig, hc.TxPool, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestPprofFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.PprofConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: config.DefaultConfig.Pprof,
		},
		{
			args: []string{"--pprof"},
			expConfig: config.PprofConfig{
				Enabled:    true,
				ListenAddr: config.DefaultConfig.Pprof.ListenAddr,
			},
		},
		{
			args: []string{"--pprof.addr", "8.8.8.8:9001"},
			expConfig: config.PprofConfig{
				Enabled:    true,
				ListenAddr: "8.8.8.8:9001",
			},
		},
		{
			args: []string{"--pprof=false", "--pprof.addr", "8.8.8.8:9001"},
			expConfig: config.PprofConfig{
				Enabled:    false,
				ListenAddr: "8.8.8.8:9001",
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
		assert.Equal(t, test.expConfig, hc.Pprof, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestLogFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig config.LogConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: config.DefaultConfig.Log,
		},
		{
			args: []string{"--log.dir", "latest_log", "--log.max-size", "10", "--log.name", "harmony.log",
				"--log.verb", "5"},
			expConfig: config.LogConfig{
				Folder:     "latest_log",
				FileName:   "harmony.log",
				RotateSize: 10,
				Verbosity:  5,
				Context:    nil,
			},
		},
		{
			args: []string{"--log.ctx.ip", "8.8.8.8", "--log.ctx.port", "9001"},
			expConfig: config.LogConfig{
				Folder:     config.DefaultConfig.Log.Folder,
				FileName:   config.DefaultConfig.Log.FileName,
				RotateSize: config.DefaultConfig.Log.RotateSize,
				Verbosity:  config.DefaultConfig.Log.Verbosity,
				Context: &config.LogContext{
					IP:   "8.8.8.8",
					Port: 9001,
				},
			},
		},
		{
			args: []string{"--log_folder", "latest_log", "--log_max_size", "10", "--verbosity",
				"5", "--ip", "8.8.8.8", "--port", "9001"},
			expConfig: config.LogConfig{
				Folder:     "latest_log",
				FileName:   "validator-8.8.8.8-9001.log",
				RotateSize: 10,
				Verbosity:  5,
				Context: &config.LogContext{
					IP:   "8.8.8.8",
					Port: 9001,
				},
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(logFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *config.HarmonyConfig) {
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

		assert.Equal(t, test.expConfig, hc.Log, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestSysFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *config.SysConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: &config.SysConfig{
				NtpServer: config.DefaultSysConfig.NtpServer,
			},
		},
		{
			args: []string{"--sys.ntp", "0.pool.ntp.org"},
			expConfig: &config.SysConfig{
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

		assert.Equal(t, test.expConfig, hc.Sys, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestDevnetFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *config.DevnetConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--devnet.num-shard", "3", "--devnet.shard-size", "100",
				"--devnet.hmy-node-size", "60"},
			expConfig: &config.DevnetConfig{
				NumShards:   3,
				ShardSize:   100,
				HmyNodeSize: 60,
			},
		},
		{
			args: []string{"--dn_num_shards", "3", "--dn_shard_size", "100", "--dn_hmy_size",
				"60"},
			expConfig: &config.DevnetConfig{
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

		assert.Equal(t, test.expConfig, hc.Devnet, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestRevertFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *config.RevertConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--revert.beacon"},
			expConfig: &config.RevertConfig{
				RevertBeacon: true,
				RevertTo:     config.DefaultRevertConfig.RevertTo,
				RevertBefore: config.DefaultRevertConfig.RevertBefore,
			},
		},
		{
			args: []string{"--revert.beacon", "--revert.to", "100", "--revert.do-before", "10000"},
			expConfig: &config.RevertConfig{
				RevertBeacon: true,
				RevertTo:     100,
				RevertBefore: 10000,
			},
		},
		{
			args: []string{"--revert_beacon", "--do_revert_before", "10000", "--revert_to", "100"},
			expConfig: &config.RevertConfig{
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
		assert.Equal(t, test.expConfig, hc.Revert, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func TestSyncFlags(t *testing.T) {
	tests := []struct {
		args      []string
		network   string
		expConfig config.SyncConfig
		expErr    error
	}{
		{
			args:      []string{},
			network:   "mainnet",
			expConfig: config.DefaultMainnetSyncConfig,
		},
		{
			args: []string{"--sync.downloader", "--sync.concurrency", "10", "--sync.min-peers", "10",
				"--sync.init-peers", "10", "--sync.disc.soft-low-cap", "10",
				"--sync.disc.hard-low-cap", "10", "--sync.disc.hi-cap", "10",
				"--sync.disc.batch", "10",
			},
			network: "mainnet",
			expConfig: func() config.SyncConfig {
				cfg := config.DefaultMainnetSyncConfig
				cfg.Downloader = true
				cfg.Concurrency = 10
				cfg.MinPeers = 10
				cfg.InitStreams = 10
				cfg.DiscSoftLowCap = 10
				cfg.DiscHardLowCap = 10
				cfg.DiscHighCap = 10
				cfg.DiscBatch = 10
				return cfg
			}(),
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, syncFlags, func(command *cobra.Command, config *config.HarmonyConfig) {
			config.Network.NetworkType = test.network
			applySyncFlags(command, config)
		})
		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		assert.Equal(t, test.expConfig, hc.Sync, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

type flagTestSuite struct {
	t *testing.T

	cmd *cobra.Command
	hc  config.HarmonyConfig
}

func TestDNSSyncFlags(t *testing.T) {
	tests := []struct {
		args      []string
		network   string
		expConfig config.DNSSyncConfig
		expErr    error
	}{
		{
			args:    []string{},
			network: "mainnet",
			expConfig: config.DNSSyncConfig{
				LegacySyncing: false,
				DNSZone:       nodeconfig.GetDefaultDNSZone(config.DefNetworkType),
				DNSPort:       nodeconfig.GetDefaultDNSPort(config.DefNetworkType),
				LegacyClient:  true,
				LegacyServer:  true,
			},
		},
		{
			args:    []string{"--sync.legacy.server", "--sync.legacy.client"},
			network: "mainnet",
			expConfig: func() config.DNSSyncConfig {
				cfg := config.GetDefaultDNSSyncConfig(nodeconfig.Mainnet)
				cfg.LegacyClient = true
				cfg.LegacyServer = true
				return cfg
			}(),
		},
		{
			args:    []string{},
			network: "stn",
			expConfig: config.DNSSyncConfig{
				LegacySyncing: false,
				DNSZone:       nodeconfig.GetDefaultDNSZone(nodeconfig.Stressnet),
				DNSPort:       nodeconfig.GetDefaultDNSPort(nodeconfig.Stressnet),
				LegacyClient:  false,
				LegacyServer:  true,
			},
		},
		{
			args:    []string{"--dns.zone", "8.8.8.8", "--dns.port", "9001"},
			network: "testnet",
			expConfig: config.DNSSyncConfig{
				LegacySyncing: false,
				DNSZone:       "8.8.8.8",
				DNSPort:       9001,
				LegacyClient:  true,
				LegacyServer:  true,
			},
		},
		{
			args:    []string{"--dns_zone", "8.8.8.8", "--dns_port", "9001"},
			network: "stk",
			expConfig: config.DNSSyncConfig{
				LegacySyncing: false,
				DNSZone:       "8.8.8.8",
				DNSPort:       9001,
				LegacyClient:  false,
				LegacyServer:  true,
			},
		},
		{
			args:    []string{"--dns=false"},
			network: "mainnet",
			expConfig: config.DNSSyncConfig{
				LegacySyncing: true,
				DNSZone:       nodeconfig.GetDefaultDNSZone(config.DefNetworkType),
				DNSPort:       nodeconfig.GetDefaultDNSPort(config.DefNetworkType),
				LegacyClient:  true,
				LegacyServer:  true,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, getRootFlags(), func(cmd *cobra.Command, cfg *config.HarmonyConfig) {
			cfg.DNSSync = config.GetDefaultDNSSyncConfig(config.ParseNetworkType(test.network))
			applyDNSSyncFlags(cmd, cfg)
		})

		got, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		assert.Equal(t, test.expConfig, got.DNSSync, "test %d: configs should be equal", i)
		ts.tearDown()
	}
}

func newFlagTestSuite(t *testing.T, flags []cli.Flag, applyFlags func(*cobra.Command, *config.HarmonyConfig)) *flagTestSuite {
	cli.SetParseErrorHandle(func(err error) { t.Fatal(err) })

	ts := &flagTestSuite{hc: config.DefaultConfig}
	ts.cmd = makeTestCommand(func(cmd *cobra.Command, args []string) {
		applyFlags(cmd, &ts.hc)
	})
	if err := cli.RegisterFlags(ts.cmd, flags); err != nil {
		t.Fatal(err)
	}

	return ts
}

func (ts *flagTestSuite) run(args []string) (config.HarmonyConfig, error) {
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
