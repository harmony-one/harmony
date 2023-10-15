package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/stretchr/testify/require"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

type testCfgOpt func(config *harmonyconfig.HarmonyConfig)

func makeTestConfig(nt nodeconfig.NetworkType, opt testCfgOpt) harmonyconfig.HarmonyConfig {
	cfg := getDefaultHmyConfigCopy(nt)
	if opt != nil {
		opt(&cfg)
	}
	return cfg
}

var testBaseDir = ".testdata"

func init() {
	if _, err := os.Stat(testBaseDir); os.IsNotExist(err) {
		os.MkdirAll(testBaseDir, 0777)
	}
}

func TestV1_0_4Config(t *testing.T) {
	testConfig := `
Version = "1.0.4"
[BLSKeys]
  KMSConfigFile = ""
  KMSConfigSrcType = "shared"
  KMSEnabled = false
  KeyDir = "./.hmy/blskeys"
  KeyFiles = []
  MaxKeys = 10
  PassEnabled = true
  PassFile = ""
  PassSrcType = "auto"
  SavePassphrase = false

[General]
  DataDir = "./"
  IsArchival = false
  NoStaking = false
  NodeType = "validator"
  ShardID = -1

[HTTP]
  Enabled = true
  IP = "127.0.0.1"
  Port = 9500

[Log]
  Console = false
  FileName = "harmony.log"
  Folder = "./latest"
  RotateSize = 100
  RotateCount = 0
  RotateMaxAge = 0
  Verbosity = 3

[Network]
  BootNodes = ["/dnsaddr/bootstrap.t.hmny.io"]
  DNSPort = 9000
  DNSZone = "t.hmny.io"
  LegacySyncing = false
  NetworkType = "mainnet"

[P2P]
  KeyFile = "./.hmykey"
  Port = 9000

[Pprof]
  Enabled = false
  ListenAddr = "127.0.0.1:6060"

[TxPool]
  BlacklistFile = "./.hmy/blacklist.txt"
  LocalAccountsFile = "./.hmy/locals.txt"
  AllowedTxsFile = "./.hmy/allowedtxs.txt"
  AccountQueue = 64
  GlobalQueue = 5120
  Lifetime = "30m"
  PriceBump = 1
  PriceLimit = 100e9

[Sync]
  Downloader = false
  Concurrency = 6
  DiscBatch = 8
  DiscHardLowCap = 6
  DiscHighCap = 128
  DiscSoftLowCap = 8
  InitStreams = 8
  LegacyClient = true
  LegacyServer = true
  MinPeers = 6

[ShardData]
  EnableShardData = false
  DiskCount = 8
  ShardCount = 4
  CacheTime = 10
  CacheSize = 512

[WS]
  Enabled = true
  IP = "127.0.0.1"
  Port = 9800`
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)
	file := filepath.Join(testDir, "test.config")
	err := os.WriteFile(file, []byte(testConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}
	config, migratedFrom, err := loadHarmonyConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	defConf := getDefaultHmyConfigCopy(nodeconfig.Mainnet)
	if config.HTTP.RosettaEnabled {
		t.Errorf("Expected rosetta http server to be disabled when loading old config")
	}
	if config.General.IsOffline {
		t.Errorf("Expect node to de online when loading old config")
	}
	if config.P2P.IP != defConf.P2P.IP {
		t.Errorf("Expect default p2p IP if old config is provided")
	}
	if migratedFrom != "1.0.4" {
		t.Errorf("Expected config version: 1.0.4, not %v", config.Version)
	}
	config.Version = defConf.Version // Shortcut for testing, value checked above
	require.Equal(t, config, defConf)
}

func TestPersistConfig(t *testing.T) {
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)

	tests := []struct {
		config harmonyconfig.HarmonyConfig
	}{
		{
			config: makeTestConfig("mainnet", nil),
		},
		{
			config: makeTestConfig("devnet", nil),
		},
		{
			config: makeTestConfig("mainnet", func(cfg *harmonyconfig.HarmonyConfig) {
				consensus := getDefaultConsensusConfigCopy()
				cfg.Consensus = &consensus

				devnet := getDefaultDevnetConfigCopy()
				cfg.Devnet = &devnet

				revert := getDefaultRevertConfigCopy()
				cfg.Revert = &revert

				webHook := "web hook"
				cfg.Legacy = &harmonyconfig.LegacyConfig{
					WebHookConfig:         &webHook,
					TPBroadcastInvalidTxn: &trueBool,
				}

				logCtx := getDefaultLogContextCopy()
				cfg.Log.Context = &logCtx
			}),
		},
	}
	for i, test := range tests {
		file := filepath.Join(testDir, fmt.Sprintf("%d.conf", i))

		if err := writeHarmonyConfigToFile(test.config, file); err != nil {
			t.Fatal(err)
		}
		config, _, err := loadHarmonyConfig(file)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(config, test.config) {
			t.Errorf("Test %v: unexpected config \n\t%+v \n\t%+v", i, config, test.config)
		}
	}
}
