//package main
//
//import (
//	"fmt"
//	"os"
//	"path/filepath"
//	"reflect"
//	"testing"
//	"time"
//)
//
//var testHmyConfig = hmyConfig{
//	Run: runConfig{
//		NodeType:  "validator",
//		IsStaking: true,
//		ShardID:   0,
//	},
//	NetworkType: networkConfig{
//		NetworkType:    "mainnet",
//		IP:         "127.0.0.1",
//		Port:       9000,
//		MinPeers:   32,
//		P2PKeyFile: "./.hmykey",
//		PublicRPC:  false,
//		BootNodes: []string{
//			"/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
//			"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9",
//			"/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX",
//			"/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj",
//		},
//		DNSZone: "t.hmny.io",
//		DNSPort: 9000,
//	},
//	Consensus: consensusConfig{
//		DelayCommit: 0,
//		BlockTime:   8 * time.Second,
//	},
//	BLSKey: blsConfig{
//		KeyDir:           "./.hmy/blskeys",
//		KeyFiles:         []string{"./xxxx.key"},
//		PassSrcType:      "auto",
//		PassFile:         "pass.file",
//		SavePassphrase:   true,
//		KmsConfigSrcType: "shared",
//		KmsConfigFile:    "config.json",
//	},
//	TxPool: txPoolConfig{
//		BlacklistFile:      ".hmy/blacklist.txt",
//		BroadcastInvalidTx: false,
//	},
//	Storage: storageConfig{
//		IsArchival:  false,
//		DatabaseDir: "./",
//	},
//	Pprof: pprofConfig{
//		Enabled:    true,
//		ListenAddr: "localhost:6060",
//	},
//	Log: logConfig{
//		LogFolder:  "latest",
//		LogMaxSize: 100,
//	},
//	Devnet: nil,
//}
//
//var devnetHmyConfig = hmyConfig{
//	NetworkType: networkConfig{
//		NetworkType:   "devnet",
//		BootNodes: []string{},
//	},
//	Devnet: &devnetConfig{
//		NumShards: 2,
//	},
//	BLSKey: blsConfig{
//		KeyFiles: []string{},
//	},
//}
//
//var testBaseDir = filepath.Join(os.TempDir(), "harmony", "cmd", "harmony")
//
//func init() {
//	if _, err := os.Stat(testBaseDir); os.IsNotExist(err) {
//		os.MkdirAll(testBaseDir, 0777)
//	}
//}
//
//func TestPersistConfig(t *testing.T) {
//	testDir := filepath.Join(testBaseDir, t.Name())
//	os.RemoveAll(testDir)
//	os.MkdirAll(testDir, 0777)
//
//	tests := []struct {
//		config hmyConfig
//	}{
//		{
//			config: testHmyConfig,
//		},
//		{
//			config: devnetHmyConfig,
//		},
//	}
//	for i, test := range tests {
//		file := filepath.Join(testDir, fmt.Sprintf("%d.conf", i))
//
//		if err := writeConfigToFile(test.config, file); err != nil {
//			t.Fatal(err)
//		}
//		config, err := loadConfig(file)
//		if err != nil {
//			t.Fatal(err)
//		}
//		if !reflect.DeepEqual(config, test.config) {
//			t.Errorf("Test %v: unexpected config \n\t%+v \n\t%+v", i, config, test.config)
//		}
//	}
//}
