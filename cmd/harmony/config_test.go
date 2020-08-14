package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

type testCfgOpt func(config *harmonyConfig)

func makeTestConfig(nt nodeconfig.NetworkType, opt testCfgOpt) harmonyConfig {
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

func TestV1_0_0Config(t *testing.T) {
	testConfig := `Version = "1.0.0"

[BLSKeys]
  KMSConfigFile = ""
  KMSConfigSrcType = "shared"
  KMSEnabled = true
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
  FileName = "harmony.log"
  Folder = "./latest"
  RotateSize = 100
  Verbosity = 3

[Network]
  BootNodes = ["/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv","/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9","/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX","/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj"]
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

[WS]
  Enabled = true
  IP = "127.0.0.1"
  Port = 9800`
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)
	file := filepath.Join(testDir, "test.config")
	err := ioutil.WriteFile(file, []byte(testConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}
	config, err := loadHarmonyConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTP.RosettaEnabled {
		t.Errorf("Expected rosetta http server to be disabled when loading old config")
	}
	if config.Version != "1.0.0" {
		t.Errorf("Expected config version: 1.0.0, not %v", config.Version)
	}
	config.Version = defaultConfig.Version // Shortcut for testing, value checked above
	if !reflect.DeepEqual(config, defaultConfig) {
		t.Errorf("Unexpected config \n\t%+v \n\t%+v", config, defaultConfig)
	}
}

func TestPersistConfig(t *testing.T) {
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)

	tests := []struct {
		config harmonyConfig
	}{
		{
			config: makeTestConfig("mainnet", nil),
		},
		{
			config: makeTestConfig("devnet", nil),
		},
		{
			config: makeTestConfig("mainnet", func(cfg *harmonyConfig) {
				consensus := getDefaultConsensusConfigCopy()
				cfg.Consensus = &consensus

				devnet := getDefaultDevnetConfigCopy()
				cfg.Devnet = &devnet

				revert := getDefaultRevertConfigCopy()
				cfg.Revert = &revert

				webHook := "web hook"
				cfg.Legacy = &legacyConfig{
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
		config, err := loadHarmonyConfig(file)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(config, test.config) {
			t.Errorf("Test %v: unexpected config \n\t%+v \n\t%+v", i, config, test.config)
		}
	}
}
