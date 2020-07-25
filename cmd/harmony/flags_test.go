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
			args: []string{"--p2p.ip", "8.8.8.8"},
			expConfig: p2pConfig{
				IP:      "8.8.8.8",
				Port:    defaultConfig.P2P.Port,
				KeyFile: defaultConfig.P2P.KeyFile,
			},
		},
		{
			args: []string{"--p2p.ip", "8.8.8.8", "--p2p.port", "9001", "--p2p.keyfile", "./key.file"},
			expConfig: p2pConfig{
				IP:      "8.8.8.8",
				Port:    9001,
				KeyFile: "./key.file",
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--key", "./key.file"},
			expConfig: p2pConfig{
				IP:      "8.8.8.8",
				Port:    9001,
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
		expConfig rpcConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.RPC,
		},
		{
			args: []string{"--http=false"},
			expConfig: rpcConfig{
				Enabled: false,
				IP:      defaultConfig.RPC.IP,
				Port:    defaultConfig.RPC.Port,
			},
		},
		{
			args: []string{"--http.ip", "8.8.8.8", "--http.port", "9001"},
			expConfig: rpcConfig{
				Enabled: true,
				IP:      "8.8.8.8",
				Port:    9001,
			},
		},
		{
			args: []string{"--ip", "8.8.8.8", "--port", "9001", "--public_rpc"},
			expConfig: rpcConfig{
				Enabled: true,
				IP:      "8.8.8.8",
				Port:    9001,
			},
		},
	}
	for i, test := range tests {
		ts := newFlagTestSuite(t, append(rpcFlags, legacyMiscFlags...),
			func(cmd *cobra.Command, config *harmonyConfig) {
				applyLegacyMiscFlags(cmd, config)
				applyRPCFlags(cmd, config)
			},
		)

		hc, err := ts.run(test.args)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if !reflect.DeepEqual(hc.RPC, test.expConfig) {
			t.Errorf("Test %v: unexpected config: \n\t%+v\n\t%+v", i, hc.RPC, test.expConfig)
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
			args: []string{"--ip", "8.8.8.8", "--port", "9001"},
			expConfig: wsConfig{
				Enabled: true,
				IP:      "8.8.8.8",
				Port:    9001,
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
				KMSEnabled:       true,
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
				KMSEnabled:       true,
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
		expConfig consensusConfig
		expErr    error
	}{
		{
			args:      []string{},
			expConfig: defaultConfig.Consensus,
		},
		{
			args: []string{"--consensus.block-time", "5s", "--consensus.delay-commit", "10ms",
				"--consensus.min-peers", "10"},
			expConfig: consensusConfig{
				DelayCommit: "10ms",
				BlockTime:   "5s",
				MinPeers:    10,
			},
		},
		{
			args: []string{"--delay_commit", "10ms", "--block_period", "5", "--min_peers", "10"},
			expConfig: consensusConfig{
				DelayCommit: "10ms",
				BlockTime:   "5s",
				MinPeers:    10,
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
				BlacklistFile:      defaultConfig.TxPool.BlacklistFile,
				BroadcastInvalidTx: defaultConfig.TxPool.BroadcastInvalidTx,
			},
		},
		{
			args: []string{"--txpool.blacklist", "blacklist.file", "--txpool.broadcast-invalid-tx"},
			expConfig: txPoolConfig{
				BlacklistFile:      "blacklist.file",
				BroadcastInvalidTx: true,
			},
		},
		{
			args: []string{"--blacklist", "blacklist.file", "--broadcast_invalid_tx"},
			expConfig: txPoolConfig{
				BlacklistFile:      "blacklist.file",
				BroadcastInvalidTx: true,
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
