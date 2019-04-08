package utils

import (
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/p2p"
)

func TestReadWalletProfile(t *testing.T) {
	config := []*WalletProfile{
		&WalletProfile{
			Profile:   "default",
			Bootnodes: []string{"127.0.0.1:9000/abcd", "127.0.0.1:9999/daeg"},
			Shards:    4,
			RPCServer: [][]p2p.Peer{
				[]p2p.Peer{
					p2p.Peer{
						IP:   "127.0.0.4",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.0.4",
						Port: "9876",
					},
				},
				[]p2p.Peer{
					p2p.Peer{
						IP:   "127.0.0.1",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.0.1",
						Port: "9876",
					},
				},
				[]p2p.Peer{
					p2p.Peer{
						IP:   "127.0.0.2",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.0.2",
						Port: "9876",
					},
				},
				[]p2p.Peer{
					p2p.Peer{
						IP:   "127.0.0.3",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.0.3",
						Port: "9876",
					},
				},
			},
		},
		&WalletProfile{
			Profile:   "testnet",
			Bootnodes: []string{"192.168.0.1:9990/abcd", "127.0.0.1:8888/daeg"},
			Shards:    3,
			RPCServer: [][]p2p.Peer{
				[]p2p.Peer{
					p2p.Peer{
						IP:   "192.168.2.3",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.192.3",
						Port: "9877",
					},
				},
				[]p2p.Peer{
					p2p.Peer{
						IP:   "192.168.2.1",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.192.1",
						Port: "9877",
					},
				},
				[]p2p.Peer{
					p2p.Peer{
						IP:   "192.168.2.2",
						Port: "8888",
					},
					p2p.Peer{
						IP:   "192.168.192.2",
						Port: "9877",
					},
				},
			},
		},
	}

	config1, err := ReadWalletProfile("test.ini", "default")
	if err != nil {
		t.Fatalf("ReadWalletProfile Error: %v", err)
	}
	if !reflect.DeepEqual(config[0], config1) {
		t.Errorf("Got: %v\nExpect: %v\n", config1, config[0])
	}
	config2, err := ReadWalletProfile("test.ini", "testnet")
	if err != nil {
		t.Fatalf("ReadWalletProfile Error: %v", err)
	}
	if !reflect.DeepEqual(config[1], config2) {
		t.Errorf("Got: %v\nExpect: %v\n", config2, config[1])
	}
}
