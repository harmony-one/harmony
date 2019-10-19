package utils

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/p2p"
)

func TestReadWalletProfile(t *testing.T) {
	config := []*WalletProfile{
		{
			Profile:   "default",
			ChainID:   params.MainnetChainID.String(),
			Bootnodes: []string{"127.0.0.1:9000/abcd", "127.0.0.1:9999/daeg"},
			Shards:    4,
			Network:   "mainnet",
			RPCServer: [][]p2p.Peer{
				{
					{
						IP:   "127.0.0.4",
						Port: "8888",
					},
					{
						IP:   "192.168.0.4",
						Port: "9876",
					},
				},
				{
					{
						IP:   "127.0.0.1",
						Port: "8888",
					},
					{
						IP:   "192.168.0.1",
						Port: "9876",
					},
				},
				{
					{
						IP:   "127.0.0.2",
						Port: "8888",
					},
					{
						IP:   "192.168.0.2",
						Port: "9876",
					},
				},
				{
					{
						IP:   "127.0.0.3",
						Port: "8888",
					},
					{
						IP:   "192.168.0.3",
						Port: "9876",
					},
				},
			},
		},
		{
			Profile:   "testnet",
			ChainID:   params.TestnetChainID.String(),
			Bootnodes: []string{"192.168.0.1:9990/abcd", "127.0.0.1:8888/daeg"},
			Shards:    3,
			Network:   "testnet",
			RPCServer: [][]p2p.Peer{
				{
					{
						IP:   "192.168.2.3",
						Port: "8888",
					},
					{
						IP:   "192.168.192.3",
						Port: "9877",
					},
				},
				{
					{
						IP:   "192.168.2.1",
						Port: "8888",
					},
					{
						IP:   "192.168.192.1",
						Port: "9877",
					},
				},
				{
					{
						IP:   "192.168.2.2",
						Port: "8888",
					},
					{
						IP:   "192.168.192.2",
						Port: "9877",
					},
				},
			},
		},
	}

	testIniBytes, err := ioutil.ReadFile("test.ini")
	if err != nil {
		t.Fatalf("Failed to read test.ini: %v", err)
	}

	config1, err := ReadWalletProfile(testIniBytes, "default")
	if err != nil {
		t.Fatalf("ReadWalletProfile Error: %v", err)
	}
	if !reflect.DeepEqual(config[0], config1) {
		t.Errorf("Got: %v\nExpect: %v\n", config1, config[0])
	}
	config2, err := ReadWalletProfile(testIniBytes, "testnet")
	if err != nil {
		t.Fatalf("ReadWalletProfile Error: %v", err)
	}
	if !reflect.DeepEqual(config[1], config2) {
		t.Errorf("Got: %v\nExpect: %v\n", config2, config[1])
	}
}
