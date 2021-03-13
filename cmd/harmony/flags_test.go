package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/spf13/cobra"
)

var (
	trueBool = true
)

func TestHarmonyFlags(t *testing.T) {
	tests := []struct {
		argStr    string
		expConfig harmonyConfig
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
			expConfig: harmonyConfig{
				Version: tomlConfigVersion,
				General: generalConfig{
					NodeType:   "validator",
					NoStaking:  false,
					ShardID:    -1,
					IsArchival: false,
					DataDir:    "./",
				},
				Network: networkConfig{
					NetworkType: "mainnet",
					BootNodes: []string{
						"/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
						"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9",
						"/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX",
						"/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj",
					},
					DNSZone: "t.hmny.io",
					DNSPort: 9000,
				},
				P2P: p2pConfig{
					Port:    9000,
					IP:      defaultConfig.P2P.IP,
					KeyFile: defaultConfig.P2P.KeyFile,
				},
				HTTP: httpConfig{
					Enabled:        true,
					IP:             "127.0.0.1",
					Port:           9500,
					RosettaEnabled: false,
					RosettaPort:    9700,
				},
				WS: wsConfig{
					Enabled: true,
					IP:      "127.0.0.1",
					Port:    9800,
				},
				Consensus: &consensusConfig{
					MinPeers:     6,
					AggregateSig: true,
				},
				BLSKeys: blsConfig{
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
				TxPool: txPoolConfig{
					BlacklistFile: "./.hmy/blacklist.txt",
				},
				Pprof: pprofConfig{
					Enabled:    false,
					ListenAddr: "127.0.0.1:6060",
				},
				Log: logConfig{
					Folder:     "./latest",
					FileName:   "validator-8.8.8.8-9000.log",
					RotateSize: 100,
					Verbosity:  3,
					Context: &logContext{
						IP:   "8.8.8.8",
						Port: 9000,
					},
				},
				Sys: &sysConfig{
					NtpServer: defaultSysConfig.NtpServer,
				},
				Legacy: &legacyConfig{
					TPBroadcastInvalidTxn: &trueBool,
				},
				Prometheus: &prometheusConfig{
					Enabled:    true,
					IP:         "0.0.0.0",
					Port:       9900,
					EnablePush: true,
					Gateway:    "https://gateway.harmony.one",
				},
				Sync: defaultMainnetSyncConfig,
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
		expConfig generalConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: generalConfig{
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
			expConfig: generalConfig{
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
			expConfig: generalConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: true,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--staking=false", "--is_archival=false"},
			expConfig: generalConfig{
				NodeType:   "validator",
				NoStaking:  true,
				ShardID:    -1,
				IsArchival: false,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.shard", "0"},
			expConfig: generalConfig{
				NodeType:   "explorer",
				NoStaking:  false,
				ShardID:    0,
				IsArchival: true,
				DataDir:    "./",
			},
		},
		{
			args: []string{"--run", "explorer", "--run.shard", "0", "--run.archive=false"},
			expConfig: generalConfig{
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
		expConfig networkConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: networkConfig{
				NetworkType:   defNetworkType,
				BootNodes:     nodeconfig.GetDefaultBootNodes(defNetworkType),
				LegacySyncing: false,
				DNSZone:       nodeconfig.GetDefaultDNSZone(defNetworkType),
				DNSPort:       nodeconfig.GetDefaultDNSPort(defNetworkType),
			},
		},
		{
			args: []string{"-n", "stn"},
			expConfig: networkConfig{
				NetworkType:   nodeconfig.Stressnet,
				BootNodes:     nodeconfig.GetDefaultBootNodes(nodeconfig.Stressnet),
				LegacySyncing: false,
				DNSZone:       nodeconfig.GetDefaultDNSZone(nodeconfig.Stressnet),
				DNSPort:       nodeconfig.GetDefaultDNSPort(nodeconfig.Stressnet),
			},
		},
		{
			args: []string{"--network", "stk", "--bootnodes", "1,2,3,4", "--dns.zone", "8.8.8.8",
				"--dns.port", "9001"},
			expConfig: networkConfig{
				NetworkType:   "pangaea",
				BootNodes:     []string{"1", "2", "3", "4"},
				LegacySyncing: false,
				DNSZone:       "8.8.8.8",
				DNSPort:       9001,
			},
		},
		{
			args: []string{"--network_type", "stk", "--bootnodes", "1,2,3,4", "--dns_zone", "8.8.8.8",
				"--dns_port", "9001"},
			expConfig: networkConfig{
				NetworkType:   "pangaea",
				BootNodes:     []string{"1", "2", "3", "4"},
				LegacySyncing: false,
				DNSZone:       "8.8.8.8",
				DNSPort:       9001,
			},
		},
		{
			args: []string{"--dns=false"},
			expConfig: networkConfig{
				NetworkType:   defNetworkType,
				BootNodes:     nodeconfig.GetDefaultBootNodes(defNetworkType),
				LegacySyncing: true,
				DNSZone:       nodeconfig.GetDefaultDNSZone(defNetworkType),
				DNSPort:       nodeconfig.GetDefaultDNSPort(defNetworkType),
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, networkFlags, func(cmd *cobra.Command, config *harmonyConfig) {
			// This is the network related logic in function getHarmonyConfig
			nt := getNetworkType(cmd)
			config.Network = getDefaultNetworkConfig(nt)
			applyNetworkFlags(cmd, config)
		})

		got, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if !reflect.DeepEqual(got.Network, test.expConfig) {
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, got.Network, test.expConfig)
		}
		ts.tearDown()
	}
}

var defDataStore = ".dht-127.0.0.1"

func TestP2PFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig p2pConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.P2P,
		},
		{
			args: []string{"--p2p.port", "9001", "--p2p.keyfile", "./key.file", "--p2p.dht.datastore",
				defDataStore},
			expConfig: p2pConfig{
				Port:         9001,
				IP:           nodeconfig.DefaultPublicListenIP,
				KeyFile:      "./key.file",
				DHTDataStore: &defDataStore,
			},
		},
		{
			args: []string{"--port", "9001", "--key", "./key.file"},
			expConfig: p2pConfig{
				Port:    9001,
				IP:      nodeconfig.DefaultPublicListenIP,
				KeyFile: "./key.file",
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(p2pFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyConfig) {
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
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, got.Network, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestRPCFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig httpConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.HTTP,
		},
		{
			args: []string{"--http=false"},
			expConfig: httpConfig{
				Enabled:        false,
				RosettaEnabled: false,
				IP:             defaultConfig.HTTP.IP,
				Port:           defaultConfig.HTTP.Port,
				RosettaPort:    defaultConfig.HTTP.RosettaPort,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001"},
			expConfig: httpConfig{
				Enabled:        true,
				RosettaEnabled: false,
				IP:             "8.8.8.8",
				Port:           9001,
				RosettaPort:    defaultConfig.HTTP.RosettaPort,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001", "--http.rosetta.port", "10001"},
			expConfig: httpConfig{
				Enabled:        true,
				RosettaEnabled: true,
				IP:             "8.8.8.8",
				Port:           9001,
				RosettaPort:    10001,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.rosetta.port", "10001"},
			expConfig: httpConfig{
				Enabled:        true,
				RosettaEnabled: true,
				IP:             "8.8.8.8",
				Port:           defaultConfig.HTTP.Port,
				RosettaPort:    10001,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: httpConfig{
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
			func(cmd *cobra.Command, config *harmonyConfig) {
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
		expConfig wsConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.WS,
		},
		{
			args: []string{"--ws=false"},
			expConfig: wsConfig{
				Enabled: false,
				IP:      defaultConfig.WS.IP,
				Port:    defaultConfig.WS.Port,
			},
		},
		{
			args: []string{"--ws", "--ws.ip", "8.8.8.8", "--ws.port", "9001"},
			expConfig: wsConfig{
				Enabled: true,
				IP:      "8.8.8.8",
				Port:    9001,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: wsConfig{
				Enabled: true,
				IP:      nodeconfig.DefaultPublicListenIP,
				Port:    9801,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(wsFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyConfig) {
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
		expConfig rpcOptConfig
	}{
		{
			args: []string{"--rpc.debug"},
			expConfig: rpcOptConfig{
				DebugEnabled: true,
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
		expConfig blsConfig
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
			expConfig: blsConfig{
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
			expConfig: blsConfig{
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
			expConfig: blsConfig{
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
		expConfig *consensusConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--consensus.min-peers", "10", "--consensus.aggregate-sig=false"},
			expConfig: &consensusConfig{
				MinPeers:     10,
				AggregateSig: false,
			},
		},
		{
			args: []string{"--delay_commit", "10ms", "--block_period", "5", "--min_peers", "10",
				"--consensus.aggregate-sig=true"},
			expConfig: &consensusConfig{
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
		expConfig txPoolConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: txPoolConfig{
				BlacklistFile: defaultConfig.TxPool.BlacklistFile,
			},
		},
		{
			args: []string{"--txpool.blacklist", "blacklist.file"},
			expConfig: txPoolConfig{
				BlacklistFile: "blacklist.file",
			},
		},
		{
			args: []string{"--blacklist", "blacklist.file"},
			expConfig: txPoolConfig{
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

		if !reflect.DeepEqual(hc.TxPool, test.expConfig) {
			t.Errorf("Test %v: unexpected config\n\t%+v\n\t%+v", i, hc.TxPool, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestPprofFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig pprofConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.Pprof,
		},
		{
			args: []string{"--pprof"},
			expConfig: pprofConfig{
				Enabled:    true,
				ListenAddr: defaultConfig.Pprof.ListenAddr,
			},
		},
		{
			args: []string{"--pprof.addr", "8.8.8.8:9001"},
			expConfig: pprofConfig{
				Enabled:    true,
				ListenAddr: "8.8.8.8:9001",
			},
		},
		{
			args: []string{"--pprof=false", "--pprof.addr", "8.8.8.8:9001"},
			expConfig: pprofConfig{
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
		if !reflect.DeepEqual(hc.Pprof, test.expConfig) {
			t.Errorf("Test %v: unexpected config\n\t%+v\n\t%+v", i, hc.Pprof, test.expConfig)
		}
		ts.tearDown()
	}
}

func TestLogFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig logConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.Log,
		},
		{
			args: []string{"--log.dir", "latest_log", "--log.max-size", "10", "--log.name", "harmony.log",
				"--log.verb", "5"},
			expConfig: logConfig{
				Folder:     "latest_log",
				FileName:   "harmony.log",
				RotateSize: 10,
				Verbosity:  5,
				Context:    nil,
			},
		},
		{
			args: []string{"--log.ctx.ip", "8.8.8.8", "--log.ctx.port", "9001"},
			expConfig: logConfig{
				Folder:     defaultConfig.Log.Folder,
				FileName:   defaultConfig.Log.FileName,
				RotateSize: defaultConfig.Log.RotateSize,
				Verbosity:  defaultConfig.Log.Verbosity,
				Context: &logContext{
					IP:   "8.8.8.8",
					Port: 9001,
				},
			},
		},
		{
			args: []string{"--log_folder", "latest_log", "--log_max_size", "10", "--verbosity",
				"5", "--ip", "8.8.8.8", "--port", "9001"},
			expConfig: logConfig{
				Folder:     "latest_log",
				FileName:   "validator-8.8.8.8-9001.log",
				RotateSize: 10,
				Verbosity:  5,
				Context: &logContext{
					IP:   "8.8.8.8",
					Port: 9001,
				},
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(logFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyConfig) {
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
		expConfig *sysConfig
		expErr    error
	}{
		{
			args: []string{},
			expConfig: &sysConfig{
				NtpServer: defaultSysConfig.NtpServer,
			},
		},
		{
			args: []string{"--sys.ntp", "0.pool.ntp.org"},
			expConfig: &sysConfig{
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

func TestDevnetFlags(t *testing.T) {
	tests := []struct {
		args      []string
		expConfig *devnetConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--devnet.num-shard", "3", "--devnet.shard-size", "100",
				"--devnet.hmy-node-size", "60"},
			expConfig: &devnetConfig{
				NumShards:   3,
				ShardSize:   100,
				HmyNodeSize: 60,
			},
		},
		{
			args: []string{"--dn_num_shards", "3", "--dn_shard_size", "100", "--dn_hmy_size",
				"60"},
			expConfig: &devnetConfig{
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
		expConfig *revertConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: nil,
		},
		{
			args: []string{"--revert.beacon"},
			expConfig: &revertConfig{
				RevertBeacon: true,
				RevertTo:     defaultRevertConfig.RevertTo,
				RevertBefore: defaultRevertConfig.RevertBefore,
			},
		},
		{
			args: []string{"--revert.beacon", "--revert.to", "100", "--revert.do-before", "10000"},
			expConfig: &revertConfig{
				RevertBeacon: true,
				RevertTo:     100,
				RevertBefore: 10000,
			},
		},
		{
			args: []string{"--revert_beacon", "--do_revert_before", "10000", "--revert_to", "100"},
			expConfig: &revertConfig{
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

func TestSyncFlags(t *testing.T) {
	tests := []struct {
		args      []string
		network   string
		expConfig syncConfig
		expErr    error
	}{
		{
			args:      []string{},
			network:   "mainnet",
			expConfig: defaultMainnetSyncConfig,
		},
		{
			args:    []string{"--sync.legacy.server", "--sync.legacy.client"},
			network: "mainnet",
			expConfig: func() syncConfig {
				cfg := defaultMainnetSyncConfig
				cfg.LegacyClient = true
				cfg.LegacyServer = true
				return cfg
			}(),
		},
		{
			args:    []string{"--sync.legacy.server", "--sync.legacy.client"},
			network: "testnet",
			expConfig: func() syncConfig {
				cfg := defaultTestNetSyncConfig
				cfg.LegacyClient = true
				cfg.LegacyServer = true
				return cfg
			}(),
		},
		{
			args: []string{"--sync.concurrency", "10", "--sync.min-peers", "10",
				"--sync.init-peers", "10", "--sync.disc.soft-low-cap", "10",
				"--sync.disc.hard-low-cap", "10", "--sync.disc.hi-cap", "10",
				"--sync.disc.batch", "10",
			},
			network: "mainnet",
			expConfig: func() syncConfig {
				cfg := defaultMainnetSyncConfig
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
		ts := newFlagTestSuite(t, syncFlags, func(command *cobra.Command, config *harmonyConfig) {
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
		if !reflect.DeepEqual(hc.Sync, test.expConfig) {
			t.Errorf("Test %v:\n\t%+v\n\t%+v", i, hc.Sync, test.expConfig)
		}
		ts.tearDown()
	}
}

type flagTestSuite struct {
	t *testing.T

	cmd *cobra.Command
	hc  harmonyConfig
}

func newFlagTestSuite(t *testing.T, flags []cli.Flag, applyFlags func(*cobra.Command, *harmonyConfig)) *flagTestSuite {
	cli.SetParseErrorHandle(func(err error) { t.Fatal(err) })

	ts := &flagTestSuite{hc: defaultConfig}
	ts.cmd = makeTestCommand(func(cmd *cobra.Command, args []string) {
		applyFlags(cmd, &ts.hc)
	})
	if err := cli.RegisterFlags(ts.cmd, flags); err != nil {
		t.Fatal(err)
	}

	return ts
}

func (ts *flagTestSuite) run(args []string) (harmonyConfig, error) {
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
