package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/stretchr/testify/assert"
)

var (
	trueBool = true
)

type testCfgOpt func(config *HarmonyConfig)

func makeTestConfig(nt nodeconfig.NetworkType, opt testCfgOpt) HarmonyConfig {
	cfg := GetDefaultHmyConfigCopy(nt)
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
	testConfigFileName, err := filepath.Abs(filepath.Join("testdata", "config_v1.toml"))
	assert.Nil(t, err)
	testConfig, err := ioutil.ReadFile(testConfigFileName)
	assert.Nil(t, err)
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)
	file := filepath.Join(testDir, "test.config")
	err = ioutil.WriteFile(file, []byte(testConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}
	config, err := LoadHarmonyConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, config.IsLatestVersion())
	defConf := GetDefaultHmyConfigCopy(nodeconfig.Mainnet)
	if config.HTTP.RosettaEnabled {
		t.Errorf("Expected rosetta http server to be disabled when loading old config")
	}
	if config.General.IsOffline {
		t.Errorf("Expect node to de online when loading old config")
	}
	if config.P2P.IP != defConf.P2P.IP {
		t.Errorf("Expect default p2p IP if old config is provided")
	}
	if config.Version != "1.0.4" {
		t.Errorf("Expected config version: 1.0.4, not %v", config.Version)
	}
	config.Version = defConf.Version // Shortcut for testing, value checked above
	assert.Equal(t, defConf, config)
}

func TestV2_0_0Config(t *testing.T) {
	testConfigFileName, err := filepath.Abs(filepath.Join("testdata", "config_v2.toml"))
	assert.Nil(t, err)
	testConfig, err := ioutil.ReadFile(testConfigFileName)
	assert.Nil(t, err)
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)
	file := filepath.Join(testDir, "test.config")
	err = ioutil.WriteFile(file, []byte(testConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}
	config, err := LoadHarmonyConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, config.IsLatestVersion())
	defConf := GetDefaultHmyConfigCopy(nodeconfig.Mainnet)
	defConf.DNSSync.DNSPort = 9002
	defConf.DNSSync.DNSZone = "8.8.8.8"
	assert.Equal(t, defConf, config)
}

func TestPersistConfig(t *testing.T) {
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)

	tests := []struct {
		config HarmonyConfig
	}{
		{
			config: makeTestConfig("mainnet", nil),
		},
		{
			config: makeTestConfig("devnet", nil),
		},
		{
			config: makeTestConfig("mainnet", func(cfg *HarmonyConfig) {
				consensus := GetDefaultConsensusConfigCopy()
				cfg.Consensus = &consensus

				devnet := GetDefaultDevnetConfigCopy()
				cfg.Devnet = &devnet

				revert := GetDefaultRevertConfigCopy()
				cfg.Revert = &revert

				webHook := "web hook"
				cfg.Legacy = &LegacyConfig{
					WebHookConfig:         &webHook,
					TPBroadcastInvalidTxn: &trueBool,
				}

				logCtx := GetDefaultLogContextCopy()
				cfg.Log.Context = &logCtx
			}),
		},
	}
	for i, test := range tests {
		file := filepath.Join(testDir, fmt.Sprintf("%d.conf", i))

		if err := WriteHarmonyConfigToFile(test.config, file); err != nil {
			t.Fatal(err)
		}
		config, err := LoadHarmonyConfig(file)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, test.config, config, "test %d: configs should match", i)
	}
}

func TestBackupAndUpgradeConfigToTheLatestVersion(t *testing.T) {
	testDir := filepath.Join(testBaseDir, t.Name())
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0777)

	fileName := filepath.Join(testDir, "config.toml")
	testConfig := `Version = "1.0.4"`
	err := ioutil.WriteFile(fileName, []byte(testConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}
	config, err := LoadHarmonyConfig(fileName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "1.0.4", config.Version)
	// when there is no fileName.old
	oldFileName, err := BackupAndUpgradeConfigToTheLatestVersion(config, fileName)
	assert.Nil(t, err)
	assert.Equal(t, fileName+".old", oldFileName)
	assert.FileExists(t, oldFileName)
	oldTestConfig, err := ioutil.ReadFile(oldFileName)
	assert.Nil(t, err)
	assert.Equal(t, []byte(testConfig), oldTestConfig)
	newConfig, err := LoadHarmonyConfig(fileName)
	assert.Nil(t, err)
	assert.Equal(t, TOMLConfigVersion, newConfig.Version)

	// when there exists fileName.old
	oldFileName, err = BackupAndUpgradeConfigToTheLatestVersion(config, fileName)
	assert.Nil(t, err)
	assert.Equal(t, fileName+".old.old", oldFileName)
	assert.FileExists(t, oldFileName)
}
