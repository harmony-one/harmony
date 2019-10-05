package validation

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
)

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		str string
		exp bool
	}{
		{"one1ay37rp2pc3kjarg7a322vu3sa8j9puahg679z3", true},
		{"0x7c41E0668B551f4f902cFaec05B5Bdca68b124CE", true},
		{"onefoofoo", false},
		{"0xbarbar", false},
		{"dsasdadsasaadsas", false},
		{"32312123213213212321", false},
	}

	for _, test := range tests {
		valid, _ := ValidateAddress("sender", test.str, common.ParseAddr(test.str))

		if valid != test.exp {
			t.Errorf("validation.validateAddress(\"%s\") returned %v, expected %v", test.str, valid, test.exp)
		}
	}
}

func TestIsValidShard(t *testing.T) {
	readProfile("local")

	tests := []struct {
		shardID int
		exp     bool
	}{
		{0, true},
		{1, true},
		{-1, false},
		{99, false},
	}

	for _, test := range tests {
		valid := ValidShard(test.shardID, walletProfile.Shards)

		if valid != test.exp {
			t.Errorf("validation.ValidShard(%d) returned %v, expected %v", test.shardID, valid, test.exp)
		}
	}
}

// Helper functions for the testing of valid shards - just copied from github.com/harmony-one/harmony/cmd/client/wallet/main.go
const (
	defaultConfigFile = ".hmy/wallet.ini"
	defaultProfile    = "local"
)

var (
	walletProfile *utils.WalletProfile
)

func readProfile(profile string) {
	fmt.Printf("Using %s profile for wallet\n", profile)

	// try to load .hmy/wallet.ini from filesystem
	// use default_wallet_ini if .hmy/wallet.ini doesn't exist
	var err error
	var iniBytes []byte

	iniBytes, err = ioutil.ReadFile(defaultConfigFile)
	if err != nil {
		log.Debug(fmt.Sprintf("%s doesn't exist, using default ini\n", defaultConfigFile))
		iniBytes = []byte(defaultWalletIni)
	}

	walletProfile, err = utils.ReadWalletProfile(iniBytes, profile)
	if err != nil {
		fmt.Printf("Read wallet profile error: %v\nExiting ...\n", err)
		os.Exit(2)
	}
}

// Tried embedding defaultWalletIni using utils.EmbedFile, but without any luck. So this will do for now
const (
	defaultWalletIni = `[main]
chain_id = 1
bootnode = /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
bootnode = /ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
bootnode = /ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj
shards = 4

[main.shard0.rpc]
rpc = l0.t.hmny.io:14555
rpc = s0.t.hmny.io:14555

[main.shard1.rpc]
rpc = l1.t.hmny.io:14555
rpc = s1.t.hmny.io:14555

[main.shard2.rpc]
rpc = l2.t.hmny.io:14555
rpc = s2.t.hmny.io:14555

[main.shard3.rpc]
rpc = l3.t.hmny.io:14555
rpc = s3.t.hmny.io:14555

[local]
chain_id = 2
bootnode = /ip4/127.0.0.1/tcp/19876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv
shards = 2

[local.shard0.rpc]
rpc = 127.0.0.1:14555
rpc = 127.0.0.1:14557
rpc = 127.0.0.1:14559

[local.shard1.rpc]
rpc = 127.0.0.1:14556
rpc = 127.0.0.1:14558
rpc = 127.0.0.1:14560

[beta]
chain_id = 2
bootnode = /ip4/54.213.43.194/tcp/9868/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
bootnode = /ip4/100.26.90.187/tcp/9868/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/13.113.101.219/tcp/12018/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
shards = 2

[beta.shard0.rpc]
rpc = l0.b.hmny.io:14555
rpc = s0.b.hmny.io:14555

[beta.shard1.rpc]
rpc = l1.b.hmny.io:14555
rpc = s1.b.hmny.io:14555

[pangaea]
chain_id = 3
bootnode = /ip4/54.86.126.90/tcp/9889/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/52.40.84.2/tcp/9889/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
shards = 4

[pangaea.shard0.rpc]
rpc = l0.p.hmny.io:14555
rpc = s0.p.hmny.io:14555

[pangaea.shard1.rpc]
rpc = l1.p.hmny.io:14555
rpc = s1.p.hmny.io:14555

[pangaea.shard2.rpc]
rpc = l2.p.hmny.io:14555
rpc = s2.p.hmny.io:14555

[pangaea.shard3.rpc]
rpc = l3.p.hmny.io:14555
rpc = s3.p.hmny.io:14555
`
)
