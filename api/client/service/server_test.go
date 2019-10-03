package client

import (
	"bytes"
	"math/big"
	"testing"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/internal/chain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	client "github.com/harmony-one/harmony/api/client/service/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
)

var (
	// Test accounts
	testBankKey, _  = crypto.GenerateKey()
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(8000000000000000000)

	chainConfig  = params.TestChainConfig
	blockFactory = blockfactory.NewFactory(chainConfig)
)

func TestGetFreeToken(test *testing.T) {
	hash := common.Hash{}
	hash.SetBytes([]byte("hello"))
	server := NewServer(func() (*state.DB, error) {
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
		database = ethdb.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Factory: blockFactory,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 10,
		}
	)

	genesis := gspec.MustCommit(database)
	_ = genesis
	chain, _ := core.NewBlockChain(database, nil, gspec.Config, chain.Engine, vm.Config{}, nil)

	hash := common.Hash{}
	hash.SetBytes([]byte("hello"))
	server := NewServer(func() (*state.DB, error) {
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
