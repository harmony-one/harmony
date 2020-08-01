package main

import (
	"fmt"
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
