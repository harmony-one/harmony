package client

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/api/client/service/proto"
	"github.com/harmony-one/harmony/core/state"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/db"
)

var (
	// Test accounts
	testBankKey, _  = crypto.GenerateKey()
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(8000000000000000000)

	chainConfig = params.TestChainConfig
)

func TestGetFreeToken(test *testing.T) {
	hash := common.Hash{}
	hash.SetBytes([]byte("hello"))
	server := NewServer(func() (*state.StateDB, error) {
		return nil, nil
	}, func(common.Address) common.Hash {
		return hash
	})

	testBankKey, _ := crypto.GenerateKey()
	testBankAddress := crypto.PubkeyToAddress(testBankKey.PublicKey)
	response, err := server.GetFreeToken(nil, &client.GetFreeTokenRequest{Address: testBankAddress.Bytes()})

	if err != nil {
		test.Errorf("Failed to get free token")
	}
	if bytes.Compare(response.TxId, hash.Bytes()) != 0 {
		test.Errorf("Wrong transaction id is returned")
	}
}

func TestFetchAccountState(test *testing.T) {
	var (
		database = db.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 10,
		}
	)

	genesis := gspec.MustCommit(database)
	_ = genesis
	chain, _ := core.NewBlockChain(database, nil, gspec.Config, consensus.NewFaker(), vm.Config{}, nil)

	hash := common.Hash{}
	hash.SetBytes([]byte("hello"))
	server := NewServer(func() (*state.StateDB, error) {
		return chain.State()
	}, func(common.Address) common.Hash {
		return hash
	})

	response, err := server.FetchAccountState(nil, &client.FetchAccountStateRequest{Address: testBankAddress.Bytes()})

	if err != nil {
		test.Errorf("Failed to get free token")
	}

	if bytes.Compare(response.Balance, testBankFunds.Bytes()) != 0 {
		test.Errorf("Wrong balance is returned")
	}

	if response.Nonce != 0 {
		test.Errorf("Wrong nonce is returned")
	}
}
